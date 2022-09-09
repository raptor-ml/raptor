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

// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=features,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=features/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=features/finalizers,verbs=update

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	raptorApi "github.com/raptor-ml/raptor/api/v1alpha1"
)

// FeatureReconciler reconciles a Feature object
type FeatureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *FeatureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Feature definition from the Kubernetes API.
	feature := &raptorApi.Feature{}
	err := r.Get(ctx, req.NamespacedName, feature)
	if err != nil {
		logger.Error(err, "Failed to get Feature")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if feature.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(feature, finalizerName) {
			controllerutil.AddFinalizer(feature, finalizerName)
			if err := r.Update(ctx, feature); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(feature, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteFromConnector(ctx, feature); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(feature, finalizerName)
			if err := r.Update(ctx, feature); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if err := r.addToConnector(ctx, feature); err != nil {
		// If the error is "not found" then requeue this because maybe the user trying to add both the DataConnector
		// and the Feature on the same time
		if client.IgnoreNotFound(err) == nil {
			logger.Error(err, "Trying to add a Feature to a non-existing DataConnector")
		}
		return ctrl.Result{RequeueAfter: time.Millisecond * 100}, client.IgnoreNotFound(err)
	}

	feature.Status.FQN = feature.FQN()
	if err := r.Status().Update(ctx, feature); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *FeatureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&raptorApi.Feature{}).
		Complete(r)
}

func (r *FeatureReconciler) deleteFromConnector(ctx context.Context, feature *raptorApi.Feature) error {
	if feature.Spec.DataConnector == nil {
		return nil
	}

	logger := log.FromContext(ctx)

	// fix connector namespace
	if feature.Spec.DataConnector.Namespace == "" {
		feature.Spec.DataConnector.Namespace = feature.Namespace
	}

	conn := &raptorApi.DataConnector{}
	err := r.Get(ctx, feature.Spec.DataConnector.ObjectKey(), conn)
	if err != nil {
		logger.Error(err, "Failed to get associated DataConnector")
		// we'll ignore not-found errors, since they probably deleted and there's nothing we can do.
		return client.IgnoreNotFound(err)
	}

	if len(conn.Status.Features) == 0 {
		return nil
	}
	fts := make([]raptorApi.ResourceReference, 0, len(conn.Status.Features))
	for _, f := range conn.Status.Features {
		if f.Name != feature.Name {
			fts = append(fts, f)
		}
	}
	conn.Status.Features = fts
	return r.Status().Update(ctx, conn)
}

func (r *FeatureReconciler) addToConnector(ctx context.Context, feature *raptorApi.Feature) error {
	if feature.Spec.DataConnector == nil {
		return nil
	}

	logger := log.FromContext(ctx)

	// fix connector namespace
	if feature.Spec.DataConnector.Namespace == "" {
		feature.Spec.DataConnector.Namespace = feature.Namespace
	}

	conn := &raptorApi.DataConnector{}
	err := r.Get(ctx, feature.Spec.DataConnector.ObjectKey(), conn)
	if err != nil {
		logger.Error(err, "Failed to get associated DataConnector")
		return err
	}
	for _, f := range conn.Status.Features {
		if f.Name == feature.Name {
			return nil
		}
	}

	conn.Status.Features = append(conn.Status.Features, feature.ResourceReference())
	return r.Status().Update(ctx, conn)
}
