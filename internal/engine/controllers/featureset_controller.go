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

package controllers

// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=featuresets,verbs=get;list;watch

import (
	"context"
	"encoding/json"
	"github.com/raptor-ml/natun/api"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	natunApi "github.com/raptor-ml/natun/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FeatureSetReconciler reconciles a Feature object
// This reconciler is used in every instance of the app, and not only the leader.
// It is used to ensure the EngineManager's state is synchronized with the CustomResources.
type FeatureSetReconciler struct {
	client.Reader
	Scheme        *runtime.Scheme
	EngineManager api.FeatureManager
}

// Reconcile is the main function of the reconciler.
func (r *FeatureSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the FeatureSet definition from the Kubernetes API.
	fs := &natunApi.FeatureSet{}
	err := r.Get(ctx, req.NamespacedName, fs)
	if err != nil {
		logger.Error(err, "Failed to get FeatureSet")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Convert the FeatureSet definition to a MetaData object.
	ft := &natunApi.Feature{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Feature",
			APIVersion: natunApi.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fs.Name,
			Namespace: fs.Namespace,
		},
		Spec: natunApi.FeatureSpec{
			Primitive: api.PrimitiveTypeHeadless.String(),
			Timeout:   fs.Spec.Timeout,
		},
	}
	ft.Spec.Builder.Kind = api.FeatureSetBuilder
	ft.Spec.Builder.Raw, err = json.Marshal(fs.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal features")
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if !fs.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		// Since this controller is used for the internal Core, we don't need to use finalizers

		if err := r.EngineManager.UnbindFeature(ft.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if r.EngineManager.HasFeature(ft.FQN()) {
		if err := r.EngineManager.UnbindFeature(ft.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			return ctrl.Result{}, err
		}
	}

	if err := r.EngineManager.BindFeature(ft); err != nil {
		logger.Error(err, "Failed to bind FeatureSet as feature")
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *FeatureSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return attachCoreConnector(r, &natunApi.FeatureSet{}, true, mgr)
}
