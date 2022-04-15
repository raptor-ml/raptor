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

import (
	"context"
	"github.com/natun-ai/natun/pkg/api"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	natunApi "github.com/natun-ai/natun/pkg/api/v1alpha1"
)

// FeatureReconciler reconciles a Feature object
// This reconciler is used in every instance of the app, and not only the leader.
// It is used to ensure the EngineManager's state is synchronized with the CustomResources.
//
// For the creation and modification external resources, the operator's controller is used.
// For the operator's controller see the `internal/operator/feature_controller.go` file
type FeatureReconciler struct {
	client.Reader
	Scheme         *runtime.Scheme
	UpdatesAllowed bool
	EngineManager  api.Manager
}

//+kubebuilder:rbac:groups=k8s.natun.ai,resources=features,verbs=get;list;watch

// Reconcile is the main function of the reconciler.
func (r *FeatureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Feature definition from the Kubernetes API.
	feature := &natunApi.Feature{}
	err := r.Get(ctx, req.NamespacedName, feature)
	if err != nil {
		logger.Error(err, "Failed to get Feature")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if !feature.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		// Since this controller is used for the internal Core, we don't need to use finalizers

		if err := r.EngineManager.UnbindFeature(feature.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			return ctrl.Result{}, err
		}
	}

	if r.EngineManager.HasFeature(feature.FQN()) {
		if !r.UpdatesAllowed {
			logger.Info("Feature already exists. Ignoring since updates are not allowed")
			return ctrl.Result{}, nil
		}
		if err := r.EngineManager.UnbindFeature(feature.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			return ctrl.Result{}, err
		}
	}

	if err := r.EngineManager.BindFeature(feature); err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FeatureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return attachCoreConnector(r, &natunApi.Feature{}, r.UpdatesAllowed, mgr)
}
