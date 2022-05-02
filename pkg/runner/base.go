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

package runner

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/api"
	natunApi "github.com/natun-ai/natun/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const runtimeImg = "ghcr.io/natun-ai/natun-runtime"

var distrolessNoRootUser int64 = 65532

type Base interface {
	Reconcile(ctx context.Context, md api.ReconcileRequest, conn *natunApi.DataConnector) error
}
type BaseRunner struct {
	Image           string
	RuntimeVersion  string
	Command         []string
	SecurityContext *corev1.SecurityContext
}

func (r BaseRunner) Reconciler() (api.DataConnectorReconcile, error) {
	if r.Image == "" {
		return nil, fmt.Errorf("runner image is required")
	}
	if r.RuntimeVersion == "" {
		return nil, fmt.Errorf("runtime version is required")
	}
	if len(r.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	// defaults
	if r.RuntimeVersion == "master" {
		r.RuntimeVersion = "latest"
	}
	return r.reconcile, nil
}
func (r BaseRunner) reconcile(ctx context.Context, req api.ReconcileRequest) (bool, error) {
	logger := log.FromContext(ctx)

	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      deploymentName(req.DataConnector),
		Namespace: req.DataConnector.GetNamespace(),
	}}

	op, err := ctrl.CreateOrUpdate(ctx, req.Client, deploy, func() error {
		r.updateDeployment(deploy, req)
		return ctrl.SetControllerReference(req.DataConnector, deploy, req.Scheme)
	})
	if err != nil {
		logger.Error(err, "Deployment reconcile failed")
		return false, err
	} else {
		logger.V(1).Info("Deployment successfully reconciled", "operation", op)
	}

	return op != controllerutil.OperationResultNone, nil
}

func (r BaseRunner) updateDeployment(deploy *appsv1.Deployment, req api.ReconcileRequest) {
	labels := map[string]string{
		"dataconnector-kind": req.DataConnector.Spec.Kind,
		"dataconnector":      req.DataConnector.GetName(),
	}
	deploy.ObjectMeta.Labels = labels

	if deploy.Spec.Replicas == nil {
		var replicas int32
		if req.DataConnector.Spec.Replicas != nil {
			replicas = *req.DataConnector.Spec.Replicas
		} else {
			replicas = 1
		}
		deploy.Spec.Replicas = &replicas
	}

	// Deployment selector is immutable, so we set this value only if
	// a new object is going to be created
	if deploy.ObjectMeta.CreationTimestamp.IsZero() {
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
	}

	deploy.Spec.Template.ObjectMeta.Labels = labels
	deploy.Spec.Template.ObjectMeta.Annotations = map[string]string{
		"kubectl.kubernetes.io/default-container": "runner",
	}

	t := true

	deploy.Spec.Template.Spec.Containers = []corev1.Container{
		containerWithDefaults(corev1.Container{
			Image: r.Image,
			Name:  "runner",
			Command: append(r.Command, []string{
				"--dataconnector-resource", req.DataConnector.Name,
				"--dataconnector-namespace", req.DataConnector.Namespace,
				"--runtime-grpc-addr", ":60005"}...),
			Resources: corev1.ResourceRequirements{
				Limits: req.DataConnector.Spec.Resources.Limits,
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser:    &distrolessNoRootUser,
				RunAsNonRoot: &t,
			},
		}),
		containerWithDefaults(corev1.Container{
			Image: fmt.Sprintf("%s:%s", runtimeImg, r.RuntimeVersion),
			Name:  "runtime",
			Command: []string{
				"./runtime",
				"--core-grpc-url", req.CoreAddress,
				"--grpc-addr", ":60005",
			},
			Resources:       req.DataConnector.Spec.Resources,
			SecurityContext: r.SecurityContext,
		}),
	}
}

func containerWithDefaults(container corev1.Container) corev1.Container {
	if container.TerminationMessagePath == "" {
		container.TerminationMessagePath = corev1.TerminationMessagePathDefault
	}
	if container.TerminationMessagePolicy == "" {
		container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	}
	if container.ImagePullPolicy == "" {
		container.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if container.SecurityContext == nil {
		t := true
		container.SecurityContext = &corev1.SecurityContext{
			RunAsNonRoot: &t,
		}
	}
	return container
}

func deploymentName(conn *natunApi.DataConnector) string {
	return fmt.Sprintf("natun-conn-%s", conn.Name)
}
