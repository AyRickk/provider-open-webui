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
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/AyRickk/provider-openwebui/apis/v1alpha1"
)

// Setup adds the controllers that reconcile ProviderConfigs and ClusterProviderConfigs by
// accounting for their current usage, and by reporting their connection to Open WebUI,
// made with c, in their status.
func Setup(mgr ctrl.Manager, o controller.Options, c *Connector) error {
	if err := setup(mgr, o, c, v1alpha1.ProviderConfigGroupVersionKind, &v1alpha1.ProviderConfig{}); err != nil {
		return err
	}
	return setup(mgr, o, c, v1alpha1.ClusterProviderConfigGroupVersionKind, &v1alpha1.ClusterProviderConfig{})
}

// setup adds the controller of the provider config kind gvk, whose objects look like obj.
func setup(mgr ctrl.Manager, o controller.Options, c *Connector, gvk schema.GroupVersionKind, obj kclient.Object) error {
	name := providerconfig.ControllerName(gvk.GroupKind().String())

	// Every MR in this provider is namespaced, so every usage record is a namespaced
	// ProviderConfigUsage. There is no cluster scoped usage type, even for a ClusterProviderConfig.
	// Instead the usage lives beside the namespaced MR that created it and records which kind of
	// config it refers to.
	kinds := resource.ProviderConfigKinds{
		Config:    gvk,
		Usage:     v1alpha1.ProviderConfigUsageGroupVersionKind,
		UsageList: v1alpha1.ProviderConfigUsageListGroupVersionKind,
	}

	record := event.NewAPIRecorder(mgr.GetEventRecorderFor(name)) //nolint:staticcheck // TODO(jbw976) Crossplane needs to update to the new events API, see https://github.com/crossplane/crossplane/issues/7152
	r := &statusReconciler{
		kube: mgr.GetClient(),
		usage: providerconfig.NewReconciler(mgr, kinds,
			providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
			providerconfig.WithRecorder(record)),
		connector: c,
		newConfig: func() resource.ProviderConfig {
			return resource.MustCreateObject(gvk, mgr.GetScheme()).(resource.ProviderConfig) //nolint:forcetypeassert // Always a provider config.
		},
		record:   record,
		interval: o.PollInterval,
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(obj).
		Watches(&v1alpha1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{Kind: gvk.Kind}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}
