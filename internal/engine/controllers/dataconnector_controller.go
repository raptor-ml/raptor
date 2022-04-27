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

// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors,verbs=get;list;watch

import (
	"context"
	"github.com/natun-ai/natun/api"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	natunApi "github.com/natun-ai/natun/api/v1alpha1"
)

// DataConnectorReconciler reconciles a DataConnector object
// This reconciler is used in every instance of the app, and not only the leader.
// It is used to ensure the EngineManager's state is synchronized with the CustomResources.
//
// For the creation and modification external resources, the operator's controller is used.
// For the operator's controller see the `internal/operator/dataconnector_controller.go` file
type DataConnectorReconciler struct {
	client.Reader
	Scheme        *runtime.Scheme
	EngineManager api.DataConnectorManager
}

// Reconcile is the main function of the reconciler.
func (r *DataConnectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the DataConnector definition from the Kubernetes API.
	dc := &natunApi.DataConnector{}
	err := r.Get(ctx, req.NamespacedName, dc)
	if err != nil {
		logger.Error(err, "Failed to get DataConnector")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if !dc.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is being deleted
		// Since this controller is used for the internal Core, we don't need to use finalizers

		if err := r.EngineManager.UnbindDataConnector(dc.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if r.EngineManager.HasDataConnector(dc.FQN()) {
		if err := r.EngineManager.UnbindDataConnector(dc.FQN()); err != nil {
			// if fail to delete, return with error, so that it can be retried
			return ctrl.Result{}, err
		}
	}

	dci, err := api.DataConnectorFromManifest(ctx, dc, r.Reader)
	if err != nil {
		logger.Error(err, "Failed to get DataConnector: %w", err)
		return ctrl.Result{}, err
	}

	if err := r.EngineManager.BindDataConnector(dci); err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *DataConnectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return attachCoreConnector(r, &natunApi.DataConnector{}, true, mgr)
}
