// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// codeError is the code label of an attempt that got no response.
const codeError = "error"

// Metrics counts the requests sent to Open WebUI. One Metrics is shared by every
// Transport of the provider.
type Metrics struct {
	requests *prometheus.CounterVec
}

// NewMetrics creates the request counter and registers it with reg.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "openwebui_api_requests_total",
		Help: `Attempts of requests to the Open WebUI API, by method, endpoint template and status code ("error" when no response arrived).`,
	}, []string{"method", "endpoint", "code"})
	if err := reg.Register(c); err != nil {
		return nil, errors.Wrap(err, "cannot register the Open WebUI request counter")
	}
	return &Metrics{requests: c}, nil
}

// count counts one attempt; a nil Metrics counts nothing.
func (m *Metrics) count(method, endpoint, code string) {
	if m == nil {
		return
	}
	m.requests.WithLabelValues(method, endpoint, code).Inc()
}
