// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/AyRickk/provider-openwebui/apis/v1alpha1"
	"github.com/AyRickk/provider-openwebui/internal/client/transport"
)

// recorder records the events of a reconciler.
type recorder struct{ events []event.Event }

func (r *recorder) Event(_ runtime.Object, e event.Event) { r.events = append(r.events, e) }

func (r *recorder) WithAnnotations(...string) event.Recorder { return r }

// usageReconciler stands in for crossplane-runtime's usage reconciler.
var usageReconciler = reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
})

// observed is what the reconciler reports of a provider config.
type observed struct {
	Version, Profile string
	Ready            corev1.ConditionStatus
	Reason           xpv2.ConditionReason
	Message          string
	Events           []event.Event
}

const interval = time.Minute

func TestReconcile(t *testing.T) {
	ok := []kclient.Object{secret("team", "owui-key", map[string]string{tokenKey: apiKey})}
	unauthorized := errors.Wrap(transport.ErrUnauthorized, "GET /api/v1/users/default/permissions: 401 Your session has expired or the token is invalid")
	cases := map[string]struct {
		reason string
		owui   fakeOpenWebUI
		objs   []kclient.Object
		pc     resource.ProviderConfig
		// reconciles is how many times the provider config is reconciled; 1 by default.
		reconciles int
		want       observed
	}{
		"Healthy": {
			reason: "A provider config whose instance is supported and accepts the key is ready, with the version and profile in its status.",
			owui:   fakeOpenWebUI{version: v0124},
			objs:   ok,
			pc:     providerConfig(),
			want:   observed{Version: v0124, Profile: minor012, Ready: corev1.ConditionTrue, Reason: xpv2.ReasonAvailable},
		},
		"HealthyCluster": {
			reason: "A ClusterProviderConfig reports its connection the same way.",
			owui:   fakeOpenWebUI{version: v0114},
			objs:   []kclient.Object{secret("crossplane-system", "owui-key", map[string]string{tokenKey: apiKey})},
			pc:     clusterProviderConfig(),
			want:   observed{Version: v0114, Profile: minor011, Ready: corev1.ConditionTrue, Reason: xpv2.ReasonAvailable},
		},
		"MissingSecret": {
			reason: "A provider config whose Secret is missing is unavailable, with a warning.",
			owui:   fakeOpenWebUI{version: v0124},
			pc:     providerConfig(),
			want: observed{
				Ready: corev1.ConditionFalse, Reason: xpv2.ReasonUnavailable,
				Message: `cannot read the API key: cannot get Secret team/owui-key: secrets "owui-key" not found`,
				Events: []event.Event{{
					Type: event.TypeWarning, Reason: reasonCannotConnect,
					Message:     `cannot read the API key: cannot get Secret team/owui-key: secrets "owui-key" not found`,
					Annotations: map[string]string{},
				}},
			},
		},
		"BadKey": {
			reason: "A provider config whose key is rejected is unavailable, and its last version is cleared.",
			owui:   fakeOpenWebUI{version: v0124, credentials: unauthorized},
			objs:   ok,
			pc: providerConfig(func(pc *v1alpha1.ProviderConfig) {
				pc.Status.Version, pc.Status.Profile = "0.12.3", "0.12"
			}),
			want: observed{
				Ready: corev1.ConditionFalse, Reason: xpv2.ReasonUnavailable,
				Message: "cannot check the Open WebUI credentials: GET /api/v1/users/default/permissions: 401 Your session has expired or the token is invalid: unauthorized",
				Events: []event.Event{{
					Type: event.TypeWarning, Reason: reasonCannotConnect,
					Message:     "cannot check the Open WebUI credentials: GET /api/v1/users/default/permissions: 401 Your session has expired or the token is invalid: unauthorized",
					Annotations: map[string]string{},
				}},
			},
		},
		"UnknownMinor": {
			reason: "A provider config whose instance runs an unknown minor is unavailable, with the supported range.",
			owui:   fakeOpenWebUI{version: v0130},
			objs:   ok,
			pc:     providerConfig(),
			want: observed{
				Ready: corev1.ConditionFalse, Reason: xpv2.ReasonUnavailable,
				Message: "Open WebUI v0.13.0 is not in the supported range [0.11, 0.12]",
				Events: []event.Event{{
					Type: event.TypeWarning, Reason: reasonCannotConnect,
					Message:     "Open WebUI v0.13.0 is not in the supported range [0.11, 0.12]",
					Annotations: map[string]string{},
				}},
			},
		},
		"UnknownMinorAllowed": {
			reason: "With allowUnsupportedVersions, an unknown minor is ready with the newest profile and a warning.",
			owui:   fakeOpenWebUI{version: v0130},
			objs:   ok,
			pc: providerConfig(func(pc *v1alpha1.ProviderConfig) {
				pc.Spec.AllowUnsupportedVersions = true
			}),
			want: observed{
				Version: v0130, Profile: minor012, Ready: corev1.ConditionTrue, Reason: xpv2.ReasonAvailable,
				Message: "Open WebUI v0.13.0 is not in the supported range [0.11, 0.12]: using the 0.12 profile, as allowUnsupportedVersions is set",
				Events: []event.Event{{
					Type: event.TypeWarning, Reason: reasonUnsupportedVersion,
					Message:     "Open WebUI v0.13.0 is not in the supported range [0.11, 0.12]: using the 0.12 profile, as allowUnsupportedVersions is set",
					Annotations: map[string]string{},
				}},
			},
		},
		"EventsOnChangeOnly": {
			reason:     "Reconciling an unchanged failure again emits no second event.",
			owui:       fakeOpenWebUI{version: v0124, credentials: unauthorized},
			objs:       ok,
			pc:         providerConfig(),
			reconciles: 3,
			want: observed{
				Ready: corev1.ConditionFalse, Reason: xpv2.ReasonUnavailable,
				Message: "cannot check the Open WebUI credentials: GET /api/v1/users/default/permissions: 401 Your session has expired or the token is invalid: unauthorized",
				Events: []event.Event{{
					Type: event.TypeWarning, Reason: reasonCannotConnect,
					Message:     "cannot check the Open WebUI credentials: GET /api/v1/users/default/permissions: 401 Your session has expired or the token is invalid: unauthorized",
					Annotations: map[string]string{},
				}},
			},
		},
		"UnsupportedWarningOnce": {
			reason: "Reconciling an allowed unsupported version again emits no second warning.",
			owui:   fakeOpenWebUI{version: v0130},
			objs:   ok,
			pc: providerConfig(func(pc *v1alpha1.ProviderConfig) {
				pc.Spec.AllowUnsupportedVersions = true
			}),
			reconciles: 2,
			want: observed{
				Version: v0130, Profile: minor012, Ready: corev1.ConditionTrue, Reason: xpv2.ReasonAvailable,
				Message: "Open WebUI v0.13.0 is not in the supported range [0.11, 0.12]: using the 0.12 profile, as allowUnsupportedVersions is set",
				Events: []event.Event{{
					Type: event.TypeWarning, Reason: reasonUnsupportedVersion,
					Message:     "Open WebUI v0.13.0 is not in the supported range [0.11, 0.12]: using the 0.12 profile, as allowUnsupportedVersions is set",
					Annotations: map[string]string{},
				}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kc := kube(t, append(tc.objs, tc.pc)...)
			rec := &recorder{}
			r := &statusReconciler{
				kube:      kc,
				usage:     usageReconciler,
				connector: NewConnector(kc, WithNewDoer(tc.owui.newDoer)),
				newConfig: func() resource.ProviderConfig { return tc.pc.DeepCopyObject().(resource.ProviderConfig) },
				record:    rec,
				interval:  interval,
			}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: tc.pc.GetNamespace(), Name: tc.pc.GetName()}}
			for range max(tc.reconciles, 1) {
				res, err := r.Reconcile(context.Background(), req)
				if err != nil {
					t.Fatalf("\n%s\nReconcile(...): %v", tc.reason, err)
				}
				if res.RequeueAfter != interval {
					t.Errorf("\n%s\nReconcile(...): requeue after %s, want %s", tc.reason, res.RequeueAfter, interval)
				}
			}

			pc := r.newConfig()
			if err := kc.Get(context.Background(), req.NamespacedName, pc); err != nil {
				t.Fatal(err)
			}
			st := statusOf(pc)
			ready := pc.GetCondition(xpv2.TypeReady)
			got := observed{
				Version: st.Version, Profile: st.Profile,
				Ready: ready.Status, Reason: ready.Reason, Message: ready.Message,
				Events: rec.events,
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nReconcile(...): -want, +got:\n%s", tc.reason, diff)
			}

			b, err := json.Marshal(pc)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), apiKey) {
				t.Errorf("\n%s\nReconcile(...): the provider config contains the API key: %s", tc.reason, b)
			}
			for _, e := range rec.events {
				if strings.Contains(e.Message, apiKey) {
					t.Errorf("\n%s\nReconcile(...): event %q contains the API key", tc.reason, e.Message)
				}
			}
		})
	}
}

