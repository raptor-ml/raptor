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

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/internal/plugin"
	"github.com/natun-ai/natun/pkg/api"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	natunApi "github.com/natun-ai/natun/pkg/api/v1alpha1"
)

// DataConnectorReconciler reconciles a DataConnector object
type DataConnectorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	CoreAddr string
}

//+kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DataConnectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Feature definition from the Kubernetes API.
	conn := &natunApi.DataConnector{}
	err := r.Get(ctx, req.NamespacedName, conn)
	if err != nil {
		logger.Error(err, "Failed to get DataConnector")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if conn.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(conn, finalizerName) {
			controllerutil.AddFinalizer(conn, finalizerName)
			if err := r.Update(ctx, conn); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(conn, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if len(conn.Status.Features) > 0 {
				// return with error so that it can be retried
				return ctrl.Result{}, fmt.Errorf("cannot delete DataConnector with associated Features")
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(conn, finalizerName)
			if err := r.Update(ctx, conn); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if p := plugin.DataConnectorReconciler.Get(conn.Spec.Kind); p != nil {
		if err := p(ctx, r.reconcileMetadata(), conn); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *DataConnectorReconciler) reconcileMetadata() api.ReconcileMetadata {
	return api.ReconcileMetadata{
		Client:      r.Client,
		Scheme:      r.Scheme,
		CoreAddress: r.CoreAddr,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataConnectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&natunApi.DataConnector{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
