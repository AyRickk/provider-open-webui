// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSelectProfile(t *testing.T) {
	type args struct {
		v                Version
		allowUnsupported bool
	}
	type want struct {
		minor       string
		unsupported bool
		err         error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"V011": {
			reason: "0.11.4 uses the 0.11 profile.",
			args:   args{v: Version{0, 11, 4}},
			want:   want{minor: minor011},
		},
		"V012": {
			reason: "0.12.4 uses the 0.12 profile.",
			args:   args{v: Version{0, 12, 4}},
			want:   want{minor: minor012},
		},
		"UnknownPatch": {
			reason: "An unknown patch of a supported minor is accepted silently.",
			args:   args{v: Version{0, 12, 9}},
			want:   want{minor: minor012},
		},
		"UnknownMinor": {
			reason: "An unknown minor is an error.",
			args:   args{v: Version{0, 13, 0}},
			want:   want{err: ErrUnsupportedVersion},
		},
		"UnknownMajor": {
			reason: "A supported minor number of another major is an unknown minor.",
			args:   args{v: Version{1, 12, 4}},
			want:   want{err: ErrUnsupportedVersion},
		},
		"UnknownMinorAllowed": {
			reason: "With allowUnsupportedVersions, an unknown minor uses the newest profile and is reported unsupported.",
			args:   args{v: Version{0, 13, 0}, allowUnsupported: true},
			want:   want{minor: minor012, unsupported: true},
		},
		"OlderMinorAllowed": {
			reason: "An older unknown minor uses the newest profile too.",
			args:   args{v: Version{0, 10, 2}, allowUnsupported: true},
			want:   want{minor: minor012, unsupported: true},
		},
		"SupportedMinorAllowed": {
			reason: "allowUnsupportedVersions changes nothing for a supported minor.",
			args:   args{v: Version{0, 11, 4}, allowUnsupported: true},
			want:   want{minor: minor011},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, unsupported, err := SelectProfile(tc.args.v, tc.args.allowUnsupported)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("\n%s\nSelectProfile(%s, %t): -want error, +got error:\n%s", tc.reason, tc.args.v, tc.args.allowUnsupported, diff)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tc.want, want{minor: p.Minor(), unsupported: unsupported}, cmp.AllowUnexported(want{})); diff != "" {
				t.Errorf("\n%s\nSelectProfile(%s, %t): -want, +got:\n%s", tc.reason, tc.args.v, tc.args.allowUnsupported, diff)
			}
		})
	}
}
