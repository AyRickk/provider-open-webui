// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/AyRickk/provider-openwebui/apis/v1alpha1"
	"github.com/AyRickk/provider-openwebui/internal/client"
)

// The reasons of the events of a provider config.
const (
	reasonCannotConnect      event.Reason = "CannotConnect"
	reasonUnsupportedVersion event.Reason = "UnsupportedVersion"
)

// A statusReconciler reconciles the usage of a provider config, then connects to its Open
// WebUI instance and reports the outcome in its status: the version, the profile and the
// Ready condition. It emits an event when the Ready condition changes to an error or an
// unsupported version.
type statusReconciler struct {
	kube      kclient.Client
	usage     reconcile.Reconciler
	connector *Connector
	newConfig func() resource.ProviderConfig
	record    event.Recorder

	// interval is how often the connection is checked again.
	interval time.Duration
}

// Reconcile reconciles the provider config of req.
func (r *statusReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	res, err := r.usage.Reconcile(ctx, req)
	if err != nil {
		return res, err
	}

	pc := r.newConfig()
	if err := r.kube.Get(ctx, req.NamespacedName, pc); err != nil {
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get the provider config")
	}
	if meta.WasDeleted(pc) {
		return res, nil
	}

	r.observe(ctx, pc)

	if err := r.kube.Status().Update(ctx, pc); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update the provider config status")
	}
	if res.RequeueAfter > 0 && res.RequeueAfter < r.interval {
		return res, nil
	}
	return reconcile.Result{RequeueAfter: r.interval}, nil
}

// observe connects to the Open WebUI instance of pc and reports the outcome in its status.
// It emits a warning when the Ready condition changes to an error or an unsupported version.
func (r *statusReconciler) observe(ctx context.Context, pc resource.ProviderConfig) {
	st := statusOf(pc)
	st.Version, st.Profile = "", ""
	// warning is the event of a ready condition that is an error or an unsupported version.
	var ready xpv2.Condition
	var warning *event.Event
	conn, err := r.connector.ConnectConfig(ctx, pc)
	switch {
	case err != nil:
		ready = xpv2.Unavailable().WithMessage(err.Error())
		w := event.Warning(reasonCannotConnect, err)
		warning = &w
	case conn.Unsupported:
		err := errors.Errorf("%s: using the %s profile, as allowUnsupportedVersions is set", client.UnsupportedVersion(conn.Version), conn.Profile.Minor())
		ready = xpv2.Available().WithMessage(err.Error())
		w := event.Warning(reasonUnsupportedVersion, err)
		warning = &w
	default:
		ready = xpv2.Available()
	}
	if conn != nil {
		st.Version, st.Profile = conn.Version.String(), conn.Profile.Minor()
	}
	if warning != nil && !pc.GetCondition(xpv2.TypeReady).Equal(ready) {
		r.record.Event(pc, *warning)
	}
	pc.SetConditions(ready)
}

// statusOf returns the status of pc, a ProviderConfig or a ClusterProviderConfig.
func statusOf(pc resource.ProviderConfig) *v1alpha1.ProviderConfigStatus {
	switch pc := pc.(type) {
	case *v1alpha1.ProviderConfig:
		return &pc.Status
	case *v1alpha1.ClusterProviderConfig:
		return &pc.Status
	}
	return &v1alpha1.ProviderConfigStatus{}
}
