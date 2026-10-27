// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"net/http"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// v011 is the profile of Open WebUI 0.11, observed on 0.11.4.
var v011 = newProfile("0.11",
	map[Operation][]transport.StatusError{
		OpGroupGet:       {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
		OpGroupUpdate:    {{Status: http.StatusBadRequest, Detail: "[ERROR: Error updating group]", Err: transport.ErrNotFound}},
		OpFunctionUpdate: {{Status: http.StatusBadRequest, Detail: "[ERROR: Error updating function]", Err: transport.ErrNotFound}},
	},
	// A missing group's delete answers 200 true.
	OpGroupDelete,
)
