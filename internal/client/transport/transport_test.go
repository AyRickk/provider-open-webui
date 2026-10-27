// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/AyRickk/provider-openwebui/internal/version"
)

const (
	token         = "sk-0123456789abcdef0123456789abcdef"
	groupEndpoint = "/api/v1/groups/id/{id}"
	apiKeyHeader  = "X-Api-Key"
)

// seen is what the test server observed of one request.
type seen struct {
	Method    string
	Path      string
	Query     string
	UserAgent string
	Auth      map[string]string
	Body      string
}

// recordingServer answers every request with status and body, and records what it saw.
func recordingServer(t *testing.T, status int, body string, got *[]seen) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		auth := map[string]string{}
		for _, h := range []string{"Authorization", apiKeyHeader} {
			if v := r.Header.Get(h); v != "" {
				auth[h] = v
			}
		}
		*got = append(*got, seen{
			Method:    r.Method,
			Path:      r.URL.EscapedPath(),
			Query:     r.URL.RawQuery,
			UserAgent: r.Header.Get("User-Agent"),
			Auth:      auth,
			Body:      string(b),
		})
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDoSuccess(t *testing.T) {
	type args struct {
		authHeader string
		req        Request
	}
	type want struct {
		seen []seen
		out  map[string]any
		err  error
	}
	ua := "provider-openwebui/" + version.Version
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BearerByDefault": {
			reason: "Without an auth header setting, the token goes in Authorization: Bearer, and path parameters are escaped.",
			args: args{req: Request{
				Method:   http.MethodGet,
				Endpoint: groupEndpoint,
				Params:   map[string]string{"id": "a b/c"},
			}},
			want: want{
				seen: []seen{{Method: http.MethodGet, Path: "/api/v1/groups/id/a%20b%2Fc", UserAgent: ua, Auth: map[string]string{"Authorization": "Bearer " + token}}},
				out:  map[string]any{"id": "g1"},
			},
		},
		"CustomHeaderSendsTheBareToken": {
			reason: "Another auth header, such as Open WebUI's x-api-key, carries the bare token.",
			args: args{authHeader: apiKeyHeader, req: Request{
				Method:   http.MethodPost,
				Endpoint: "/api/v1/groups/create",
				Query:    map[string][]string{"q": {"x"}},
				Body:     map[string]string{"name": "g"},
			}},
			want: want{
				seen: []seen{{Method: http.MethodPost, Path: "/api/v1/groups/create", Query: "q=x", UserAgent: ua, Auth: map[string]string{apiKeyHeader: token}, Body: `{"name":"g"}`}},
				out:  map[string]any{"id": "g1"},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []seen
			srv := recordingServer(t, http.StatusOK, `{"id":"g1"}`, &got)
			tr, err := New(Config{BaseURL: srv.URL + "/", Token: token, AuthHeader: tc.args.authHeader})
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			var out map[string]any
			err = tr.Do(context.Background(), tc.args.req, &out)
			if diff := cmp.Diff(tc.want.err, err); diff != "" {
				t.Errorf("\n%s\nDo(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.out, out); diff != "" {
				t.Errorf("\n%s\nDo(...): -want out, +got out:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.seen, got); diff != "" {
				t.Errorf("\n%s\nDo(...): -want requests, +got requests:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDoErrors(t *testing.T) {
	type args struct {
		status       int
		body         string
		statusErrors map[int]error
	}
	type want struct {
		sentinel error
		err      string
	}
	long := strings.Repeat("é", 600)
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotFound": {
			reason: "A 404 is ErrNotFound, and the FastAPI detail is the message.",
			args:   args{status: http.StatusNotFound, body: `{"detail":"We could not find what you are looking for :/"}`},
			want:   want{sentinel: ErrNotFound, err: `GET /api/v1/groups/id/{id}: 404 Not Found: "We could not find what you are looking for :/"`},
		},
		"Conflict": {
			reason: "A 409 is ErrConflict.",
			args:   args{status: http.StatusConflict, body: `{"detail":"exists"}`},
			want:   want{sentinel: ErrConflict, err: `GET /api/v1/groups/id/{id}: 409 Conflict: "exists"`},
		},
		"Unauthorized": {
			reason: "A 401 is ErrUnauthorized.",
			args:   args{status: http.StatusUnauthorized, body: `{"detail":"Not authenticated"}`},
			want:   want{sentinel: ErrUnauthorized, err: `GET /api/v1/groups/id/{id}: 401 Unauthorized: "Not authenticated"`},
		},
		"Forbidden": {
			reason: "A 403 is ErrForbidden.",
			args:   args{status: http.StatusForbidden, body: `{"detail":"A human session is required."}`},
			want:   want{sentinel: ErrForbidden, err: `GET /api/v1/groups/id/{id}: 403 Forbidden: "A human session is required."`},
		},
		"EndpointQuirk": {
			reason: "A per-endpoint mapping overrides the default: a missing group answers 401 on 0.11.4.",
			args:   args{status: http.StatusUnauthorized, body: `{"detail":"Not found"}`, statusErrors: map[int]error{http.StatusUnauthorized: ErrNotFound}},
			want:   want{sentinel: ErrNotFound, err: `GET /api/v1/groups/id/{id}: 401 Unauthorized: "Not found"`},
		},
		"OtherStatus": {
			reason: "Another status maps to no sentinel; a body without detail is kept as is.",
			args:   args{status: http.StatusBadRequest, body: "plain text\n"},
			want:   want{err: `GET /api/v1/groups/id/{id}: 400 Bad Request: "plain text"`},
		},
		"StructuredDetail": {
			reason: "A detail that is not a string, such as a validation error, is kept as JSON.",
			args:   args{status: http.StatusUnprocessableEntity, body: `{"detail":[{"loc":["body","name"],"msg":"Field required"}]}`},
			want:   want{err: `GET /api/v1/groups/id/{id}: 422 Unprocessable Entity: "[{\"loc\":[\"body\",\"name\"],\"msg\":\"Field required\"}]"`},
		},
		"Truncated": {
			reason: "A detail longer than 512 characters is cut to 512 characters, the last one an ellipsis.",
			args:   args{status: http.StatusBadRequest, body: `{"detail":"` + long + `"}`},
			want:   want{err: `GET /api/v1/groups/id/{id}: 400 Bad Request: "` + long[:511*len("é")] + `…"`},
		},
		"Redacted": {
			reason: "The token, other API keys, bearer credentials and JWTs are redacted before truncation.",
			args:   args{status: http.StatusBadRequest, body: `{"detail":"key ` + token + ` other sk-abcdefabcdef12 header Bearer xyz.123 jwt eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjEifQ.c2ln end"}`},
			want:   want{err: `GET /api/v1/groups/id/{id}: 400 Bad Request: "key [REDACTED] other [REDACTED] header [REDACTED] jwt [REDACTED] end"`},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got []seen
			srv := recordingServer(t, tc.args.status, tc.args.body, &got)
			tr, err := New(Config{BaseURL: srv.URL, Token: token})
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			err = tr.Do(context.Background(), Request{
				Method:       http.MethodGet,
				Endpoint:     groupEndpoint,
				Params:       map[string]string{"id": "g1"},
				StatusErrors: tc.args.statusErrors,
			}, nil)
			if diff := cmp.Diff(tc.want.sentinel, err, cmpopts.EquateErrors()); tc.want.sentinel != nil && diff != "" {
				t.Errorf("\n%s\nDo(...): -want sentinel, +got error:\n%s", tc.reason, diff)
			}
			for _, s := range []error{ErrNotFound, ErrConflict, ErrUnauthorized, ErrForbidden} {
				if !errors.Is(tc.want.sentinel, s) && errors.Is(err, s) {
					t.Errorf("\n%s\nDo(...): error %q also matches %q", tc.reason, err, s)
				}
			}
			if diff := cmp.Diff(tc.want.err, fmt.Sprint(err)); diff != "" {
				t.Errorf("\n%s\nDo(...): -want error string, +got error string:\n%s", tc.reason, diff)
			}
		})
	}
}

// reply is one scripted answer of a scriptedServer.
type reply struct {
	status     int
	retryAfter string
	// drop closes the connection without answering.
	drop bool
}

// scriptedServer answers the n-th request with replies[n], then 200 {} once exhausted, and
// counts the requests.
func scriptedServer(t *testing.T, replies []reply, count *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(count.Add(1) - 1)
		if n >= len(replies) {
			_, _ = io.WriteString(w, "{}")
			return
		}
		rp := replies[n]
		if rp.drop {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("Hijack(): %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		if rp.retryAfter != "" {
			w.Header().Set("Retry-After", rp.retryAfter)
		}
		w.WriteHeader(rp.status)
		_, _ = io.WriteString(w, `{"detail":"busy"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// closedURL returns the URL of a port nothing listens on.
func closedURL(t *testing.T) string {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(...): %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr
}

// window is an accepted range [min, max] for a wait.
type window struct{ min, max time.Duration }

func TestDoRetries(t *testing.T) {
	type args struct {
		replies []reply
		method  string
		// refused sends the request to a closed port instead of the server.
		refused bool
	}
	type want struct {
		attempts int
		waits    []window
		status   int
		// netErr expects an error without a response.
		netErr bool
	}
	inTen := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RetriedThenSucceeds": {
			reason: "A 503 is retried after an exponential backoff with jitter: the first wait is in [250ms, 500ms].",
			args:   args{replies: []reply{{status: http.StatusServiceUnavailable}}},
			want:   want{attempts: 2, waits: []window{{250 * time.Millisecond, 500 * time.Millisecond}}},
		},
		"AtMostThreeAttempts": {
			reason: "502 and 504 are retried, at most 3 attempts in all, with a doubling backoff; the last error is returned.",
			args:   args{replies: []reply{{status: http.StatusBadGateway}, {status: http.StatusBadGateway}, {status: http.StatusGatewayTimeout}}},
			want:   want{attempts: 3, waits: []window{{250 * time.Millisecond, 500 * time.Millisecond}, {500 * time.Millisecond, time.Second}}, status: http.StatusGatewayTimeout},
		},
		"RetryAfterSeconds": {
			reason: "A 429 waits as long as its Retry-After in seconds.",
			args:   args{replies: []reply{{status: http.StatusTooManyRequests, retryAfter: "2"}}},
			want:   want{attempts: 2, waits: []window{{2 * time.Second, 2 * time.Second}}},
		},
		"RetryAfterDate": {
			reason: "A Retry-After date waits until that date.",
			args:   args{replies: []reply{{status: http.StatusServiceUnavailable, retryAfter: inTen}}},
			want:   want{attempts: 2, waits: []window{{8 * time.Second, 10 * time.Second}}},
		},
		"RetryAfterTooLong": {
			reason: "A Retry-After beyond 30s is not waited for: the error returns at once, and the next reconcile retries.",
			args:   args{replies: []reply{{status: http.StatusTooManyRequests, retryAfter: "120"}}},
			want:   want{attempts: 1, status: http.StatusTooManyRequests},
		},
		"NetworkError": {
			reason: "A connection closed without an answer is a network error, and is retried.",
			args:   args{replies: []reply{{drop: true}}},
			want:   want{attempts: 2, waits: []window{{250 * time.Millisecond, 500 * time.Millisecond}}},
		},
		"WriteSentNotRetried": {
			reason: "A write whose connection closed after it was sent is not resent: it may have created the object.",
			args:   args{method: http.MethodPost, replies: []reply{{drop: true}}},
			want:   want{attempts: 1, netErr: true},
		},
		"WriteNotSentRetried": {
			reason: "A write that could not connect never reached the server, and is retried.",
			args:   args{method: http.MethodPost, refused: true},
			want:   want{waits: []window{{250 * time.Millisecond, 500 * time.Millisecond}, {500 * time.Millisecond, time.Second}}, netErr: true},
		},
		"WriteRefusedRetried": {
			reason: "A write the server refused with 503 or 429 was not applied, and is retried.",
			args:   args{method: http.MethodPost, replies: []reply{{status: http.StatusServiceUnavailable}, {status: http.StatusTooManyRequests, retryAfter: "1"}}},
			want:   want{attempts: 3, waits: []window{{250 * time.Millisecond, 500 * time.Millisecond}, {time.Second, time.Second}}},
		},
		"WriteBadGatewayNotRetried": {
			reason: "A write answered 502 or 504 may have been applied upstream, and is not resent.",
			args:   args{method: http.MethodPost, replies: []reply{{status: http.StatusBadGateway}}},
			want:   want{attempts: 1, status: http.StatusBadGateway},
		},
		"ServerErrorNotRetried": {
			reason: "A 500 is not transient and is not retried.",
			args:   args{replies: []reply{{status: http.StatusInternalServerError}}},
			want:   want{attempts: 1, status: http.StatusInternalServerError},
		},
		"ClientErrorNotRetried": {
			reason: "Other 4xx are not retried.",
			args:   args{replies: []reply{{status: http.StatusNotFound}}},
			want:   want{attempts: 1, status: http.StatusNotFound},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var attempts atomic.Int64
			srv := scriptedServer(t, tc.args.replies, &attempts)
			base := srv.URL
			if tc.args.refused {
				base = closedURL(t)
			}
			tr, err := New(Config{BaseURL: base, Token: token})
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			var waits []time.Duration
			tr.sleep = func(_ context.Context, d time.Duration) error {
				waits = append(waits, d)
				return nil
			}
			method := tc.args.method
			if method == "" {
				method = http.MethodGet
			}
			err = tr.Do(context.Background(), Request{Method: method, Endpoint: "/api/v1/groups/", Body: map[string]string{}}, nil)
			status := 0
			var herr *Error
			if errors.As(err, &herr) {
				status = herr.StatusCode
			} else if (err != nil) != tc.want.netErr {
				t.Errorf("\n%s\nDo(...): got error %v, want a network error: %t", tc.reason, err, tc.want.netErr)
			}
			if diff := cmp.Diff(tc.want.status, status); diff != "" {
				t.Errorf("\n%s\nDo(...): -want status, +got status:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.attempts, int(attempts.Load())); diff != "" {
				t.Errorf("\n%s\nDo(...): -want attempts, +got attempts:\n%s", tc.reason, diff)
			}
			if len(waits) != len(tc.want.waits) {
				t.Fatalf("\n%s\nDo(...): waited %v, want %d waits", tc.reason, waits, len(tc.want.waits))
			}
			for i, w := range tc.want.waits {
				if waits[i] < w.min || waits[i] > w.max {
					t.Errorf("\n%s\nDo(...): wait %d is %v, want within [%v, %v]", tc.reason, i, waits[i], w.min, w.max)
				}
			}
		})
	}
}

func TestDoCanceledContextIsNotRetried(t *testing.T) {
	var attempts atomic.Int64
	srv := scriptedServer(t, []reply{{status: http.StatusServiceUnavailable}}, &attempts)
	tr, err := New(Config{BaseURL: srv.URL, Token: token})
	if err != nil {
		t.Fatalf("New(...): %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	tr.sleep = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}
	err = tr.Do(ctx, Request{Method: http.MethodGet, Endpoint: "/api/v1/groups/"}, nil)
	if diff := cmp.Diff(int64(1), attempts.Load()); diff != "" {
		t.Errorf("Do(...): a wait interrupted by the context stops the retries: -want attempts, +got attempts:\n%s", diff)
	}
	var herr *Error
	if !errors.As(err, &herr) || herr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Do(...): want the last response's error, got %v", err)
	}
}

func TestDoTimeouts(t *testing.T) {
	// slow answers after a second, unless the client gives up first.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	t.Cleanup(slow.Close)
	// mute accepts connections and never answers, so a TLS handshake never completes.
	mute, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(...): %v", err)
	}
	t.Cleanup(func() { _ = mute.Close() })
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			c, err := mute.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})

	cases := map[string]struct {
		reason  string
		cfg     Config
		wantErr string
	}{
		"Request": {
			reason:  "An attempt that takes longer than the request timeout fails, and is retried as a network error.",
			cfg:     Config{BaseURL: slow.URL, RequestTimeout: 50 * time.Millisecond, ConnectTimeout: 5 * time.Second},
			wantErr: "context deadline exceeded",
		},
		"Connect": {
			reason:  "A connection that takes longer than the connect timeout to establish, TLS handshake included, fails.",
			cfg:     Config{BaseURL: "https://" + mute.Addr().String(), RequestTimeout: 5 * time.Second, ConnectTimeout: 50 * time.Millisecond},
			wantErr: "TLS handshake timeout",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr, err := New(tc.cfg)
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			waits := 0
			tr.sleep = func(context.Context, time.Duration) error {
				waits++
				return nil
			}
			start := time.Now()
			err = tr.Do(context.Background(), Request{Method: http.MethodGet, Endpoint: "/api/version"}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("\n%s\nDo(...): want an error containing %q, got %v", tc.reason, tc.wantErr, err)
			}
			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Errorf("\n%s\nDo(...): took %v, want the timeout to cut it short", tc.reason, elapsed)
			}
			if diff := cmp.Diff(maxAttempts-1, waits); diff != "" {
				t.Errorf("\n%s\nDo(...): -want retries, +got retries:\n%s", tc.reason, diff)
			}
		})
	}
}

// entry is one message a recordingLogger received.
type entry struct {
	Level string
	Msg   string
	KV    []any
}

// recordingLogger records every message, with the values of WithValues prepended.
type recordingLogger struct {
	mu      *sync.Mutex
	entries *[]entry
	kv      []any
}

func newRecordingLogger() recordingLogger {
	return recordingLogger{mu: &sync.Mutex{}, entries: &[]entry{}}
}

func (l recordingLogger) log(level, msg string, kv []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, entry{Level: level, Msg: msg, KV: append(append([]any{}, l.kv...), kv...)})
}

func (l recordingLogger) Info(msg string, kv ...any)  { l.log("info", msg, kv) }
func (l recordingLogger) Debug(msg string, kv ...any) { l.log("debug", msg, kv) }
func (l recordingLogger) WithValues(kv ...any) logging.Logger {
	return recordingLogger{mu: l.mu, entries: l.entries, kv: append(append([]any{}, l.kv...), kv...)}
}

func (l recordingLogger) Entries() []entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]entry{}, *l.entries...)
}

func TestTLS(t *testing.T) {
	attempts := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		_, _ = io.WriteString(w, "{}")
	}))
	t.Cleanup(srv.Close)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})

	type want struct {
		newErr   string
		doErr    string
		attempts int
		warnings []string
	}
	cases := map[string]struct {
		reason string
		cfg    Config
		want   want
	}{
		"UnknownAuthority": {
			reason: "A server certificate from an unknown authority fails, and is not retried.",
			cfg:    Config{},
			want:   want{doErr: "certificate signed by unknown authority", attempts: 0},
		},
		"CustomCA": {
			reason: "A custom CA is trusted.",
			cfg:    Config{CACertPEM: ca},
			want:   want{attempts: 1},
		},
		"InvalidCA": {
			reason: "A CA without a PEM certificate is refused.",
			cfg:    Config{CACertPEM: []byte("not a certificate")},
			want:   want{newErr: "cannot parse the CA certificate: no PEM certificate found"},
		},
		"InsecureSkipVerify": {
			reason: "insecureSkipVerify accepts any certificate, and logs a warning.",
			cfg:    Config{InsecureSkipVerify: true},
			want:   want{attempts: 1, warnings: []string{"TLS certificate verification is disabled: the identity of the Open WebUI server is not checked"}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			attempts = 0
			log := newRecordingLogger()
			cfg := tc.cfg
			cfg.BaseURL, cfg.Token, cfg.Logger = srv.URL, token, log
			tr, err := New(cfg)
			if diff := cmp.Diff(tc.want.newErr, errString(err)); diff != "" {
				t.Fatalf("\n%s\nNew(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}
			retries := 0
			tr.sleep = func(context.Context, time.Duration) error {
				retries++
				return nil
			}
			err = tr.Do(context.Background(), Request{Method: http.MethodGet, Endpoint: "/api/version"}, nil)
			if retries != 0 {
				t.Errorf("\n%s\nDo(...): retried %d times, want no retry", tc.reason, retries)
			}
			if tc.want.doErr == "" && err != nil || tc.want.doErr != "" && (err == nil || !strings.Contains(err.Error(), tc.want.doErr)) {
				t.Errorf("\n%s\nDo(...): want an error containing %q, got %v", tc.reason, tc.want.doErr, err)
			}
			if diff := cmp.Diff(tc.want.attempts, attempts); diff != "" {
				t.Errorf("\n%s\nDo(...): -want attempts, +got attempts:\n%s", tc.reason, diff)
			}
			var warnings []string
			for _, e := range log.Entries() {
				if e.Level == "info" {
					warnings = append(warnings, e.Msg)
				}
			}
			if diff := cmp.Diff(tc.want.warnings, warnings); diff != "" {
				t.Errorf("\n%s\nNew(...): -want warnings, +got warnings:\n%s", tc.reason, diff)
			}
		})
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestMetricsAndLogs(t *testing.T) {
	var attempts atomic.Int64
	srv := scriptedServer(t, []reply{{drop: true}, {status: http.StatusServiceUnavailable}}, &attempts)
	reg := prometheus.NewRegistry()
	m, err := NewMetrics(reg)
	if err != nil {
		t.Fatalf("NewMetrics(...): %v", err)
	}
	log := newRecordingLogger()
	tr, err := New(Config{BaseURL: srv.URL, Token: token, Metrics: m, Logger: log})
	if err != nil {
		t.Fatalf("New(...): %v", err)
	}
	tr.sleep = func(context.Context, time.Duration) error { return nil }
	req := Request{Method: http.MethodGet, Endpoint: groupEndpoint, Params: map[string]string{"id": "g1"}}
	if err := tr.Do(context.Background(), req, nil); err != nil {
		t.Fatalf("Do(...): %v", err)
	}
	if err := tr.Do(context.Background(), req, nil); err != nil {
		t.Fatalf("Do(...): %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather(): %v", err)
	}
	counts := map[string]float64{}
	for _, f := range families {
		for _, mt := range f.GetMetric() {
			key := f.GetName()
			for _, l := range mt.GetLabel() {
				key += " " + l.GetName() + "=" + l.GetValue()
			}
			counts[key] = mt.GetCounter().GetValue()
		}
	}
	wantCounts := map[string]float64{
		"openwebui_api_requests_total code=error endpoint=/api/v1/groups/id/{id} method=GET": 1,
		"openwebui_api_requests_total code=503 endpoint=/api/v1/groups/id/{id} method=GET":   1,
		"openwebui_api_requests_total code=200 endpoint=/api/v1/groups/id/{id} method=GET":   2,
	}
	if diff := cmp.Diff(wantCounts, counts); diff != "" {
		t.Errorf("Every attempt is counted by method, endpoint template and status code, \"error\" without a response: -want, +got:\n%s", diff)
	}

	var requests [][]any
	for _, e := range log.Entries() {
		if e.Msg != "Open WebUI request" {
			continue
		}
		kv := []any{}
		for i := 0; i+1 < len(e.KV); i += 2 {
			if e.KV[i] != "duration" && e.KV[i] != "error" {
				kv = append(kv, e.KV[i], e.KV[i+1])
			}
		}
		requests = append(requests, append([]any{e.Level}, kv...))
	}
	wantRequests := [][]any{
		{"debug", "method", http.MethodGet, "endpoint", groupEndpoint, "attempt", 1, "code", "error"},
		{"debug", "method", http.MethodGet, "endpoint", groupEndpoint, "attempt", 2, "code", "503"},
		{"debug", "method", http.MethodGet, "endpoint", groupEndpoint, "attempt", 3, "code", "200"},
		{"debug", "method", http.MethodGet, "endpoint", groupEndpoint, "attempt", 1, "code", "200"},
	}
	if diff := cmp.Diff(wantRequests, requests); diff != "" {
		t.Errorf("Every attempt is logged at debug level: -want, +got:\n%s", diff)
	}
}

func TestTokenNeverLeaks(t *testing.T) {
	// echo puts the credentials it received everywhere it can: the error detail, a raw
	// body, a header, and a validation error.
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cred := r.Header.Get("Authorization") + " " + r.Header.Get(apiKeyHeader)
		w.Header().Set("X-Echo", cred)
		switch r.URL.Path {
		case "/detail":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"detail":"invalid token `+cred+`"}`)
		case "/raw":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, "<html>upstream rejected "+cred+"</html>")
		case "/validation":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"detail":[{"msg":"bad","input":"`+cred+`"}]}`)
		case "/long":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"detail":"`+strings.Repeat("x", 500)+cred+`"}`)
		}
	}))
	t.Cleanup(echo.Close)

	for _, header := range []string{"", apiKeyHeader} {
		for _, path := range []string{"/detail", "/raw", "/validation", "/long"} {
			t.Run(header+path, func(t *testing.T) {
				log := newRecordingLogger()
				tr, err := New(Config{BaseURL: echo.URL, Token: token, AuthHeader: header, Logger: log})
				if err != nil {
					t.Fatalf("New(...): %v", err)
				}
				tr.sleep = func(context.Context, time.Duration) error { return nil }
				err = tr.Do(context.Background(), Request{Method: http.MethodPost, Endpoint: path, Body: map[string]string{"k": "v"}}, nil)
				if err == nil {
					t.Fatal("Do(...): want an error")
				}
				entries := log.Entries()
				texts := make([]string, 0, 3+len(entries))
				texts = append(texts, err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err))
				for _, e := range entries {
					texts = append(texts, fmt.Sprint(e.Msg, e.KV))
				}
				for _, text := range texts {
					// The first 12 characters are enough to tell a truncated token.
					if strings.Contains(text, token[:12]) {
						t.Errorf("Do(...): the token leaked into %q", text)
					}
				}
			})
		}
	}
}

