//go:build contract

// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/AyRickk/provider-openwebui/internal/client"
	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// TestEndpointRestrictions is the contract of the API-key allow-list of ADR-0001,
// docs/api-notes/api-key-allowed-endpoints.txt. With the API keys restricted to it, the
// provider still detects the version and checks its credentials, and read-only routes the
// provider never needs are forbidden. On the default instance, the same routes answer: their
// refusal comes from the restriction. The resource routes of the list get their contracts with
// each resource.
func TestEndpointRestrictions(t *testing.T) {
	inst := instanceFromEnv(t)
	ctx := context.Background()
	d := newTransport(t, inst.URL, inst.APIKey)
	restricted := inst.Kind == instanceRestricted

	v, err := client.DetectVersion(ctx, d)
	if err != nil {
		t.Fatalf("DetectVersion(...): %v", err)
	}
	p, _, err := client.SelectProfile(v, false)
	if err != nil {
		t.Fatalf("SelectProfile(%s, false): %v", v, err)
	}
	if err := p.CheckCredentials(ctx, d); err != nil {
		t.Errorf("CheckCredentials(...): %v", err)
	}

	cases := map[string]struct {
		endpoint string
		// unrestricted is the error on the default instance, per profile minor; nil by default.
		unrestricted map[string]error
	}{
		"SessionUser": {
			endpoint: "/api/v1/auths/",
			// 0.12 refuses API keys on routes that need a human session.
			unrestricted: map[string]error{"0.12": transport.ErrForbidden},
		},
		"ListUsers":    {endpoint: "/api/v1/users/"},
		"ListModels":   {endpoint: "/api/v1/models/all"},
		"OpenAIConfig": {endpoint: "/openai/config"},
		"ListFiles":    {endpoint: "/api/v1/files/"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want := tc.unrestricted[p.Minor()]
			if restricted {
				want = transport.ErrForbidden
			}
			err := d.Do(ctx, transport.Request{Method: http.MethodGet, Endpoint: tc.endpoint}, nil)
			if diff := cmp.Diff(want, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("GET %s on a %s instance: -want, +got:\n%s", tc.endpoint, inst.Kind, diff)
			}
		})
	}
}