func TestReconcileNeverShowsTheKey(t *testing.T) {
	// The server echoes the auth header in its errors, as a misconfigured proxy may.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			_, _ = w.Write([]byte(`{"version":"` + v0124 + `"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "invalid key " + r.Header.Get("x-api-key")})
	}))
	defer srv.Close()

	pc := providerConfig(func(pc *v1alpha1.ProviderConfig) {
		pc.Spec.URL = srv.URL
		pc.Spec.AuthHeader = "x-api-key"
	})
	kc := kube(t, pc, secret("team", "owui-key", map[string]string{tokenKey: apiKey}))
	rec := &recorder{}
	r := &statusReconciler{
		kube:      kc,
		usage:     usageReconciler,
		connector: NewConnector(kc),
		newConfig: func() resource.ProviderConfig { return &v1alpha1.ProviderConfig{} },
		record:    rec,
		interval:  interval,
	}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "team", Name: "owui"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("Reconcile(...): %v", err)
	}

	got := &v1alpha1.ProviderConfig{}
	if err := kc.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if c := got.GetCondition(xpv2.TypeReady); c.Status != corev1.ConditionFalse || !strings.Contains(c.Message, "invalid key [REDACTED]") {
		t.Errorf("Reconcile(...): want Ready=False with the redacted server detail, got %s: %q", c.Status, c.Message)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), apiKey) {
		t.Errorf("Reconcile(...): the provider config contains the API key: %s", b)
	}
	if len(rec.events) == 0 {
		t.Errorf("Reconcile(...): want a warning event, got none")
	}
	for _, e := range rec.events {
		if strings.Contains(e.Message, apiKey) {
			t.Errorf("Reconcile(...): event %q contains the API key", e.Message)
		}
	}
}
