// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Sentinel errors an Error matches with errors.Is, depending on its status code and on the
// request's StatusErrors.
var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// defaultStatusErrors maps a status code to its sentinel unless a request overrides it.
var defaultStatusErrors = map[int]error{
	http.StatusNotFound:     ErrNotFound,
	http.StatusConflict:     ErrConflict,
	http.StatusUnauthorized: ErrUnauthorized,
	http.StatusForbidden:    ErrForbidden,
}

const (
	// maxDetail is the length, in characters, of the server message kept in an Error.
	maxDetail = 512
	// maxErrorBody is how much of an error response is read.
	maxErrorBody = 64 << 10
	redacted     = "[REDACTED]"
)

// credential matches the shapes of secrets a server may echo: Open WebUI API keys, bearer
// credentials and JWTs (Open WebUI session tokens).
var credential = regexp.MustCompile(`\bsk-[A-Za-z0-9]{8,}|(?i:\bbearer\s+[^\s"',]+)|\beyJ[\w-]+\.[\w-]+\.[\w-]*`)

// An Error is an error response of the Open WebUI API.
type Error struct {
	// Method and Endpoint identify the request; Endpoint is its path template.
	Method   string
	Endpoint string

	// StatusCode is the HTTP status of the response.
	StatusCode int

	// Detail is the server's message: the detail of a FastAPI error body, or else the
	// body, with credentials redacted and cut to 512 characters.
	Detail string

	sentinel error
}

// Error returns a message such as `GET /api/v1/groups/id/{id}: 404 Not Found: "detail"`.
func (e *Error) Error() string {
	msg := fmt.Sprintf("%s %s: %d %s", e.Method, e.Endpoint, e.StatusCode, http.StatusText(e.StatusCode))
	if e.Detail != "" {
		msg += fmt.Sprintf(": %q", e.Detail)
	}
	return msg
}

// Unwrap returns the sentinel the status code maps to, if any.
func (e *Error) Unwrap() error {
	return e.sentinel
}

// newError builds the Error of a response with the supplied status and body.
func (t *Transport) newError(req Request, status int, body []byte) *Error {
	e := &Error{
		Method:     req.Method,
		Endpoint:   req.Endpoint,
		StatusCode: status,
		Detail:     truncate(t.redact(detail(body)), maxDetail),
		sentinel:   defaultStatusErrors[status],
	}
	for _, se := range req.StatusErrors {
		if se.Status == status && strings.Contains(e.Detail, se.Detail) {
			e.sentinel = se.Err
			break
		}
	}
	return e
}

// detail returns the detail of a FastAPI error body, or else the trimmed body.
func detail(body []byte) string {
	var b struct {
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(body, &b); err != nil || len(b.Detail) == 0 {
		return strings.TrimSpace(string(body))
	}
	var s string
	if err := json.Unmarshal(b.Detail, &s); err == nil {
		return s
	}
	return string(b.Detail)
}

// redact replaces the token and anything shaped like a credential.
func (t *Transport) redact(s string) string {
	if t.token != "" {
		s = strings.ReplaceAll(s, t.token, redacted)
	}
	return credential.ReplaceAllString(s, redacted)
}

// truncate cuts s to n characters, the last one an ellipsis.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
