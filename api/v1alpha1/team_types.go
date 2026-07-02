/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TeamSpec defines the desired state of Team
type TeamSpec struct {
	// namespace is the Kubernetes namespace to be created/managed for this team.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace"`

	// description is a human-readable description of the team.
	// +optional
	Description string `json:"description,omitempty"`

	// owner identifies the team owner (e.g. email or username).
	// +required
	Owner string `json:"owner"`

	// labels are additional labels applied to created resources
	// (namespace, ServiceAccount, RBAC objects).
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// Condition types used in Team status.
const (
	ConditionReady                 = "Ready"
	ConditionNamespaceCreated      = "NamespaceCreated"
	ConditionServiceAccountCreated = "ServiceAccountCreated"
	ConditionRBACCreated           = "RBACCreated"
	ConditionIAMRoleCreated        = "IAMRoleCreated"
)

// TeamStatus defines the observed state of Team.
type TeamStatus struct {
	// conditions represent the current state of the Team resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the last generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.spec.namespace`
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=`.spec.owner`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Team is the Schema for the teams API
type Team struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Team
	// +required
	Spec TeamSpec `json:"spec"`

	// status defines the observed state of Team
	// +optional
	Status TeamStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TeamList contains a list of Team
type TeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Team `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Team{}, &TeamList{})
		return nil
	})
}
