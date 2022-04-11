package streaming

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/internal/plugin"
	"github.com/natun-ai/natun/pkg/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const streamingImage = "natun/streaming:latest"
const runtimeSidecarImage = "natun/runtime-sidecar:latest"

func init() {
	// Register the plugin
	plugin.DataConnectorReconciler.Register("streaming", Reconcile)
}

func Reconcile(ctx context.Context, client client.Client, scheme *runtime.Scheme, coreAddr string, conn *v1alpha1.DataConnector) error {
	logger := log.FromContext(ctx)

	objectKey := types.NamespacedName{
		Name:      deploymentName(conn),
		Namespace: conn.Namespace,
	}

	// Check if the deployment already exists, if not create a new one
	dep := &appsv1.Deployment{}
	err := client.Get(ctx, objectKey, dep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := deploymentForConn(conn, coreAddr)
		err := ctrl.SetControllerReference(conn, dep, scheme)
		if err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		logger.Info("Creating a new Deployment", "objectKey", objectKey)
		err = client.Create(ctx, dep)
		if err != nil {
			logger.Error(err, "Failed to create new Deployment", "objectKey", objectKey)
			return err
		}

		// Deployment created successfully - return and requeue
		return nil
	}

	logger.Error(err, "Failed to get Deployment")
	return err
}

func deploymentForConn(conn *v1alpha1.DataConnector, coreAddr string) *appsv1.Deployment {
	var replicas int32 = 3
	labels := map[string]string{"app": "streaming", "dataconnector": conn.GetName()}

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
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:     streamingImage,
							Name:      "streaming",
							Command:   []string{"streaming"},
							Resources: conn.Spec.Resources,
						},
						{
							Image:     runtimeSidecarImage,
							Name:      "runtime",
							Command:   []string{"runtime", "--core-grpc-url", coreAddr},
							Resources: conn.Spec.Resources,
						},
					},
				},
			},
		},
	}
	return dep
}

func deploymentName(conn *v1alpha1.DataConnector) string {
	return fmt.Sprintf("natun-conn-%s", conn.Name)
}
