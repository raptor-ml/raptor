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

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Important: Run "make" to regenerate code after modifying this file

// DataConnectorSpec defines the desired state of DataConnector
type DataConnectorSpec struct {
	// Kind of the DataConnector
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Connector Kind"
	Kind string `json:"kind"`

	// Config of the DataConnector
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config"
	Config []ConfigVar `json:"config"`

	// Resources defines the required resources for a single container(underlying implementation) of this DataConnector.
	// Notice that this is not applicable for every DataConnector, but only for those who implement an External Runner.
	//
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resources",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Replicas defines the number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Replicas",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas *int32 `json:"replicas,omitempty"`
}

// ConfigVar is a name/value pair for the config.
type ConfigVar struct {
	// Configuration name
	Name string `json:"name"`
	// Configuration value
	// +optional
	Value string `json:"value,omitempty"`
	// Configuration value from secret
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Secret"}
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// ResourceReference represents a resource reference. It has enough information to retrieve resource in any namespace.
// +structType=atomic
type ResourceReference struct {
	// Name is unique within a namespace to reference a resource.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource's Name"
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// Namespace defines the space within which the resource name must be unique.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource's Namespace"
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
}

// ObjectKey is a helper function to get a client.ObjectKey from an ObjectReference
func (in *ResourceReference) ObjectKey() client.ObjectKey {
	return client.ObjectKey{
		Name:      in.Name,
		Namespace: in.Namespace,
	}
}

func (in *ResourceReference) FQN() string {
	return fmt.Sprintf("%s.%s", in.Name, in.Namespace)
}

// DataConnectorStatus defines the observed state of DataConnector
type DataConnectorStatus struct {
	// Features includes a list of references for the Feature that uses this DataConnector
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Features []ResourceReference `json:"features"`

	// +operator-sdk:csv:customresourcedefinitions:type=status
	Replicas int32 `json:"replicas,omitempty"`
}

// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:resource:categories=datascience,shortName=conn
// +operator-sdk:csv:customresourcedefinitions:displayName="DataConnector",resources={{Deployment,v1,raptor-conn-<name>}}

// DataConnector is the Schema for the dataconnectors API
type DataConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataConnectorSpec   `json:"spec,omitempty"`
	Status DataConnectorStatus `json:"status,omitempty"`
}

// FQN returns the fully qualified name of the feature.
func (in *DataConnector) FQN() string {
	return fmt.Sprintf("%s.%s", in.GetName(), in.GetNamespace())
}

func (in *DataConnector) ResourceReference() ResourceReference {
	return ResourceReference{
		Namespace: in.GetNamespace(),
		Name:      in.GetName(),
	}
}

//+kubebuilder:object:root=true

// DataConnectorList contains a list of DataConnector
type DataConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(new(DataConnector), new(DataConnectorList))
}
