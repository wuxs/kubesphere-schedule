/*
Copyright 2022.

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
	cranev1 "github.com/gocrane/api/analysis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceSelector = cranev1.ResourceSelector
type CompletionStrategy = cranev1.CompletionStrategy

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AnalysisSpec defines the desired state of Analysis
type AnalysisSpec struct {
	// ResourceSelector indicates how to select resources(e.g. a set of Deployments) for an Analytics.
	// +required
	// +kubebuilder:validation:Required
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// CompletionStrategy indicate how to complete an Analytics.
	// +optional
	CompletionStrategy CompletionStrategy `json:"completionStrategy"`
}

// AnalysisStatus defines the observed state of Analysis
type AnalysisStatus struct {
	// LastUpdateTime is the last time the status updated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Conditions is an array of current analytics conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Analysis is the Schema for the analyses API
type Analysis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalysisSpec   `json:"spec,omitempty"`
	Status AnalysisStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AnalysisList contains a list of Analysis
type AnalysisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Analysis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Analysis{}, &AnalysisList{})
}
