// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// The versions with committed fixtures, and the minors of their profiles.
const (
	v0114    = "0.11.4"
	v0124    = "0.12.4"
	minor011 = "0.11"
	minor012 = "0.12"
)

// fixtureVersions are the versions with committed fixtures.
var fixtureVersions = []string{v0114, v0124}

// A step is one recorded exchange of a fixture under test/fixtures/openwebui/.
type step struct {
	Step    string `json:"step"`
	Request struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	} `json:"request"`
	Response struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	} `json:"response"`
}

// fixtureStep returns the step named name of fixture v<version>/<resource>/<scenario>.json.
func fixtureStep(t *testing.T, version, resource, scenario, name string) step {
	t.Helper()
	path := filepath.Join("..", "..", "test", "fixtures", "openwebui", "v"+version, resource, scenario+".json")
	b, err := os.ReadFile(path) //nolint:gosec // A path built from test constants.
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	var f struct {
		Steps []step `json:"steps"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("Unmarshal(%s): %v", path, err)
	}
	for _, s := range f.Steps {
		if s.Step == name {
			return s
		}
	}
	t.Fatalf("%s has no step %q", path, name)
	return step{}
}

// profileFor returns the profile SelectProfile picks for version.
func profileFor(t *testing.T, version string) *Profile {
	t.Helper()
	v, err := ParseVersion(version)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", version, err)
	}
	p, _, err := SelectProfile(v, false)
	if err != nil {
		t.Fatalf("SelectProfile(%s): %v", v, err)
	}
	return p
}

// checkNoOtherSentinel reports err if it matches a sentinel other than want, which may be nil.
func checkNoOtherSentinel(t *testing.T, call string, want, err error) {
	t.Helper()
	for _, s := range []error{transport.ErrNotFound, transport.ErrConflict, transport.ErrUnauthorized, transport.ErrForbidden} {
		if !errors.Is(want, s) && errors.Is(err, s) {
			t.Errorf("%s: error %q also matches %q", call, err, s)
		}
	}
}

// replay serves the recorded response of each step to a request with its method and path,
// query included, and 599 to any other request. It counts the requests it receives.
func replay(t *testing.T, count *atomic.Int64, steps ...step) *transport.Transport {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		for _, s := range steps {
			if s.Request.Method == r.Method && s.Request.Path == r.URL.RequestURI() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(s.Response.Status)
				_, _ = w.Write(s.Response.Body)
				return
			}
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
		w.WriteHeader(599)
	}))
	t.Cleanup(srv.Close)
	tr, err := transport.New(transport.Config{BaseURL: srv.URL, Token: "sk-0123456789abcdef0123456789abcdef"})
	if err != nil {
		t.Fatalf("transport.New(...): %v", err)
	}
	return tr
}
