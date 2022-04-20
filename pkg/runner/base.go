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
	"github.com/natun-ai/natun/pkg/api"
	natunApi "github.com/natun-ai/natun/pkg/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const runtimeImg = "ghcr.io/natun-ai/natun-runtime"

type Base interface {
	Reconcile(ctx context.Context, md api.ReconcileMetadata, conn *natunApi.DataConnector) error
}
type runner struct {
	RunnerImage    string
	RuntimeVersion string
	Command        []string
}

func New(runnerImage string, runtimeVersion string, command []string) (Base, error) {
	if runnerImage == "" {
		return nil, fmt.Errorf("runner image is required")
	}
	if runtimeVersion == "" {
		return nil, fmt.Errorf("runtime version is required")
	}
	if len(command) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	if runtimeVersion == "master" {
		runtimeVersion = "latest"
	}
	return &runner{
		RunnerImage:    runnerImage,
		RuntimeVersion: runtimeVersion,
		Command:        command,
	}, nil
}

func (r runner) Reconcile(ctx context.Context, md api.ReconcileMetadata, conn *natunApi.DataConnector) error {
	logger := log.FromContext(ctx)

	objectKey := types.NamespacedName{
		Name:      deploymentName(conn),
		Namespace: conn.Namespace,
	}

	// Check if the deployment already exists, if not create a new one
	dep := &appsv1.Deployment{}
	err := md.Client.Get(ctx, objectKey, dep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForConn(conn, md.CoreAddress)
		err := ctrl.SetControllerReference(conn, dep, md.Scheme)
		if err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		logger.Info("Creating a new Deployment", "objectKey", objectKey)
		err = md.Client.Create(ctx, dep)
		if err != nil {
			logger.Error(err, "Failed to create new Deployment", "objectKey", objectKey)
			return err
		}

		conn.Status.Deployments = append(conn.Status.Deployments, natunApi.ResourceReference{
			Name:      dep.Name,
			Namespace: dep.Namespace,
		})
		if err != nil {
			logger.Error(err, "Failed to update the deployment to the DataConnector status", "objectKey", objectKey)
			return err
		}

		// Deployment created successfully - return and requeue
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get Deployment", "objectKey", objectKey)
		return err
	}

	return nil
}

func (r runner) deploymentForConn(conn *natunApi.DataConnector, coreAddr string) *appsv1.Deployment {
	labels := map[string]string{"dataconnector-kind": conn.Kind, "dataconnector": conn.GetName()}

	resources := corev1.ResourceRequirements{
		Limits:   conn.Spec.Resources.Limits,
		Requests: conn.Spec.Resources.Requests,
	}
	var replicas int32
	if conn.Spec.Resources.Replicas != nil {
		replicas = *conn.Spec.Resources.Replicas
	} else {
		replicas = 1
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(conn),
			Namespace: conn.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container": "runner",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: runtimeImg,
							Name:  "runner",
							Command: append(r.Command, []string{
								"--dataconnector-resource", conn.Name,
								"--dataconnector-namespace", conn.Namespace,
								"--runtime-grpc-addr", ":60005"}...),
							Resources: resources,
						},
						{
							Image: fmt.Sprintf("%s:%s", runtimeImg, r.RuntimeVersion),
							Name:  "runtime",
							Command: []string{
								"runtime",
								"--core-grpc-url", coreAddr,
								"--grpc-addr", ":60005",
							},
							Resources: resources,
						},
					},
				},
			},
		},
	}
	return dep
}

func deploymentName(conn *natunApi.DataConnector) string {
	return fmt.Sprintf("natun-conn-%s", conn.Name)
}
