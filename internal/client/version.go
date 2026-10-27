// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package client is the version-aware layer of the Open WebUI client: it detects the
// version of an instance, selects the API profile of its minor, and checks the
// provider's credentials. The HTTP layer lives in package transport.
package client

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// A Version is an Open WebUI release.
type Version struct {
	Major, Minor, Patch int
}

// release matches major.minor.patch, with an optional v prefix and an optional pre-release
// or build suffix.
var release = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

// ParseVersion parses a version such as 0.12.4, as GET /api/version answers it.
func ParseVersion(s string) (Version, error) {
	m := release.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Version{}, errors.Errorf("invalid Open WebUI version %q: want major.minor.patch", s)
	}
	var parts [3]int
	for i, p := range m[1:] {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, errors.Wrapf(err, "invalid Open WebUI version %q", s)
		}
		parts[i] = n
	}
	return Version{Major: parts[0], Minor: parts[1], Patch: parts[2]}, nil
}

// String returns the version as major.minor.patch.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// MinorString returns the minor the version belongs to, as major.minor.
func (v Version) MinorString() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}
