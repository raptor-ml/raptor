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

package runner

import (
	"context"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var distrolessNoRootUser int64 = 65532

type Base interface {
	Reconcile(ctx context.Context, rr api.ReconcileRequest, src *manifests.DataSource) error
}
type BaseRunner struct {
	Image           string
	Command         []string
	SecurityContext *corev1.SecurityContext
}

func (r BaseRunner) Reconciler() (api.DataSourceReconcile, error) {
	if r.Image == "" {
		return nil, fmt.Errorf("runner image is required")
	}
	if len(r.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	return r.reconcile, nil
}
func (r BaseRunner) reconcile(ctx context.Context, req api.ReconcileRequest) (bool, error) {
	logger := log.FromContext(ctx)

	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      deploymentName(req.DataSource),
		Namespace: req.DataSource.GetNamespace(),
	}}

	op, err := ctrl.CreateOrUpdate(ctx, req.Client, deploy, func() error {
		r.updateDeployment(deploy, req)
		return ctrl.SetControllerReference(req.DataSource, deploy, req.Scheme)
	})
	if err != nil {
		logger.Error(err, "Deployment reconcile failed")
		return false, err
	} else {
		// If you see many of these, make sure you don't have 2 leaders (i.e. local, and kind)
		logger.V(1).Info("Deployment successfully reconciled", "operation", op)
	}

	return op != controllerutil.OperationResultNone, nil
}

func (r BaseRunner) updateDeployment(deploy *appsv1.Deployment, req api.ReconcileRequest) {
	labels := map[string]string{
		"data-source-kind": req.DataSource.Spec.Kind,
		"data-source":      req.DataSource.GetName(),
	}
	deploy.ObjectMeta.Labels = labels

	if deploy.Spec.Replicas == nil {
		var replicas int32
		if req.DataSource.Spec.Replicas != nil {
			replicas = *req.DataSource.Spec.Replicas
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

	sidecars := req.RuntimeManager.GetSidecars()
	for i := range sidecars {
		found := false
		for n, env := range sidecars[i].Env {
			if env.Name == "CORE_GRPC_URL" {
				sidecars[i].Env[n].Value = req.CoreAddress
				found = true
			}
		}

		if !found {
			sidecars[i].Env = append(sidecars[i].Env, corev1.EnvVar{
				Name:  "CORE_GRPC_URL",
				Value: req.CoreAddress,
			})
		}
	}
	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "grpc-udp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	deploy.Spec.Template.Spec.Containers = append([]corev1.Container{
		containerWithDefaults(corev1.Container{
			Image: r.Image,
			Name:  "runner",
			Command: append(r.Command, []string{
				"--data-source-resource", req.DataSource.Name,
				"--data-source-namespace", req.DataSource.Namespace}...),
			Env: []corev1.EnvVar{
				{
					Name:  "DEFAULT_RUNTIME",
					Value: req.RuntimeManager.GetDefaultEnv(),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "grpc-udp",
					MountPath: "/tmp/raptor",
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: req.DataSource.Spec.Resources.Limits,
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser:    &distrolessNoRootUser,
				RunAsNonRoot: &t,
			},
		}),
	}, sidecars...)
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

func deploymentName(src *manifests.DataSource) string {
	return fmt.Sprintf("raptor-dsrc-%s", src.Name)
}
