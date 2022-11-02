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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	ResourceTypeDeployment string = "Deployment"
	ResourceTypeNamespace  string = "Namespace"
)

// ResourceSelector describes how the resources will be selected.
type ResourceSelector struct {
	// Kind of the resource, e.g. Deployment
	Kind string `json:"kind"`

	// API version of the resource, e.g. "apps/v1"
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Name of the resource.
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// AnalysisSpec defines the desired state of Analysis
type AnalysisTaskSpec struct {
	// Kind of the resource, e.g. Deployment
	Type string `json:"type,omitempty"`

	// Target what the analysis is for.
	// +optional
	ResourceSelectors []ResourceSelector `json:"resourceSelectors,omitempty"`

	// CompletionStrategy indicate how to complete an Analytics.
	// +optional
	CompletionStrategy cranev1.CompletionStrategy `json:"completionStrategy"`
}

// AnalysisStatus defines the observed state of Analysis
type AnalysisTaskStatus struct {
	// LastUpdateTime is the last time the status updated.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Conditions is an array of current analytics conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Result
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=analysis
// +kubebuilder:subresource:status

// AnalysisTask is the Schema for the schedule API
type AnalysisTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalysisTaskSpec   `json:"spec,omitempty"`
	Status AnalysisTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Result

// AnalysisList contains a list of Analysis
type AnalysisTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AnalysisTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AnalysisTask{}, &AnalysisTaskList{})
}
