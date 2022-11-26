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
// +kubebuilder:webhook:path=/mutate-k8s-raptor-ml-v1alpha1-feature,mutating=true,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=k8s.raptor.ml,resources=features,verbs=create;update,versions=v1alpha1,name=mutate-feature.k8s.raptor.ml,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-k8s-raptor-ml-v1alpha1-feature,mutating=false,failurePolicy=fail,sideEffects=NoneOnDryRun,groups=k8s.raptor.ml,resources=features,verbs=create;update,versions=v1alpha1,name=validate-feature.k8s.raptor.ml,admissionReviewVersions=v1

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/internal/engine"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const FeatureWebhookValidatePath = "/validate-k8s-raptor-ml-v1alpha1-feature"
const FeatureWebhookValidateName = "raptor-validating-webhook-configuration"
const FeatureWebhookMutatePath = "/mutate-k8s-raptor-ml-v1alpha1-feature"
const FeatureWebhookMutateName = "raptor-mutating-webhook-configuration"

func SetupFeatureWebhook(mgr ctrl.Manager, updatesAllowed bool, rm api.RuntimeManager) {
	impl := &webhook{
		updatesAllowed: updatesAllowed,
		client:         mgr.GetClient(),
		logger:         mgr.GetLogger().WithName("feature-webhook"),
		runtimeManager: rm,
	}
	wh := admission.WithCustomValidator(&manifests.Feature{}, impl)
	wh.Handler = &admissionWrapper{Handler: wh.Handler}

	mgr.GetWebhookServer().Register(FeatureWebhookValidatePath, wh)

	wh = admission.WithCustomDefaulter(&manifests.Feature{}, impl)
	wh.Handler = &admissionWrapper{Handler: wh.Handler}
	mgr.GetWebhookServer().Register(FeatureWebhookMutatePath, wh)
}

type webhook struct {
	client         client.Client
	logger         logr.Logger
	updatesAllowed bool
	runtimeManager api.RuntimeManager
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
	f := obj.(*manifests.Feature)
	wh.logger.Info("defaulting", "name", f.GetName())

	if f.Spec.DataSource != nil && f.Spec.DataSource.Namespace == "" {
		f.Spec.DataSource.Namespace = f.GetNamespace()
	}
	if f.Spec.Builder.Kind == "" {
		if f.Spec.DataSource != nil {
			if ar, ok := ctx.Value(admissionRequestContextKey).(admission.Request); ok && ar.DryRun == nil || ok && !*ar.DryRun {
				src := manifests.DataSource{}
				err := wh.client.Get(ctx, f.Spec.DataSource.ObjectKey(), &src)
				if apierrors.IsNotFound(err) {
					return fmt.Errorf("DataSource %s/%s not found", f.Spec.DataSource.Namespace, f.Spec.DataSource.Name)
				}
				if err != nil {
					return fmt.Errorf("failed to get DataSource: %w", err)
				}

				srci, err := api.DataSourceFromManifest(ctx, &src, wh.client)
				if err != nil {
					return fmt.Errorf("failed to get DataSource instance: %w", err)
				}

				// Check if DataSource is mapped to a special builder
				if plugins.FeatureAppliers[srci.Kind] != nil {
					f.Spec.Builder.Kind = srci.Kind
				}
			}
		}
		if f.Spec.Builder.Kind == "" {
			f.Spec.Builder.Kind = api.HeadlessBuilder
		}

		if f.Spec.Builder.AggrGranularity.Milliseconds() > 0 && len(f.Spec.Builder.Aggr) > 0 {
			f.Spec.Freshness = f.Spec.Builder.AggrGranularity
		}
	}

	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (wh *webhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	f := obj.(*manifests.Feature)
	wh.logger.Info("validate create", "name", f.Name)

	return wh.Validate(ctx, f)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (wh *webhook) ValidateUpdate(ctx context.Context, oldObject, newObj runtime.Object) error {
	f := newObj.(*manifests.Feature)
	old := oldObject.(*manifests.Feature)
	wh.logger.Info("validate update", "name", f.GetName())
	if !equality.Semantic.DeepEqual(old.Spec, f.Spec) && !wh.updatesAllowed {
		return fmt.Errorf("features are immutable in production")
	}

	return wh.Validate(ctx, f)
}

func (wh *webhook) Validate(ctx context.Context, f *manifests.Feature) error {
	dummyEngine := engine.Dummy{RuntimeManager: wh.runtimeManager}

	if f.Spec.DataSource != nil {
		if ar, ok := ctx.Value(admissionRequestContextKey).(admission.Request); ok && ar.DryRun == nil || ok && !*ar.DryRun {
			src := manifests.DataSource{}
			err := wh.client.Get(ctx, f.Spec.DataSource.ObjectKey(), &src)
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("DataSource %s/%s not found", f.Spec.DataSource.Namespace, f.Spec.DataSource.Name)
			}
			if err != nil {
				return fmt.Errorf("failed to get DataSource: %w", err)
			}

			dci, err := api.DataSourceFromManifest(ctx, &src, wh.client)
			if err != nil {
				return fmt.Errorf("failed to get DataSource instance: %w", err)
			}
			dummyEngine.DataSource = dci
		}
	}
	_, err := engine.FeatureWithEngine(&dummyEngine, f)
	return err
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (wh *webhook) ValidateDelete(_ context.Context, obj runtime.Object) error {
	f := obj.(*manifests.Feature)
	wh.logger.Info("validate delete", "name", f.GetName())

	// DISABLED. To enable deletion validation, change the above annotation's verbs to "verbs=create;update;delete"

	return nil
}