func TestLocalErrors(t *testing.T) {
	var got []seen
	srv := recordingServer(t, http.StatusOK, "not json", &got)
	cases := map[string]struct {
		reason  string
		baseURL string
		req     Request
		wantErr string
	}{
		"InvalidURL": {
			reason:  "A base URL that is not http(s)://host is refused.",
			baseURL: "owui.example.com",
			wantErr: `invalid Open WebUI URL "owui.example.com": want http(s)://host[/path]`,
		},
		"MissingParameter": {
			reason:  "A placeholder without a parameter is a programming error, reported before any request.",
			baseURL: srv.URL,
			req:     Request{Method: http.MethodGet, Endpoint: groupEndpoint},
			wantErr: `no parameter for {id} in endpoint "/api/v1/groups/id/{id}"`,
		},
		"DotParameter": {
			reason:  "A parameter of . or .. could make a proxy resolve another endpoint, and is refused.",
			baseURL: srv.URL,
			req:     Request{Method: http.MethodDelete, Endpoint: "/api/v1/groups/id/{id}/delete", Params: map[string]string{"id": ".."}},
			wantErr: `invalid parameter id "..": empty, "." and ".." are not allowed`,
		},
		"EmptyParameter": {
			reason:  "An empty parameter would collapse the path, and is refused.",
			baseURL: srv.URL,
			req:     Request{Method: http.MethodGet, Endpoint: "/api/v1/groups/id/{id}", Params: map[string]string{"id": ""}},
			wantErr: `invalid parameter id "": empty, "." and ".." are not allowed`,
		},
		"UndecodableResponse": {
			reason:  "A successful response that is not JSON is an error, not retried.",
			baseURL: srv.URL,
			req:     Request{Method: http.MethodGet, Endpoint: "/api/version"},
			wantErr: "cannot decode the response of GET /api/version: invalid character 'o' in literal null (expecting 'u')",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got = nil
			tr, err := New(Config{BaseURL: tc.baseURL, Token: token})
			if err == nil {
				var out map[string]any
				err = tr.Do(context.Background(), tc.req, &out)
			}
			if diff := cmp.Diff(tc.wantErr, errString(err)); diff != "" {
				t.Errorf("\n%s\n-want error, +got error:\n%s", tc.reason, diff)
			}
			if strings.HasPrefix(name, "Dot") || strings.HasPrefix(name, "Empty") {
				if len(got) != 0 {
					t.Errorf("\n%s\nsent %d requests, want none", tc.reason, len(got))
				}
			}
			if len(got) > 1 {
				t.Errorf("\n%s\nsent %d requests, want at most 1", tc.reason, len(got))
			}
		})
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var leaked []string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = append(leaked, r.Header.Get(apiKeyHeader)+r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, "{}")
	}))
	t.Cleanup(other.Close)
	srv := httptest.NewServer(http.RedirectHandler(other.URL+"/steal", http.StatusTemporaryRedirect))
	t.Cleanup(srv.Close)

	for _, header := range []string{"", apiKeyHeader} {
		t.Run("Header"+header, func(t *testing.T) {
			leaked = nil
			tr, err := New(Config{BaseURL: srv.URL, Token: token, AuthHeader: header})
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			err = tr.Do(context.Background(), Request{Method: http.MethodGet, Endpoint: "/api/v1/groups/"}, nil)
			var herr *Error
			if !errors.As(err, &herr) || herr.StatusCode != http.StatusTemporaryRedirect {
				t.Errorf("Do(...): a redirect is returned as an error, not followed: got %v", err)
			}
			if len(leaked) != 0 {
				t.Errorf("Do(...): the redirect was followed, sending %q", leaked)
			}
		})
	}
}
