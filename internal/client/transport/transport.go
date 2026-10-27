// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package transport is the HTTP layer of the Open WebUI client: authentication, timeouts,
// retries, TLS, the request counter, and the mapping of error responses to sentinel errors.
// It holds no resource logic.
package transport

import (
	"bytes"
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/AyRickk/provider-openwebui/internal/version"
)

const (
	defaultAuthHeader     = "Authorization"
	defaultConnectTimeout = 10 * time.Second
	defaultRequestTimeout = 30 * time.Second

	// maxAttempts bounds the attempts of a request, the first one included.
	maxAttempts = 3
	// baseBackoff is the wait before the first retry, before jitter.
	baseBackoff = 500 * time.Millisecond
	// maxRetryAfter is the longest Retry-After waited for; beyond it the error returns at
	// once and the next reconcile retries.
	maxRetryAfter = 30 * time.Second

	// noRetry is the wait that means the failure is not retried.
	noRetry time.Duration = -1
)

// Config configures a Transport.
type Config struct {
	// BaseURL is the root URL of the Open WebUI instance, e.g. https://owui.example.com.
	BaseURL string

	// Token is the API key. It is sent only in the auth header, and redacted from errors.
	Token string

	// AuthHeader is the header that carries the token. The default, Authorization, sends
	// "Bearer <token>"; any other header (e.g. Open WebUI's x-api-key) sends the bare token.
	AuthHeader string

	// ConnectTimeout bounds establishing a connection, TLS handshake included. It
	// defaults to 10 seconds.
	ConnectTimeout time.Duration

	// RequestTimeout bounds each attempt, from sending the request to reading the whole
	// response. It defaults to 30 seconds.
	RequestTimeout time.Duration

	// CACertPEM holds PEM-encoded CA certificates trusted in addition to the system's.
	CACertPEM []byte

	// InsecureSkipVerify disables the verification of the server certificate. New logs a
	// warning when it is set.
	InsecureSkipVerify bool

	// Metrics counts every attempt. It may be nil.
	Metrics *Metrics

	// Logger receives the warnings, and the debug logs of requests and retries. It
	// defaults to a no-op logger.
	Logger logging.Logger
}

// Request is one call to the Open WebUI API.
type Request struct {
	// Method is the HTTP method.
	Method string

	// Endpoint is the path template, e.g. /api/v1/groups/id/{id}. Each {name} is replaced
	// by the path-escaped value of Params[name]. The template, not the expanded path,
	// labels the request counter and errors.
	Endpoint string

	// Params fills the placeholders of Endpoint.
	Params map[string]string

	// Query is the query string.
	Query url.Values

	// Body is sent as JSON when not nil.
	Body any

	// StatusErrors overrides, for this endpoint, the sentinel an error response maps to, for
	// Open WebUI's per-endpoint quirks (e.g. a missing group answering 401 on 0.11). The
	// first matching entry wins; without one, the status keeps its default sentinel.
	StatusErrors []StatusError
}

// A StatusError maps the error responses with Status, and a detail containing Detail
// unless it is empty, to Err. A nil Err maps them to no sentinel.
type StatusError struct {
	Status int
	Detail string
	Err    error
}

// String returns the method and endpoint template, e.g. GET /api/v1/groups/id/{id}.
func (r Request) String() string {
	return r.Method + " " + r.Endpoint
}

// idempotent reports whether resending the request cannot create a second object. A
// write (POST, PATCH) that may have reached the server is not resent: it could have
// created the object already.
func (r Request) idempotent() bool {
	switch r.Method {
	case http.MethodPost, http.MethodPatch:
		return false
	}
	return true
}

// A Transport sends requests to one Open WebUI instance. It is safe for concurrent use.
type Transport struct {
	baseURL    string
	token      string
	authHeader string
	authValue  string
	timeout    time.Duration
	http       *http.Client
	metrics    *Metrics
	log        logging.Logger

	// sleep waits between attempts; tests replace it.
	sleep func(context.Context, time.Duration) error
}

