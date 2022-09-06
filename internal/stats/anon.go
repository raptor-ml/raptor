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

package stats

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	coreV1 "k8s.io/api/core/v1"
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
	namespace, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(namespace), nil
}

var anonID = ""

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get
func getAnonID(kc client.Client) string {
	if anonID != "" {
		return anonID
	}

	// Try to get the NS first (this is better to handle to prevent namespace restrictions)
	ns, err := getInClusterNamespace()
	if err != nil {
		ns = "kube-system"
	}

	if uid, err := fetchNSUID(kc, ns); err == nil {
		anonID = hash(uid)
	} else {
		anonID = uuid.NewString()
	}
	return anonID
}

func fetchNSUID(kc client.Client, name string) (string, error) {
	ns := coreV1.Namespace{}
	err := kc.Get(context.TODO(), client.ObjectKey{Name: name}, &ns)
	if err != nil {
		return "", err
	}
	return string(ns.UID), nil
}
