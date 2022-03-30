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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DataConnectorReference represents a Secret Reference. It has enough information to retrieve secret
// in any namespace
// +structType=atomic
type DataConnectorReference struct {
	// Name is unique within a namespace to reference a DataConnector resource.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// Namespace defines the space within which the secret name must be unique.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
}

// DataConnectorSpec defines the desired state of DataConnector
type DataConnectorSpec struct {
	// Resources defines the required resources for a single container(underlying implementation) of this DataConnector.
	// Notice that this is not applicable for every DataConnector, but only for those who implement an External Runner.
	//
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
}

// DataConnectorStatus defines the observed state of DataConnector
type DataConnectorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +k8s:openapi-gen=true
// +genclient
// +genclient:noStatus
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:resource:categories=datasciense,shortName=dconn

// DataConnector is the Schema for the dataconnectors API
type DataConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataConnectorSpec   `json:"spec,omitempty"`
	Status DataConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataConnectorList contains a list of DataConnector
type DataConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataConnector{}, &DataConnectorList{})
}
