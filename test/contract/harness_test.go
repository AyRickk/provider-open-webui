//go:build contract

// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package contract holds the contract tests of the client layer (spec #3, seam S1): Go tests
// of internal/client against a real Open WebUI instance, run by hack/contract/run.sh for each
// version of the supported matrix. They build only with the contract tag.
//
// The instance comes from the environment:
//
//	OWUI_URL, OWUI_API_KEY  the base URL and an admin API key, as hack/openwebui/up.sh prints them;
//	OWUI_VERSION            the version the instance runs, e.g. 0.12.4;
//	CONTRACT_INSTANCE       "default", or "restricted" when the API keys are restricted to
//	                        docs/api-notes/api-key-allowed-endpoints.txt;
//	CONTRACT_RECORD_DIR     when set, the recording mode: each recorder writes its exchanges
//	                        there, for hack/capture/write.sh to turn into fixtures.
package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/AyRickk/provider-openwebui/internal/client"
	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// An instanceKind is a kind of instance hack/contract/run.sh starts.
type instanceKind string

// The kinds of instance.
const (
	instanceDefault    instanceKind = "default"
	instanceRestricted instanceKind = "restricted"
)

// An instance is the Open WebUI instance under test.
type instance struct {
	URL     string
	APIKey  string
	Version string
	Kind    instanceKind
}

// instanceFromEnv returns the instance under test. It fails the test when the environment
// does not describe one, and skips it on an instance of another kind than kinds, when given.
func instanceFromEnv(t *testing.T, kinds ...instanceKind) instance {
	t.Helper()
	inst := instance{
		URL:     strings.TrimSuffix(os.Getenv("OWUI_URL"), "/"),
		APIKey:  os.Getenv("OWUI_API_KEY"),
		Version: os.Getenv("OWUI_VERSION"),
		Kind:    instanceKind(os.Getenv("CONTRACT_INSTANCE")),
	}
	if inst.URL == "" || inst.APIKey == "" || inst.Version == "" {
		t.Fatal("OWUI_URL, OWUI_API_KEY and OWUI_VERSION must describe the instance under test: run mise run contract")
	}
	if inst.Kind != instanceDefault && inst.Kind != instanceRestricted {
		t.Fatalf("CONTRACT_INSTANCE is %q: want %q or %q", inst.Kind, instanceDefault, instanceRestricted)
	}
	if len(kinds) > 0 && !slices.Contains(kinds, inst.Kind) {
		t.Skipf("needs a %v instance, this one is %s", kinds, inst.Kind)
	}
	return inst
}

// newTransport returns a transport that sends to baseURL with key.
func newTransport(t *testing.T, baseURL, key string) *transport.Transport {
	t.Helper()
	tr, err := transport.New(transport.Config{BaseURL: baseURL, Token: key})
	if err != nil {
		t.Fatalf("transport.New(%s): %v", baseURL, err)
	}
	return tr
}

// profileOf detects the version of the instance d sends to, and returns its profile.
func profileOf(t *testing.T, d client.Doer) (client.Version, *client.Profile) {
	t.Helper()
	v, err := client.DetectVersion(context.Background(), d)
	if err != nil {
		t.Fatalf("DetectVersion(...): %v", err)
	}
	p, unsupported, err := client.SelectProfile(v, false)
	if err != nil || unsupported {
		t.Fatalf("SelectProfile(%s, false): unsupported %t, error %v: want a supported profile", v, unsupported, err)
	}
	return v, p
}

// A recorder sits between the client and the instance, and records each exchange under the
// scenario and step the test names, in the format of hack/capture/lib.sh's capture_call. In
// the recording mode, it writes them to $CONTRACT_RECORD_DIR/<resource>.jsonl when the test
// passes, with the secrets the test used in <resource>.secrets, so that hack/capture/write.sh
// turns them into the fixtures of test/fixtures/openwebui/v<version>/<resource>/.
type recorder struct {
	t      *testing.T
	target string
	srv    *httptest.Server

	mu        sync.Mutex
	scenario  string
	step      string
	exchanges []exchange
	secrets   []string
}

