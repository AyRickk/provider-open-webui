// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseVersion(t *testing.T) {
	type want struct {
		v   Version
		err bool
	}
	cases := map[string]struct {
		reason string
		in     string
		want   want
	}{
		"Release": {
			reason: "A release is major.minor.patch, as GET /api/version answers it.",
			in:     v0124,
			want:   want{v: Version{Major: 0, Minor: 12, Patch: 4}},
		},
		"MultiDigit": {
			reason: "Each part may have several digits.",
			in:     "1.10.123",
			want:   want{v: Version{Major: 1, Minor: 10, Patch: 123}},
		},
		"PrefixAndSuffix": {
			reason: "A v prefix, a pre-release or build suffix and spaces are ignored.",
			in:     " v0.11.4-dev+abc ",
			want:   want{v: Version{Major: 0, Minor: 11, Patch: 4}},
		},
		"MissingPatch": {
			reason: "A version without patch is invalid.",
			in:     minor012,
			want:   want{err: true},
		},
		"Empty": {
			reason: "An empty version is invalid.",
			in:     "",
			want:   want{err: true},
		},
		"NotANumber": {
			reason: "Parts must be numbers.",
			in:     "0.x.4",
			want:   want{err: true},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseVersion(tc.in)
			if (err != nil) != tc.want.err {
				t.Fatalf("\n%s\nParseVersion(%q): error %v, want error %t", tc.reason, tc.in, err, tc.want.err)
			}
			if diff := cmp.Diff(tc.want.v, got); diff != "" {
				t.Errorf("\n%s\nParseVersion(%q): -want, +got:\n%s", tc.reason, tc.in, diff)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	v := Version{Major: 0, Minor: 12, Patch: 4}
	if got, want := v.String(), "0.12.4"; got != want {
		t.Errorf("String(): got %q, want %q", got, want)
	}
	if got, want := v.MinorString(), minor012; got != want {
		t.Errorf("MinorString(): got %q, want %q", got, want)
	}
}
