// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

func TestCheckCredentials(t *testing.T) {
	cases := map[string]struct {
		reason   string
		scenario string
		step     string
		want     error
	}{
		"AdminKey": {
			reason:   "The admin's API key passes the check.",
			scenario: "check",
			step:     "admin-key",
		},
		"InvalidKey": {
			reason:   "A key the server does not know answers 401: ErrUnauthorized.",
			scenario: "check",
			step:     "invalid-key",
			want:     transport.ErrUnauthorized,
		},
		"NonAdminKey": {
			reason:   "A non-admin user's key also answers 401, with the access-prohibited detail: ErrForbidden.",
			scenario: "non-admin",
			step:     "user-key",
			want:     transport.ErrForbidden,
		},
	}
	for _, version := range fixtureVersions {
		v, err := ParseVersion(version)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", version, err)
		}
		p, _, err := SelectProfile(v, false)
		if err != nil {
			t.Fatalf("SelectProfile(%s): %v", v, err)
		}
		for name, tc := range cases {
			t.Run(version+"/"+name, func(t *testing.T) {
				var n atomic.Int64
				tr := replay(t, &n, fixtureStep(t, version, "credentials", tc.scenario, tc.step))
				err := p.CheckCredentials(context.Background(), tr)
				if diff := cmp.Diff(tc.want, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nCheckCredentials(...): -want, +got:\n%s", tc.reason, diff)
				}
				for _, s := range []error{transport.ErrUnauthorized, transport.ErrForbidden} {
					if !errors.Is(tc.want, s) && errors.Is(err, s) {
						t.Errorf("\n%s\nCheckCredentials(...): error %q also matches %q", tc.reason, err, s)
					}
				}
			})
		}
	}
}
