/*
Copyright (c) 2022 RaptorML authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operator

// These are referring tho the values in FeatureWebhookValidatePath and FeatureWebhookMutatePath
// They are package-level markers, and should be as a standalone comment block
// +kubebuilder:webhook:path=/mutate-k8s-raptor-ml-v1alpha1-feature,mutating=true,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=k8s.raptor.ml,resources=features,verbs=create;update,versions=v1alpha1,name=vfeature.kb.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-k8s-raptor-ml-v1alpha1-feature,mutating=false,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=k8s.raptor.ml,resources=features,verbs=create;update,versions=v1alpha1,name=vfeature.kb.io,admissionReviewVersions=v1

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/raptor-ml/raptor/api"
	raptorApi "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/internal/engine"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	FeatureWebhookValidatePath = "/validate-k8s-raptor-ml-v1alpha1-feature"
	FeatureWebhookValidateName = "raptor-validating-webhook-configuration"
	FeatureWebhookMutatePath   = "/mutate-k8s-raptor-ml-v1alpha1-feature"
	FeatureWebhookMutateName   = "raptor-mutating-webhook-configuration"
)

func SetupFeatureWebhook(mgr ctrl.Manager, updatesAllowed bool) {
	impl := &webhook{
		updatesAllowed: updatesAllowed,
		client:         mgr.GetClient(),
		logger:         mgr.GetLogger().WithName("feature-webhook"),
	}
	wh := admission.WithCustomValidator(&raptorApi.Feature{}, impl)
	wh.Handler = &admissionWrapper{Handler: wh.Handler}

	mgr.GetWebhookServer().Register(FeatureWebhookValidatePath, wh)

	wh = admission.WithCustomDefaulter(&raptorApi.Feature{}, impl)
	wh.Handler = &admissionWrapper{Handler: wh.Handler}
	mgr.GetWebhookServer().Register(FeatureWebhookMutatePath, wh)
}

type webhook struct {
	client         client.Client
	logger         logr.Logger
	updatesAllowed bool
}

type ctxKey string

const admissionRequestContextKey ctxKey = "AdmissionRequest"

type admissionWrapper struct {
	admission.Handler
}

func (h *admissionWrapper) InjectDecoder(d *admission.Decoder) error {
	if di, ok := h.Handler.(admission.DecoderInjector); ok {
		return di.InjectDecoder(d)
	}
	return nil
}

func (h *admissionWrapper) Handle(ctx context.Context, req admission.Request) admission.Response {
	ctx = context.WithValue(ctx, admissionRequestContextKey, req)
	return h.Handler.Handle(ctx, req)
}

func (wh *webhook) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*raptorApi.Feature)
	if !ok {
		panic("obj is not *raptorApi.Feature")
	}
	wh.logger.Info("defaulting", "name", f.GetName())

	if f.Spec.DataConnector != nil && f.Spec.DataConnector.Namespace == "" {
		f.Spec.DataConnector.Namespace = f.GetNamespace()
	}
	if f.Spec.Builder.Kind == "" {
		if f.Spec.Builder.Kind == "" && f.Spec.DataConnector != nil {
			if ar, ok := ctx.Value(admissionRequestContextKey).(admission.Request); ok && ar.DryRun == nil || ok && !*ar.DryRun {
				dc := raptorApi.DataConnector{}
				err := wh.client.Get(ctx, f.Spec.DataConnector.ObjectKey(), &dc)
				if apierrors.IsNotFound(err) {
					return fmt.Errorf("data connector %s/%s not found: %w", f.Spec.DataConnector.Namespace, f.Spec.DataConnector.Name, err)
				}
				if err != nil {
					return fmt.Errorf("failed to get data connector: %w", err)
				}

				dci, err := api.DataConnectorFromManifest(ctx, &dc, wh.client)
				if err != nil {
					return fmt.Errorf("failed to get data connector instance: %w", err)
				}

				// Check if data-connector is mapped to a special builder
				if plugins.FeatureAppliers[dci.Kind] != nil {
					f.Spec.Builder.Kind = dci.Kind
				}
			}
		}
		if f.Spec.Builder.Kind == "" {
			f.Spec.Builder.Kind = api.ExpressionBuilder
		}

		if f.Spec.Builder.AggrGranularity.Milliseconds() > 0 && len(f.Spec.Builder.Aggr) > 0 {
			f.Spec.Freshness = f.Spec.Builder.AggrGranularity
		}
	}

	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (wh *webhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*raptorApi.Feature)
	if !ok {
		panic("obj is not *raptorApi.Feature")
	}
	wh.logger.Info("validate create", "name", f.Name)

	return wh.Validate(ctx, f)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (wh *webhook) ValidateUpdate(ctx context.Context, oldObject, newObj runtime.Object) error {
	f, ok := newObj.(*raptorApi.Feature)
	if !ok {
		return fmt.Errorf("new feature failed to cast")
	}
	old, ok := oldObject.(*raptorApi.Feature)
	if !ok {
		return fmt.Errorf("old feature failed to cast")
	}
	wh.logger.Info("validate update", "name", f.GetName())
	if !equality.Semantic.DeepEqual(old.Spec, f.Spec) && !wh.updatesAllowed {
		return fmt.Errorf("features are immutable in production")
	}

	return wh.Validate(ctx, f)
}

func (wh *webhook) Validate(ctx context.Context, f *raptorApi.Feature) error {
	dummyEngine := engine.Dummy{}

	if f.Spec.DataConnector != nil {
		if ar, ok := ctx.Value(admissionRequestContextKey).(admission.Request); ok && ar.DryRun == nil || ok && !*ar.DryRun {
			dc := raptorApi.DataConnector{}
			err := wh.client.Get(ctx, f.Spec.DataConnector.ObjectKey(), &dc)
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("data connector %s/%s not found: %w", f.Spec.DataConnector.Namespace, f.Spec.DataConnector.Name, err)
			}
			if err != nil {
				return fmt.Errorf("failed to get data connector: %w", err)
			}

			dci, err := api.DataConnectorFromManifest(ctx, &dc, wh.client)
			if err != nil {
				return fmt.Errorf("failed to get data connector instance: %w", err)
			}
			dummyEngine.DataConnector = dci
		}
	}
	_, err := engine.FeatureWithEngine(&dummyEngine, f)
	return err
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (wh *webhook) ValidateDelete(_ context.Context, obj runtime.Object) error {
	f, ok := obj.(*raptorApi.Feature)
	if !ok {
		panic("obj is not *raptorApi.Feature")
	}
	wh.logger.Info("validate delete", "name", f.GetName())

	// DISABLED. To enable deletion validation, change the above annotation's verbs to "verbs=create;update;delete"

	return nil
}
