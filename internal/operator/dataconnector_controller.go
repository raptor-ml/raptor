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

// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=datasources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=datasources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=datasources/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

import (
	"context"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	raptorApi "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DataSourceReconciler reconciles a DataSource object
type DataSourceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	CoreAddr string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DataSourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Feature definition from the Kubernetes API.
	src := &raptorApi.DataSource{}
	err := r.Get(ctx, req.NamespacedName, src)
	if err != nil {
		logger.Error(err, "Failed to get DataSource")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if src.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(src, finalizerName) {
			controllerutil.AddFinalizer(src, finalizerName)
			if err := r.Update(ctx, src); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(src, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if len(src.Status.Features) > 0 {
				// return with error so that it can be retried
				return ctrl.Result{}, fmt.Errorf("cannot delete DataSource with associated Features")
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(src, finalizerName)
			if err := r.Update(ctx, src); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if p := plugins.DataSourceReconciler.Get(src.Spec.Kind); p != nil {
		if changed, err := p(ctx, r.reconcileRequest(src)); err != nil {
			return ctrl.Result{}, err
		} else if changed {
			// Ask to requeue after 1 minute in order to give enough time for the
			// pods be created on the cluster side and the operand be able
			// to do the next update step accurately.
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	// Todo change status to ready

	return ctrl.Result{}, nil
}

func (r *DataSourceReconciler) reconcileRequest(src *raptorApi.DataSource) api.ReconcileRequest {
	return api.ReconcileRequest{
		DataSource:  src,
		Client:      r.Client,
		Scheme:      r.Scheme,
		CoreAddress: r.CoreAddr,
	}
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *DataSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&raptorApi.DataSource{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