// New returns a Transport for the supplied configuration.
func New(cfg Config) (*Transport, error) {
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.Errorf("invalid Open WebUI URL %q: want http(s)://host[/path]", cfg.BaseURL)
	}
	log := cfg.Logger
	if log == nil {
		log = logging.NewNopLogger()
	}
	tlsCfg, err := tlsConfig(cfg, log)
	if err != nil {
		return nil, err
	}
	connect := cmp.Or(cfg.ConnectTimeout, defaultConnectTimeout)
	ht := http.DefaultTransport.(*http.Transport).Clone()
	ht.TLSClientConfig = tlsCfg
	ht.DialContext = (&net.Dialer{Timeout: connect, KeepAlive: 30 * time.Second}).DialContext
	ht.TLSHandshakeTimeout = connect
	t := &Transport{
		baseURL:    strings.TrimSuffix(cfg.BaseURL, "/"),
		token:      cfg.Token,
		authHeader: cmp.Or(cfg.AuthHeader, defaultAuthHeader),
		authValue:  cfg.Token,
		timeout:    cmp.Or(cfg.RequestTimeout, defaultRequestTimeout),
		http: &http.Client{
			Transport: ht,
			// A redirect could carry the token to another host: it is returned as an error.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		metrics: cfg.Metrics,
		log:     log,
		sleep:   sleep,
	}
	if strings.EqualFold(t.authHeader, defaultAuthHeader) {
		t.authValue = "Bearer " + cfg.Token
	}
	return t, nil
}

// tlsConfig returns the TLS configuration of cfg, and warns when verification is off.
func tlsConfig(cfg Config, log logging.Logger) (*tls.Config, error) {
	c := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.InsecureSkipVerify} //nolint:gosec // Opt-in, with a warning.
	if cfg.InsecureSkipVerify {
		log.Info("TLS certificate verification is disabled: the identity of the Open WebUI server is not checked", "url", cfg.BaseURL)
	}
	if len(cfg.CACertPEM) == 0 {
		return c, nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(cfg.CACertPEM) {
		return nil, errors.New("cannot parse the CA certificate: no PEM certificate found")
	}
	c.RootCAs = pool
	return c, nil
}

// Do sends req and decodes a successful JSON response into out, unless out is nil. It
// retries 429, 502, 503, 504 and network errors, at most 3 attempts in all; a write is
// retried only on 429, 503, or a network error before the request was sent.
func (t *Transport) Do(ctx context.Context, req Request, out any) error {
	u, err := t.url(req)
	if err != nil {
		return err
	}
	var body []byte
	if req.Body != nil {
		if body, err = json.Marshal(req.Body); err != nil {
			return errors.Wrapf(err, "cannot encode the body of %s", req)
		}
	}
	for attempt := 1; ; attempt++ {
		start := time.Now()
		code, wait, err := t.attempt(ctx, req, u, body, out)
		t.observe(req, attempt, code, time.Since(start), err)
		if err == nil || wait == noRetry || attempt == maxAttempts {
			return err
		}
		if wait == 0 {
			wait = backoff(attempt)
		}
		t.log.Debug("Retrying Open WebUI request", "method", req.Method, "endpoint", req.Endpoint, "wait", wait)
		if t.sleep(ctx, wait) != nil {
			return err
		}
	}
}

// observe counts and logs one attempt.
func (t *Transport) observe(req Request, attempt int, code string, d time.Duration, err error) {
	t.metrics.count(req.Method, req.Endpoint, code)
	kv := []any{"method", req.Method, "endpoint", req.Endpoint, "attempt", attempt, "code", code, "duration", d}
	if err != nil {
		kv = append(kv, "error", err)
	}
	t.log.Debug("Open WebUI request", kv...)
}

