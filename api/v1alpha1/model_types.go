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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// ModelServer defines the backend inference server for the model.
// +kubebuilder:validation:Enum=sagemaker
type ModelServer string

// ModelSpec defines the list of feature FQNs that are enabled for a given feature set
type ModelSpec struct {
	// Freshness defines the age of a prediction-result(time since the value has set) to consider as *fresh*.
	// Fresh values doesn't require re-ingestion
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Freshness"
	Freshness metav1.Duration `json:"freshness"`

	// Staleness defines the age of a prediction-result(time since the value has set) to consider as *stale*.
	// Stale values are not fit for usage, therefore will not be returned and will REQUIRE re-ingestion.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Staleness"
	Staleness metav1.Duration `json:"staleness"`

	// Timeout defines the maximum ingestion time allowed to calculate the prediction.
	// +optional
	// +nullable
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Timeout"
	Timeout metav1.Duration `json:"timeout"`

	// Features is the list of feature FQNs that are enabled for a given feature set
	// +kubebuilder:validation:MinItems=2
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Features"
	Features []string `json:"features"`

	// KeyFeature is the feature FQN that is used to align the rest of the features with their timestamp.
	// If this is unset, the first feature in the list will be used.
	// +optional
	// +nullable
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Key Feature FQN"
	KeyFeature string `json:"keyFeature,omitempty"`

	// Labels is a list of feature FQNs that are used to label the prediction result.
	// +optional
	// +nullable
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Labels"
	Labels []string `json:"labels,omitempty"`

	// ModelFramework is the framework used to train the model.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Framework"
	ModelFramework string `json:"modelFramework"`

	// ModelFrameworkVersion is the version of the framework used to train the model.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Framework Version"
	ModelFrameworkVersion string `json:"modelFrameworkVersion"`

	// ModelServer is the server used to serve the model.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Model Server"
	ModelServer ModelServer `json:"modelServer"`

	// StorageURI is the URI of the model storage.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Storage URI"
	StorageURI string `json:"storageURI"`

	// TrainingCode defines the code used to train the model.
	// +optional
	// +nullable
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Python Expression"
	TrainingCode string `json:"trainingCode"`
}

// ModelStatus defines the observed state of Model
type ModelStatus struct {
	// FQN is the Fully Qualified Name for the Model
	// +operator-sdk:csv:customresourcedefinitions:type=status
	FQN string `json:"fqn"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=datascience,shortName=model
// +operator-sdk:csv:customresourcedefinitions:displayName="ML Model",resources={{Deployment,v1,raptor-controller-core}}

// Model is the Schema for the models API
type Model struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelSpec   `json:"spec,omitempty"`
	Status ModelStatus `json:"status,omitempty"`
}

// FQN returns the fully qualified name of the feature.
func (in *Model) FQN() string {
	ns := strings.Replace(in.GetNamespace(), "-", "_", -1)
	name := strings.Replace(in.GetName(), "-", "_", -1)
	return fmt.Sprintf("%s.%s", ns, name)
}

// +kubebuilder:object:root=true

// ModelList contains a list of Model
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Model{}, &ModelList{})
}
