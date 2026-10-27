// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"net/http"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// CheckCredentials checks that the provider's API key is an admin's, with GET
// /api/v1/users/default/permissions (ADR-0001). The error matches transport.ErrUnauthorized
// when the server rejects the key, and transport.ErrForbidden when the key is not an
// admin's, or on a 403, e.g. when the API-key endpoint restrictions refuse the route.
func (p *Profile) CheckCredentials(ctx context.Context, d Doer) error {
	err := d.Do(ctx, transport.Request{
		Method:       http.MethodGet,
		Endpoint:     "/api/v1/users/default/permissions",
		StatusErrors: p.StatusErrors(OpCheckCredentials),
	}, nil)
	return errors.Wrap(err, "cannot check the Open WebUI credentials")
}
