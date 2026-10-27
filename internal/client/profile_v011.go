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
		OpGroupGet: {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
		// A catch-all detail, observed only for a missing group.
		OpGroupUpdate: {{Status: http.StatusBadRequest, Detail: "[ERROR: Error updating group]", Err: transport.ErrNotFound}},
		// OpFunctionUpdate has none: its 400 "[ERROR: Error updating function]" answers both a
		// missing function and invalid code, so only a get tells them apart.
	},
	// A missing group's delete answers 200 true.
	OpGroupDelete,
)
