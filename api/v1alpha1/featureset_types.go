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
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FeatureSetSpec defines the list of feature FQNs that are enabled for a given feature set
type FeatureSetSpec struct {
	// Timeout defines the maximum ingestion time allowed to calculate the feature value.
	// +optional
	// +nullable
	Timeout metav1.Duration `json:"timeout"`

	// Features is the list of feature FQNs that are enabled for a given feature set
	// +kubebuilder:validation:MinItems=2
	// +kubebuilder:validation:Required
	Features []string `json:"features"`

	// KeyFeature is the feature FQN that is used to align the rest of the features with.
	// If this is unset, the first feature in the list will be used.
	// +optional
	KeyFeature string `json:"keyFeature,omitempty"`
}

// FeatureSetStatus defines the observed state of FeatureSet
type FeatureSetStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=datascience,shortName=ftset

// FeatureSet is the Schema for the featuresets API
type FeatureSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureSetSpec   `json:"spec,omitempty"`
	Status FeatureSetStatus `json:"status,omitempty"`
}

// FQN returns the fully qualified name of the feature.
func (in *FeatureSet) FQN() string {
	return fmt.Sprintf("%s.%s", in.GetName(), in.GetNamespace())
}

// +kubebuilder:object:root=true

// FeatureSetList contains a list of FeatureSet
type FeatureSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FeatureSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FeatureSet{}, &FeatureSetList{})
}
