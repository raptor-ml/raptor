package main

import (
	"context"
	"fmt"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func getInClusterNamespace() (string, error) {
	// Check whether the namespace file exists.
	// If not, we are not running in cluster so can't guess the namespace.
	if _, err := os.Stat(inClusterNamespacePath); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, please specify accessor-service")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}

	// Load the namespace file and return its content
	namespace, err := ioutil.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

func detectAccessor(c client.Client) (string, error) {
	ns, err := getInClusterNamespace()
	if err != nil {
		return "", err
	}

	var svc *corev1.Service
	if err := c.Get(context.TODO(), client.ObjectKey{Name: "core", Namespace: ns}, svc); err != nil {
		return "", fmt.Errorf("error getting accessor service: %w", err)
	}

	port := 70000
	for _, p := range svc.Spec.Ports {
		if p.Name == "grpc" {
			port = int(p.Port)
			break
		}
	}
	return fmt.Sprintf("%s.%s:%d", svc.GetName(), svc.GetName(), port), nil
}
