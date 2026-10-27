// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// ErrUnsupportedVersion is returned for a version of a minor that has no profile.
var ErrUnsupportedVersion = errors.New("unsupported Open WebUI version")

// An Operation is one call of the Open WebUI API, e.g. the get of a group.
type Operation string

// The operations whose error responses are mapped by the profiles (ADR-0001, "Not-found and
// conflict map").
const (
	OpCheckCredentials Operation = "credentials/check"

	OpGroupGet    Operation = "group/get"
	OpGroupUpdate Operation = "group/update"
	OpGroupDelete Operation = "group/delete"

	OpModelCreate       Operation = "model/create"
	OpModelGet          Operation = "model/get"
	OpModelUpdate       Operation = "model/update"
	OpModelUpdateAccess Operation = "model/update-access"
	OpModelDelete       Operation = "model/delete"

	OpPromptCreate       Operation = "prompt/create"
	OpPromptGet          Operation = "prompt/get"
	OpPromptUpdate       Operation = "prompt/update"
	OpPromptToggle       Operation = "prompt/toggle"
	OpPromptUpdateAccess Operation = "prompt/update-access"
	OpPromptDelete       Operation = "prompt/delete"

	OpToolCreate       Operation = "tool/create"
	OpToolGet          Operation = "tool/get"
	OpToolUpdate       Operation = "tool/update"
	OpToolUpdateAccess Operation = "tool/update-access"
	OpToolGetValves    Operation = "tool/get-valves"
	OpToolUpdateValves Operation = "tool/update-valves"
	OpToolDelete       Operation = "tool/delete"

	OpFunctionCreate       Operation = "function/create"
	OpFunctionGet          Operation = "function/get"
	OpFunctionUpdate       Operation = "function/update"
	OpFunctionToggle       Operation = "function/toggle"
	OpFunctionToggleGlobal Operation = "function/toggle-global"
	OpFunctionGetValves    Operation = "function/get-valves"
	OpFunctionUpdateValves Operation = "function/update-valves"
	OpFunctionDelete       Operation = "function/delete"

	OpKnowledgeGet          Operation = "knowledge/get"
	OpKnowledgeUpdate       Operation = "knowledge/update"
	OpKnowledgeUpdateAccess Operation = "knowledge/update-access"
	OpKnowledgeDelete       Operation = "knowledge/delete"
)

// Details of Open WebUI error responses, the same on both minors, that tell what a status
// means on endpoints that use it for several things.
const (
	// accessProhibited refuses a principal that lacks a role or permission.
	accessProhibited = "You do not have permission to access this resource."
	// notFound is Open WebUI's generic not-found message, whatever the status.
	notFound = "We could not find what you're looking for"
	// alreadyRegistered refuses a duplicate model id, prompt command, or tool or function id.
	alreadyRegistered = "is already registered"
)

