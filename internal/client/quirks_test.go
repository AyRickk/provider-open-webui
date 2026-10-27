// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// An outcome is what an operation's response maps to.
type outcome int

const (
	noSentinel outcome = iota
	notFoundErr
	conflictErr
	unauthorizedErr
	// succeeds is the outcome of an operation that answers success on a missing object.
	succeeds
)

// sentinel returns the sentinel error of o, or nil.
func (o outcome) sentinel() error {
	switch o {
	case notFoundErr:
		return transport.ErrNotFound
	case conflictErr:
		return transport.ErrConflict
	case unauthorizedErr:
		return transport.ErrUnauthorized
	case noSentinel, succeeds:
	}
	return nil
}

// Fixture scenarios that several rows replay.
const (
	getMissing = "get-missing"
	create     = "create"
)

// Fixture steps that several rows replay.
const (
	getDeleted    = "get-deleted"
	updateDeleted = "update-deleted"
	deleteDeleted = "delete-deleted"
)

// A quirkRow is one recorded error response and what it must map to.
type quirkRow struct {
	// versions the row holds on; nil means every fixture version.
	versions []string
	op       Operation
	// scenario and step locate the fixture, under the row's resource.
	scenario, step string
	want           outcome
}

var (
	only011 = []string{v0114}
	only012 = []string{v0124}
)

