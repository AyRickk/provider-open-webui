// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/providerconfig"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/AyRickk/provider-openwebui/apis/v1alpha1"
)

// Setup adds the controllers that reconcile ProviderConfigs and ClusterProviderConfigs by
// accounting for their current usage, and by reporting their connection to Open WebUI,
// made with c, in their status.
func Setup(mgr ctrl.Manager, o controller.Options, c *Connector) error {
	if err := setupNamespacedProviderConfig(mgr, o, c); err != nil {
		return err
	}
	return setupClusterProviderConfig(mgr, o, c)
}

// newStatusReconciler returns the reconciler of the provider configs of, which reconciles
// their usage with usage first.
func newStatusReconciler(mgr ctrl.Manager, o controller.Options, c *Connector, of resource.ProviderConfigKinds, usage reconcile.Reconciler, record event.Recorder) *statusReconciler {
	return &statusReconciler{
		kube:      mgr.GetClient(),
		usage:     usage,
		connector: c,
		newConfig: func() resource.ProviderConfig {
			return resource.MustCreateObject(of.Config, mgr.GetScheme()).(resource.ProviderConfig) //nolint:forcetypeassert // Always a provider config.
		},
		record:   record,
		interval: o.PollInterval,
	}
}

func setupNamespacedProviderConfig(mgr ctrl.Manager, o controller.Options, c *Connector) error {
	name := providerconfig.ControllerName(v1alpha1.ProviderConfigGroupKind)

	of := resource.ProviderConfigKinds{
		Config:    v1alpha1.ProviderConfigGroupVersionKind,
		Usage:     v1alpha1.ProviderConfigUsageGroupVersionKind,
		UsageList: v1alpha1.ProviderConfigUsageListGroupVersionKind,
	}

	record := event.NewAPIRecorder(mgr.GetEventRecorderFor(name)) //nolint:staticcheck // TODO(jbw976) Crossplane needs to update to the new events API, see https://github.com/crossplane/crossplane/issues/7152
	usage := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(record))
	r := newStatusReconciler(mgr, o, c, of, usage, record)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.ProviderConfig{}).
		Watches(&v1alpha1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{Kind: v1alpha1.ProviderConfigKind}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

func setupClusterProviderConfig(mgr ctrl.Manager, o controller.Options, c *Connector) error {
	name := providerconfig.ControllerName(v1alpha1.ClusterProviderConfigGroupKind)

	// Every MR in this provider is namespaced, so every usage record is a namespaced
	// ProviderConfigUsage. There is no cluster scoped usage type, even for a ClusterProviderConfig.
	// Instead the usage lives beside the namespaced MR that created it and records which kind of
	// config it refers to.
	of := resource.ProviderConfigKinds{
		Config:    v1alpha1.ClusterProviderConfigGroupVersionKind,
		Usage:     v1alpha1.ProviderConfigUsageGroupVersionKind,
		UsageList: v1alpha1.ProviderConfigUsageListGroupVersionKind,
	}

	record := event.NewAPIRecorder(mgr.GetEventRecorderFor(name)) //nolint:staticcheck // TODO(jbw976) Crossplane needs to update to the new events API, see https://github.com/crossplane/crossplane/issues/7152
	usage := providerconfig.NewReconciler(mgr, of,
		providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
		providerconfig.WithRecorder(record))
	r := newStatusReconciler(mgr, o, c, of, usage, record)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.ClusterProviderConfig{}).
		Watches(&v1alpha1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{Kind: v1alpha1.ClusterProviderConfigKind}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}