// sharedStatusErrors are the quirks common to every minor. A 404 needs none: it is
// ErrNotFound by default.
var sharedStatusErrors = map[Operation][]transport.StatusError{
	// A non-admin key answers 401, as an invalid one does, but with another detail.
	OpCheckCredentials: {{Status: http.StatusUnauthorized, Detail: accessProhibited, Err: transport.ErrForbidden}},

	OpModelCreate: {{Status: http.StatusUnauthorized, Detail: alreadyRegistered, Err: transport.ErrConflict}},
	OpModelUpdate: {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
	OpModelDelete: {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},

	OpPromptCreate: {{Status: http.StatusBadRequest, Detail: alreadyRegistered, Err: transport.ErrConflict}},
	OpPromptUpdate: {{Status: http.StatusBadRequest, Detail: alreadyRegistered, Err: transport.ErrConflict}},

	OpToolCreate:       {{Status: http.StatusBadRequest, Detail: alreadyRegistered, Err: transport.ErrConflict}},
	OpToolUpdate:       {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
	OpToolUpdateValves: {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
	OpToolDelete:       {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},

	OpFunctionCreate:       {{Status: http.StatusBadRequest, Detail: alreadyRegistered, Err: transport.ErrConflict}},
	OpFunctionGet:          {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
	OpFunctionToggle:       {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
	OpFunctionToggleGlobal: {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
	OpFunctionGetValves:    {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},
	OpFunctionUpdateValves: {{Status: http.StatusUnauthorized, Detail: notFound, Err: transport.ErrNotFound}},

	OpKnowledgeUpdate: {{Status: http.StatusBadRequest, Detail: notFound, Err: transport.ErrNotFound}},
	OpKnowledgeDelete: {{Status: http.StatusBadRequest, Detail: notFound, Err: transport.ErrNotFound}},
}

// sharedMissingSucceeds are the operations that answer success on a missing object on every
// minor: a delete answering 200 true, or the model access update, which re-creates the model.
var sharedMissingSucceeds = []Operation{OpModelUpdateAccess, OpFunctionDelete}

// A Profile holds what differs between the supported minors of the Open WebUI API.
type Profile struct {
	minor string

	// statusErrors maps the error responses of each operation that are quirks, i.e. that
	// do not mean what their status says.
	statusErrors map[Operation][]transport.StatusError

	// missingSucceeds are the operations that answer success on a missing object.
	missingSucceeds map[Operation]bool
}

// newProfile returns the profile of minor, with its own quirks on top of the shared ones.
func newProfile(minor string, statusErrors map[Operation][]transport.StatusError, missingSucceeds ...Operation) *Profile {
	p := &Profile{minor: minor, statusErrors: map[Operation][]transport.StatusError{}, missingSucceeds: map[Operation]bool{}}
	for op, se := range statusErrors {
		p.statusErrors[op] = append(p.statusErrors[op], se...)
	}
	for op, se := range sharedStatusErrors {
		p.statusErrors[op] = append(p.statusErrors[op], se...)
	}
	for _, op := range append(missingSucceeds, sharedMissingSucceeds...) {
		p.missingSucceeds[op] = true
	}
	return p
}

// Minor returns the minor the profile supports, e.g. 0.12.
func (p *Profile) Minor() string {
	return p.minor
}

// StatusErrors returns the quirks of op, for transport.Request.StatusErrors.
func (p *Profile) StatusErrors(op Operation) []transport.StatusError {
	return p.statusErrors[op]
}

// MissingSucceeds reports whether op answers success on a missing object. Its success
// proves nothing: a delete is confirmed by a get, and an access update is only sent for an
// object a get found (ADR-0001).
func (p *Profile) MissingSucceeds(op Operation) bool {
	return p.missingSucceeds[op]
}

// profiles are the supported minors, oldest first.
var profiles = []*Profile{
	v011,
	v012,
}

// SelectProfile returns the profile of the minor of v. For a minor without a profile, it
// returns ErrUnsupportedVersion, unless allowUnsupported is set: it then returns the newest
// profile and reports v as unsupported, for the caller to emit a warning.
func SelectProfile(v Version, allowUnsupported bool) (p *Profile, unsupported bool, err error) {
	for _, candidate := range profiles {
		if candidate.minor == v.MinorString() {
			return candidate, false, nil
		}
	}
	if allowUnsupported {
		return profiles[len(profiles)-1], true, nil
	}
	return nil, false, UnsupportedVersion(v)
}

// UnsupportedVersion returns the error of v, a version whose minor has no profile. It matches
// ErrUnsupportedVersion, and its message gives the supported range.
func UnsupportedVersion(v Version) error {
	return unsupportedVersionError{v: v}
}

// An unsupportedVersionError is ErrUnsupportedVersion for a version, with the supported
// range in its message.
type unsupportedVersionError struct{ v Version }

func (e unsupportedVersionError) Error() string {
	return fmt.Sprintf("Open WebUI v%s is not in the supported range [%s]", e.v, supportedMinors())
}

func (e unsupportedVersionError) Is(target error) bool {
	return target == ErrUnsupportedVersion
}

// supportedMinors lists the minors that have a profile.
func supportedMinors() string {
	m := make([]string, len(profiles))
	for i, p := range profiles {
		m[i] = p.minor
	}
	return strings.Join(m, ", ")
}