// TestStatusErrors replays, through each profile's quirks, the error responses recorded under
// test/fixtures/openwebui/: every cell of ADR-0001's not-found and conflict map, its valves
// rows, and errors of the same endpoints that must keep no sentinel.
func TestStatusErrors(t *testing.T) {
	// Rows per fixture resource.
	cases := map[string][]quirkRow{
		"group": {
			{only011, OpGroupGet, getMissing, getDeleted, notFoundErr},
			{only011, OpGroupGet, getMissing, "get-never-existed", notFoundErr},
			{only012, OpGroupGet, getMissing, getDeleted, notFoundErr},
			{only011, OpGroupUpdate, getMissing, updateDeleted, notFoundErr},
			{only012, OpGroupUpdate, getMissing, updateDeleted, notFoundErr},
			{only011, OpGroupDelete, getMissing, deleteDeleted, succeeds},
			{only012, OpGroupDelete, getMissing, deleteDeleted, notFoundErr},
		},
		"model": {
			{nil, OpModelGet, getMissing, getDeleted, notFoundErr},
			{nil, OpModelUpdate, getMissing, updateDeleted, notFoundErr},
			{nil, OpModelDelete, getMissing, deleteDeleted, notFoundErr},
			{nil, OpModelUpdateAccess, getMissing, "update-access-deleted", succeeds},
			{nil, OpModelCreate, create, "create-duplicate-id", conflictErr},
		},
		"prompt": {
			{nil, OpPromptGet, getMissing, getDeleted, notFoundErr},
			{nil, OpPromptUpdate, getMissing, updateDeleted, notFoundErr},
			{nil, OpPromptUpdate, "update", "update-command-to-taken", conflictErr},
			{nil, OpPromptDelete, getMissing, deleteDeleted, notFoundErr},
			{nil, OpPromptToggle, getMissing, "toggle-deleted", notFoundErr},
			{nil, OpPromptUpdateAccess, getMissing, "update-access-deleted", notFoundErr},
			{nil, OpPromptCreate, create, "create-duplicate-command", conflictErr},
		},
		"tool": {
			{nil, OpToolGet, getMissing, getDeleted, notFoundErr},
			{nil, OpToolUpdate, getMissing, updateDeleted, notFoundErr},
			{nil, OpToolUpdate, "update", "update-syntax-error", noSentinel},
			{nil, OpToolDelete, getMissing, deleteDeleted, notFoundErr},
			{nil, OpToolUpdateAccess, getMissing, "update-access-deleted", notFoundErr},
			{nil, OpToolCreate, create, "create-duplicate-id", conflictErr},
			{nil, OpToolCreate, create, "create-syntax-error", noSentinel},
			{nil, OpToolCreate, create, "create-invalid-id", noSentinel},
			{nil, OpToolGetValves, getMissing, "valves-deleted", notFoundErr},
			{nil, OpToolUpdateValves, getMissing, "valves-update-deleted", notFoundErr},
			{nil, OpToolUpdateValves, "valves", "valves-update-wrong-type", noSentinel},
		},
		"function": {
			{nil, OpFunctionGet, getMissing, getDeleted, notFoundErr},
			// 0.11 answers a missing function and invalid code alike: no sentinel.
			{only011, OpFunctionUpdate, getMissing, updateDeleted, noSentinel},
			{only012, OpFunctionUpdate, getMissing, updateDeleted, notFoundErr},
			{nil, OpFunctionUpdate, "update", "update-no-class", noSentinel},
			{nil, OpFunctionDelete, getMissing, deleteDeleted, succeeds},
			{nil, OpFunctionCreate, create, "create-duplicate-id", conflictErr},
			{nil, OpFunctionCreate, create, "create-no-class", noSentinel},
			{nil, OpFunctionGetValves, getMissing, "valves-deleted", notFoundErr},
			{nil, OpFunctionUpdateValves, getMissing, "valves-update-deleted", notFoundErr},
			{nil, OpFunctionUpdateValves, "valves", "valves-update-wrong-type", noSentinel},
			{nil, OpFunctionToggle, getMissing, "toggle-deleted", notFoundErr},
			{nil, OpFunctionToggleGlobal, getMissing, "toggle-global-deleted", notFoundErr},
		},
		"knowledge": {
			{nil, OpKnowledgeGet, getMissing, getDeleted, notFoundErr},
			{nil, OpKnowledgeUpdate, getMissing, updateDeleted, notFoundErr},
			{nil, OpKnowledgeDelete, getMissing, deleteDeleted, notFoundErr},
			{nil, OpKnowledgeUpdateAccess, getMissing, "update-access-deleted", notFoundErr},
		},
		// An invalid key keeps ErrUnauthorized on an endpoint whose not-found is a 401.
		"credentials": {
			{nil, OpToolDelete, "check", "invalid-key", unauthorizedErr},
		},
	}
	for _, version := range fixtureVersions {
		p := profileFor(t, version)
		for resource, rows := range cases {
			for _, tc := range rows {
				if tc.versions != nil && !slices.Contains(tc.versions, version) {
					continue
				}
				name := version + "/" + string(tc.op) + "/" + resource + "/" + tc.scenario + "#" + tc.step
				t.Run(name, func(t *testing.T) {
					s := fixtureStep(t, version, resource, tc.scenario, tc.step)
					u, err := url.Parse(s.Request.Path)
					if err != nil {
						t.Fatalf("url.Parse(%q): %v", s.Request.Path, err)
					}
					var n atomic.Int64
					err = replay(t, &n, s).Do(context.Background(), transport.Request{
						Method:       s.Request.Method,
						Endpoint:     u.Path,
						Query:        u.Query(),
						StatusErrors: p.StatusErrors(tc.op),
					}, nil)
					if got := p.MissingSucceeds(tc.op); got != (tc.want == succeeds) {
						t.Errorf("MissingSucceeds(%s): got %t, want %t", tc.op, got, tc.want == succeeds)
					}
					want := tc.want.sentinel()
					switch {
					case tc.want == succeeds:
						if err != nil {
							t.Errorf("Do(...): got error %v, want success", err)
						}
						return
					case err == nil:
						t.Fatal("Do(...): got success, want an error")
					case want != nil && !errors.Is(err, want):
						t.Errorf("Do(...): got %v, want %v", err, want)
					}
					checkNoOtherSentinel(t, "Do(...)", want, err)
				})
			}
		}
	}
}
