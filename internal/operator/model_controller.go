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

// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.raptor.ml,resources=models/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

import (
	"context"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

// ModelReconciler reconciles a Feature object
type ModelReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("component", "model-operator")

	// Fetch the Feature definition from the Kubernetes API.
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

	if p := plugins.ModelServer.Get(string(model.Spec.ModelServer)); p != nil {
		if changed, err := p.Reconcile(log.IntoContext(ctx, logger.WithName("runner")), r.reconcileRequest(model)); err != nil {
			r.EventRecorder.Eventf(model, "Warning", "ReconcileFailed",
				"Failed to reconcile Model: %v", err)
			return ctrl.Result{}, err
		} else if changed {
			r.EventRecorder.Event(model, "Normal", "ReconcileSuccess", "Model reconciled successfully")
			// Ask to requeue after 1 minute in order to give enough time for the
			// pods be created on the cluster side and the operand be able
			// to do the next update step accurately.
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	model.Status.FQN = model.FQN()
	if err := r.Status().Update(ctx, model); err != nil {
		logger.Error(err, "Failed to update Model status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ModelReconciler) reconcileRequest(model *manifests.Model) api.ModelReconcileRequest {
	return api.ModelReconcileRequest{
		Model:  model,
		Client: r.Client,
		Scheme: r.Scheme,
	}
}

// SetupWithManager sets up the controller with the Controller Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bldr := ctrl.NewControllerManagedBy(mgr).For(&manifests.Model{})

	for _, o := range plugins.ModelServer {
		for _, t := range o.Owns() {
			gvk := t.GetObjectKind().GroupVersionKind()
			if _, err := r.Client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version); err == nil {
				bldr = bldr.Owns(t)
			}
		}
	}

	return bldr.Complete(r)
}