// exchange is one line of the raw recording. Its fields are in the order of capture_call's,
// which normalize.jq keeps.
type exchange struct {
	Scenario string           `json:"scenario"`
	Step     string           `json:"step"`
	Request  recordedRequest  `json:"request"`
	Response recordedResponse `json:"response"`
}

type recordedRequest struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body"`
}

type recordedResponse struct {
	Status      int             `json:"status"`
	ContentType string          `json:"content_type"`
	Body        json.RawMessage `json:"body"`
}

// nonJSON stands for a body that is not JSON, as in capture_call.
var nonJSON = json.RawMessage(`"<non-JSON body>"`)

// record returns a recorder of the fixtures of resource, in front of inst.
func record(t *testing.T, inst instance, resource string) *recorder {
	t.Helper()
	r := &recorder{t: t, target: inst.URL}
	r.srv = httptest.NewServer(http.HandlerFunc(r.forward))
	t.Cleanup(func() {
		r.srv.Close()
		r.write(resource)
	})
	return r
}

// transport returns a transport that sends to the instance through the recorder, with key.
func (r *recorder) transport(key string) *transport.Transport {
	r.t.Helper()
	return newTransport(r.t, r.srv.URL, key)
}

// begin starts a new scenario; the following exchanges belong to it.
func (r *recorder) begin(scenario string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scenario = scenario
}

// next names the next exchange. Every exchange must be named: an unnamed one fails the test.
func (r *recorder) next(step string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.step = step
}

// secret adds a value that must not reach a fixture, checked by hack/capture/write.sh.
func (r *recorder) secret(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets = append(r.secrets, s)
}

// forward sends the request to the instance, records the exchange and returns the answer.
func (r *recorder) forward(w http.ResponseWriter, req *http.Request) {
	fail := func(format string, args ...any) {
		r.t.Errorf("recorder: %s %s: "+format, append([]any{req.Method, req.URL.RequestURI()}, args...)...)
		w.WriteHeader(599)
	}
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		fail("reading the request body: %v", err)
		return
	}
	out, err := http.NewRequestWithContext(req.Context(), req.Method, r.target+req.URL.RequestURI(), bytes.NewReader(reqBody))
	if err != nil {
		fail("%v", err)
		return
	}
	out.Header = req.Header.Clone()
	// Without it, the client asks for and decompresses gzip itself, so bodies read as JSON.
	out.Header.Del("Accept-Encoding")
	resp, err := http.DefaultClient.Do(out)
	if err != nil {
		fail("no HTTP response: %v", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck // A read-only body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fail("reading the response body: %v", err)
		return
	}

	r.mu.Lock()
	if r.step == "" {
		// A retry of the transport (on 429 or 503) is unnamed too: a fixture holds one answer
		// per step, so a recording with retries is not kept.
		r.t.Errorf("recorder: unnamed exchange %s %s: a missing next, or a retry", req.Method, req.URL.RequestURI())
	}
	x := exchange{
		Scenario: r.scenario,
		Step:     r.step,
		Request:  recordedRequest{Method: req.Method, Path: req.URL.RequestURI(), Body: json.RawMessage("null")},
		Response: recordedResponse{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Body: nonJSON},
	}
	if len(reqBody) > 0 {
		x.Request.Body = reqBody
	}
	if json.Valid(respBody) {
		x.Response.Body = respBody
	}
	r.exchanges = append(r.exchanges, x)
	r.step = ""
	r.mu.Unlock()

	for k, v := range resp.Header {
		if k != "Content-Length" {
			w.Header()[k] = v
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

// write writes the recording, in the recording mode and when the test passed.
func (r *recorder) write(resource string) {
	dir := os.Getenv("CONTRACT_RECORD_DIR")
	if dir == "" || r.t.Failed() {
		return
	}
	var lines bytes.Buffer
	for _, x := range r.exchanges {
		b, err := json.Marshal(x)
		if err != nil {
			r.t.Errorf("recorder: %v", err)
			return
		}
		lines.Write(b)
		lines.WriteByte('\n')
	}
	secrets := strings.Join(r.secrets, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, resource+".jsonl"), lines.Bytes(), 0o600); err != nil {
		r.t.Errorf("recorder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, resource+".secrets"), []byte(secrets), 0o600); err != nil {
		r.t.Errorf("recorder: %v", err)
	}
}
