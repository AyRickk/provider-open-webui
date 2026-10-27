// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"net/http"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// v012 is the profile of Open WebUI 0.12, observed on 0.12.4. Its group routes answer 404
// for a missing group, so they have no quirk.
var v012 = newProfile("0.12",
	map[Operation][]transport.StatusError{
		OpFunctionUpdate: {{Status: http.StatusBadRequest, Detail: "Function not found", Err: transport.ErrNotFound}},
	},
)
