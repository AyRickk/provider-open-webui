//go:build contract

// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/AyRickk/provider-openwebui/internal/client"
	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// TestCredentials is the contract of version detection and of the credential check (ADR-0001):
// the version of the instance is detected, its profile selected, and the check accepts the
// admin's key, refuses an unknown key as unauthorized, and a non-admin user's key as
// forbidden. It records the fixtures of test/fixtures/openwebui/v<version>/credentials/.
func TestCredentials(t *testing.T) {
	inst := instanceFromEnv(t, instanceDefault)
	rec := record(t, inst, "credentials")
	admin := rec.transport(inst.APIKey)

	rec.begin("check")
	rec.next("version")
	v, p := profileOf(t, admin)
	if v.String() != inst.Version {
		t.Fatalf("DetectVersion(...): got %s, want %s", v, inst.Version)
	}
	if p.Minor() != v.MinorString() {
		t.Errorf("SelectProfile(%s, false): got profile %s, want %s", v, p.Minor(), v.MinorString())
	}

	rec.next("admin-key")
	checkCredentials(t, "admin's key", p, admin, nil)
	rec.next("invalid-key")
	checkCredentials(t, "unknown key", p, rec.transport("sk-00000000000000000000000000000000"), transport.ErrUnauthorized)

	userKey := nonAdminKey(t, inst, rec, admin)
	rec.begin("non-admin")
	rec.next("user-key")
	checkCredentials(t, "non-admin user's key", p, rec.transport(userKey), transport.ErrForbidden)
}

// checkCredentials checks that the credential check with d returns want, which may be nil.
func checkCredentials(t *testing.T, key string, p *client.Profile, d client.Doer, want error) {
	t.Helper()
	err := p.CheckCredentials(context.Background(), d)
	if diff := cmp.Diff(want, err, cmpopts.EquateErrors()); diff != "" {
		t.Errorf("CheckCredentials(%s): -want, +got:\n%s", key, diff)
	}
}

// nonAdminKey creates a non-admin user and returns its API key, recording the setup as the
// scenario non-admin-setup. A user may only create an API key once the default permissions
// allow it. Signing in and creating the key are not recorded: they carry the user's secrets.
func nonAdminKey(t *testing.T, inst instance, rec *recorder, admin client.Doer) string {
	t.Helper()
	ctx := context.Background()
	const email, password = "capture-user@example.com", "throwaway-user-password"
	rec.secret(password)

	rec.begin("non-admin-setup")
	rec.next("add-user")
	user := struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}{Name: "Capture User", Email: email, Password: password, Role: "user"}
	if err := admin.Do(ctx, transport.Request{Method: http.MethodPost, Endpoint: "/api/v1/auths/add", Body: user}, nil); err != nil {
		t.Fatalf("adding a user: %v", err)
	}
	rec.next("get-default-permissions")
	var defaults json.RawMessage
	if err := admin.Do(ctx, transport.Request{Method: http.MethodGet, Endpoint: "/api/v1/users/default/permissions"}, &defaults); err != nil {
		t.Fatalf("reading the default permissions: %v", err)
	}
	allowed, err := setMember(defaults, true, "features", "api_keys")
	if err != nil {
		t.Fatalf("setting features.api_keys: %v", err)
	}
	rec.next("allow-api-keys")
	if err := admin.Do(ctx, transport.Request{Method: http.MethodPost, Endpoint: "/api/v1/users/default/permissions", Body: allowed}, nil); err != nil {
		t.Fatalf("allowing API keys: %v", err)
	}

	// Straight to the instance, with no auth header for the sign-in.
	var session struct {
		Token string `json:"token"`
	}
	signin, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	post(t, inst.URL+"/api/v1/auths/signin", "", signin, &session)
	var key struct {
		APIKey string `json:"api_key"`
	}
	post(t, inst.URL+"/api/v1/auths/api_key", session.Token, nil, &key)
	if session.Token == "" || key.APIKey == "" {
		t.Fatal("signing in as the user and creating its API key: empty token or key")
	}
	rec.secret(session.Token)
	rec.secret(key.APIKey)
	return key.APIKey
}

// post sends body to url, with token as a bearer token unless it is empty, and decodes the
// answer into out. Any other status than 200 fails the test.
func post(t *testing.T, url, token string, body []byte, out any) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", req.URL.Path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // A read-only body.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: status %d", req.URL.Path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("POST %s: decoding the answer: %v", req.URL.Path, err)
	}
}

// setMember returns the JSON object obj with the member at path set to value, keeping the
// order of the other members, as jq does: recorded request bodies keep the server's order.
func setMember(obj json.RawMessage, value any, path ...string) (json.RawMessage, error) {
	if len(path) == 0 {
		return json.Marshal(value)
	}
	type member struct {
		key   string
		value json.RawMessage
	}
	var members []member
	dec := json.NewDecoder(bytes.NewReader(obj))
	if _, err := dec.Token(); err != nil { // {
		return nil, err
	}
	found := false
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		if k == path[0] {
			if v, err = setMember(v, value, path[1:]...); err != nil {
				return nil, err
			}
			found = true
		}
		members = append(members, member{k, v})
	}
	if !found {
		return nil, fmt.Errorf("no member %q", path[0])
	}
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(m.key)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(m.value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
