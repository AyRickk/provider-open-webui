// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/AyRickk/provider-openwebui/internal/client/transport"
	"github.com/AyRickk/provider-openwebui/internal/controller/config"
)

// SetupGated creates all OpenWebUI controllers with safe-start support and adds them to
// the supplied manager. They share one connector, hence one version cache and one request
// counter.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	m, err := transport.NewMetrics(metrics.Registry)
	if err != nil {
		return errors.Wrap(err, "cannot register the Open WebUI request counter")
	}
	c := config.NewConnector(mgr.GetClient(), config.WithMetrics(m), config.WithLogger(o.Logger))
	for _, setup := range []func(ctrl.Manager, controller.Options, *config.Connector) error{
		config.Setup,
	} {
		if err := setup(mgr, o, c); err != nil {
			return err
		}
	}
	return nil
}
