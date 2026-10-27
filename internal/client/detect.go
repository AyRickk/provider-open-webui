// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// DefaultVersionTTL is how long a detected version is cached.
const DefaultVersionTTL = 5 * time.Minute

// A Doer sends requests to the Open WebUI API, as *transport.Transport does.
type Doer interface {
	Do(ctx context.Context, req transport.Request, out any) error
}

// A VersionCache detects the version of Open WebUI instances with GET /api/version, and
// caches it per provider config. It is safe for concurrent use.
type VersionCache struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[string]cachedVersion

	// now returns the current time; tests replace it.
	now func() time.Time
}

type cachedVersion struct {
	version  Version
	detected time.Time
}

// NewVersionCache returns a VersionCache that keeps a detected version for ttl.
func NewVersionCache(ttl time.Duration) *VersionCache {
	return &VersionCache{ttl: ttl, entries: map[string]cachedVersion{}, now: time.Now}
}

// Get returns the version of the instance d sends to, detected at most ttl ago for the
// provider config key. The key identifies a provider config and its connection settings, so
// that a changed URL is detected again. A failed detection is not cached.
func (c *VersionCache) Get(ctx context.Context, key string, d Doer) (Version, error) {
	now := c.now()
	c.mu.Lock()
	e, ok := c.entries[key]
	c.mu.Unlock()
	if ok && now.Sub(e.detected) < c.ttl {
		return e.version, nil
	}
	v, err := DetectVersion(ctx, d)
	if err != nil {
		return Version{}, err
	}
	c.mu.Lock()
	c.entries[key] = cachedVersion{version: v, detected: now}
	c.mu.Unlock()
	return v, nil
}

// DetectVersion reads the version of the instance d sends to, with GET /api/version.
func DetectVersion(ctx context.Context, d Doer) (Version, error) {
	var out struct {
		Version string `json:"version"`
	}
	if err := d.Do(ctx, transport.Request{Method: http.MethodGet, Endpoint: "/api/version"}, &out); err != nil {
		return Version{}, errors.Wrap(err, "cannot detect the Open WebUI version")
	}
	return ParseVersion(out.Version)
}