// attempt sends the request once, and returns the code label of its outcome. On a
// transient failure it returns the wait the server asked for (0 for none); otherwise it
// returns noRetry.
func (t *Transport) attempt(ctx context.Context, req Request, u string, body []byte, out any) (string, time.Duration, error) {
	actx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	var sent atomic.Bool
	actx = httptrace.WithClientTrace(actx, &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) { sent.Store(true) },
	})
	hreq, err := t.newRequest(actx, req, u, body)
	if err != nil {
		return codeError, noRetry, err
	}
	resp, err := t.http.Do(hreq)
	if err != nil {
		err = errors.Wrapf(err, "%s", req)
		var verr *tls.CertificateVerificationError
		if ctx.Err() != nil || errors.As(err, &verr) || sent.Load() && !req.idempotent() {
			return codeError, noRetry, err
		}
		return codeError, 0, err
	}
	defer resp.Body.Close() //nolint:errcheck // Nothing useful to do with a close error.
	wait, err := t.read(req, resp, out)
	return strconv.Itoa(resp.StatusCode), wait, err
}

// newRequest builds the HTTP request of one attempt.
func (t *Transport) newRequest(ctx context.Context, req Request, u string, body []byte) (*http.Request, error) {
	var rb io.Reader
	if body != nil {
		rb = bytes.NewReader(body)
	}
	hreq, err := http.NewRequestWithContext(ctx, req.Method, u, rb)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot build %s", req)
	}
	hreq.Header.Set("User-Agent", "provider-openwebui/"+version.Version)
	hreq.Header.Set("Accept", "application/json")
	if body != nil {
		hreq.Header.Set("Content-Type", "application/json")
	}
	hreq.Header.Set(t.authHeader, t.authValue)
	return hreq, nil
}

// read decodes a successful response into out, or returns the error of a failed one with
// the wait before a retry, as attempt does. 429 and 503 mean the server refused the
// request; after 502 and 504 a write may have been applied.
func (t *Transport) read(req Request, resp *http.Response, out any) (time.Duration, error) {
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		err := t.newError(req, resp.StatusCode, b)
		switch resp.StatusCode {
		case http.StatusBadGateway, http.StatusGatewayTimeout:
			if !req.idempotent() {
				return noRetry, err
			}
			fallthrough
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			return retryAfter(resp.Header.Get("Retry-After"), time.Now()), err
		}
		return noRetry, err
	}
	if out == nil {
		return noRetry, nil
	}
	return noRetry, errors.Wrapf(json.NewDecoder(resp.Body).Decode(out), "cannot decode the response of %s", req)
}

// backoff is the wait before the attempt after the supplied one, without a Retry-After:
// baseBackoff doubled per attempt, with jitter over its upper half.
func backoff(attempt int) time.Duration {
	d := baseBackoff << (attempt - 1)
	return d/2 + rand.N(d/2+1) //nolint:gosec // Jitter needs no cryptographic randomness.
}

// retryAfter returns the wait a Retry-After header value asks for at now: 0 for none or an
// invalid value, and noRetry beyond maxRetryAfter.
func retryAfter(v string, now time.Time) time.Duration {
	var d time.Duration
	if s, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		d = time.Duration(s) * time.Second
	} else if at, err := http.ParseTime(v); err == nil {
		d = at.Sub(now)
	}
	switch {
	case d > maxRetryAfter:
		return noRetry
	case d < 0:
		return 0
	}
	return d
}

// sleep waits for d, or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var placeholder = regexp.MustCompile(`\{([^}]*)\}`)

// url expands the endpoint template of req and appends its query.
func (t *Transport) url(req Request) (string, error) {
	var err error
	path := placeholder.ReplaceAllStringFunc(req.Endpoint, func(m string) string {
		name := m[1 : len(m)-1]
		v, ok := req.Params[name]
		switch {
		case err != nil:
		case !ok:
			err = errors.Errorf("no parameter for %s in endpoint %q", m, req.Endpoint)
		case v == "" || v == "." || v == "..":
			// Escaping keeps them as is, and a proxy may resolve them to another endpoint.
			err = errors.Errorf(`invalid parameter %s %q: empty, "." and ".." are not allowed`, name, v)
		}
		return url.PathEscape(v)
	})
	if err != nil {
		return "", err
	}
	u := t.baseURL + path
	if len(req.Query) > 0 {
		u += "?" + req.Query.Encode()
	}
	return u, nil
}
