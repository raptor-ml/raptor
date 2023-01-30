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

package controllers

// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=models,verbs=get;list;watch

import (
	"context"
	"encoding/json"
	"github.com/raptor-ml/raptor/api"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelReconciler reconciles a Feature object
// This reconciler is used in every instance of the app, and not only the leader.
// It is used to ensure the EngineManager's state is synchronized with the CustomResources.
type ModelReconciler struct {
	client.Reader
	Scheme        *runtime.Scheme
	EngineManager api.FeatureManager
}

// Reconcile is the main function of the reconciler.
func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("component", "model-controller")

	// Fetch the Model definition from the Kubernetes API.
	model := &manifests.Model{}
	err := r.Get(ctx, req.NamespacedName, model)
	if err != nil {
		logger.Error(err, "Failed to get Model")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger = logger.WithValues("model", model.FQN())

	// Convert the Model definition to a FeatureDescriptor object.
	ft := &manifests.Feature{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Feature",
			APIVersion: manifests.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.Name,
			Namespace: model.Namespace,
		},
		Spec: manifests.FeatureSpec{
			Primitive:    manifests.PrimitiveType(api.PrimitiveTypeFloat.String()),
			Freshness:    model.Spec.Freshness,
			Staleness:    model.Spec.Staleness,
			Timeout:      model.Spec.Timeout,
			KeepPrevious: nil,
			Keys:         model.Spec.Keys,
			DataSource:   nil,
			Builder: manifests.FeatureBuilder{
				Kind: api.ModelBuilder,
			},
		},
	}

	cfg, err := model.ParseInferenceConfig(ctx, r.Reader)
	if err != nil {
		logger.Error(err, "Failed to parse inference config")
		return ctrl.Result{}, err
	}

	md := &api.ModelDescriptor{
		Features:        model.Spec.Features,
		KeyFeature:      model.Spec.KeyFeature,
		Keys:            model.Spec.Keys,
		ModelFramework:  model.Spec.ModelFramework,
		ModelServer:     string(model.Spec.ModelServer),
		InferenceConfig: cfg,
	}
	ft.Spec.Builder.Raw, err = json.Marshal(md)
	if err != nil {
		logger.Error(err, "Failed to marshal features")
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if !model.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		// Since this controller is used for the internal Core, we don't need to use finalizers

		if err := r.EngineManager.UnbindFeature(ft.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			logger.Error(err, "Failed to unbind feature")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if r.EngineManager.HasFeature(ft.FQN()) {
		if err := r.EngineManager.UnbindFeature(ft.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			logger.Error(err, "Failed to unbind feature")
			return ctrl.Result{}, err
		}
	}

	if err := r.EngineManager.BindFeature(ft); err != nil {
		logger.Error(err, "Failed to bind Model as feature")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return attachCoreController(r, &manifests.Model{}, true, mgr)
}
