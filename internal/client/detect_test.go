// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// clock is a settable time source.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestVersionCacheDetects(t *testing.T) {
	cases := map[string]Version{
		v0114: {Major: 0, Minor: 11, Patch: 4},
		v0124: {Major: 0, Minor: 12, Patch: 4},
	}
	for version, want := range cases {
		t.Run(version, func(t *testing.T) {
			var n atomic.Int64
			tr := replay(t, &n, fixtureStep(t, version, "credentials", "check", "version"))
			got, err := NewVersionCache(DefaultVersionTTL).Get(context.Background(), "pc", tr)
			if err != nil {
				t.Fatalf("Get(...): %v", err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Get(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestVersionCacheTTL(t *testing.T) {
	type call struct {
		after time.Duration
		key   string
	}
	cases := map[string]struct {
		reason   string
		calls    []call
		requests int64
	}{
		"WithinTTL": {
			reason:   "A second read of the same provider config within 5 minutes is cached.",
			calls:    []call{{key: "a"}, {after: 5*time.Minute - time.Second, key: "a"}},
			requests: 1,
		},
		"AfterTTL": {
			reason:   "A read 5 minutes after the detection detects again.",
			calls:    []call{{key: "a"}, {after: 5 * time.Minute, key: "a"}},
			requests: 2,
		},
		"RefreshedAfterTTL": {
			reason:   "A detection after the TTL is cached for another 5 minutes.",
			calls:    []call{{key: "a"}, {after: 6 * time.Minute, key: "a"}, {after: 4 * time.Minute, key: "a"}},
			requests: 2,
		},
		"PerProviderConfig": {
			reason:   "Each provider config has its own cache entry.",
			calls:    []call{{key: "a"}, {key: "b"}, {key: "a"}, {key: "b"}},
			requests: 2,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var n atomic.Int64
			tr := replay(t, &n, fixtureStep(t, v0124, "credentials", "check", "version"))
			clk := &clock{t: time.Unix(1700000000, 0)}
			c := NewVersionCache(DefaultVersionTTL)
			c.now = clk.now
			for _, cl := range tc.calls {
				clk.t = clk.t.Add(cl.after)
				if _, err := c.Get(context.Background(), cl.key, tr); err != nil {
					t.Fatalf("Get(...): %v", err)
				}
			}
			if got := n.Load(); got != tc.requests {
				t.Errorf("\n%s\nGET /api/version: got %d requests, want %d", tc.reason, got, tc.requests)
			}
		})
	}
}

func TestVersionCacheDoesNotCacheErrors(t *testing.T) {
	var n atomic.Int64
	var fail atomic.Bool
	fail.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s := fixtureStep(t, v0124, "credentials", "check", "version")
		_, _ = w.Write(s.Response.Body)
	}))
	t.Cleanup(srv.Close)
	tr, err := transport.New(transport.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("transport.New(...): %v", err)
	}
	c := NewVersionCache(DefaultVersionTTL)
	if _, err := c.Get(context.Background(), "a", tr); err == nil {
		t.Fatal("Get(...) on a failing server: want error, got none")
	}
	fail.Store(false)
	got, err := c.Get(context.Background(), "a", tr)
	if err != nil {
		t.Fatalf("Get(...) after the server recovered: %v", err)
	}
	if diff := cmp.Diff(Version{Major: 0, Minor: 12, Patch: 4}, got); diff != "" {
		t.Errorf("Get(...): -want, +got:\n%s", diff)
	}
	if got := n.Load(); got != 2 {
		t.Errorf("GET /api/version: got %d requests, want 2: a failure is not cached", got)
	}
}
