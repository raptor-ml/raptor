/*
Copyright 2022 Natun.

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

// +kubebuilder:webhook:path=/validate-k8s-natun-ai-v1alpha1-feature,mutating=false,failurePolicy=fail,sideEffects=None,groups=k8s.natun.ai,resources=features,verbs=create;update,versions=v1alpha1,name=vfeature.kb.io,admissionReviewVersions=v1

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/api"
	natunApi "github.com/natun-ai/natun/api/v1alpha1"
	"github.com/natun-ai/natun/internal/engine"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

type validator struct {
	client         client.Client
	logger         logr.Logger
	updatesAllowed bool
}

func generateValidatePath(gvk schema.GroupVersionKind) string {
	return "/validate-" + strings.ReplaceAll(gvk.Group, ".", "-") + "-" +
		gvk.Version + "-" + strings.ToLower(gvk.Kind)
}

type ctxKey string

const admissionRequestContextKey ctxKey = "AdmissionRequest"

func SetupFeatureWebhook(mgr ctrl.Manager, updatesAllowed bool) {
	wh := admission.WithCustomValidator(&natunApi.Feature{}, &validator{updatesAllowed: updatesAllowed})
	handler := wh.Handler
	wh.Handler = admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
		ctx = context.WithValue(ctx, admissionRequestContextKey, req.AdmissionRequest)
		return handler.Handle(ctx, req)
	})

	gvk := natunApi.GroupVersion.WithKind("Feature")
	mgr.GetWebhookServer().Register(generateValidatePath(gvk), wh)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	f := obj.(*natunApi.Feature)
	v.logger.Info("validate create", "name", f.Name)

	return v.Validate(ctx, f)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *validator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) error {
	f := newObj.(*natunApi.Feature)
	v.logger.Info("validate update", "name", f.GetName())
	if !v.updatesAllowed {
		return fmt.Errorf("features are immutable in production")
	}

	return v.Validate(ctx, f)
}

func (v *validator) Validate(ctx context.Context, f *natunApi.Feature) error {
	builder := f.Spec.Builder.Kind
	if builder == "" {
		builderType := &natunApi.FeatureBuilderKind{}
		err := json.Unmarshal(f.Spec.Builder.Raw, builderType)
		if err != nil {
			return fmt.Errorf("failed to unmarshal builder type: %w", err)
		}
		builder = builderType.Kind
	}
	if builder == "" {
		return fmt.Errorf("builder kind is empty")
	}

	dummyEngine := engine.Dummy{}

	if f.Spec.DataConnector != nil {
		if ar, ok := ctx.Value(admissionRequestContextKey).(admission.Request); ok && ar.DryRun == nil || ok && !*ar.DryRun {
			dc := natunApi.DataConnector{}
			err := v.client.Get(ctx, f.Spec.DataConnector.ObjectKey(), &dc)
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("data connector %s/%s not found", f.Spec.DataConnector.Namespace, f.Spec.DataConnector.Name)
			}
			if err != nil {
				return fmt.Errorf("failed to get data connector: %w", err)
			}

			dci, err := api.DataConnectorFromManifest(ctx, &dc, v.client)
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
func (v *validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	f := obj.(*natunApi.Feature)
	v.logger.Info("validate delete", "name", f.GetName())

	// DISABLED. To enable deletion validation, change the above annotation's verbs to "verbs=create;update;delete"

	return nil
}
