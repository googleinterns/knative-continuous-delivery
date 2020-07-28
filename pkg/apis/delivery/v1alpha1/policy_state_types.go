// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyState is
type PolicyState struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the PolicyState (from the client).
	// +optional
	Spec PolicyStateSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the PolicyState (from the controller).
	// +optional
	Status PolicyStateStatus `json:"status,omitempty"`
}

// Verify that PolicyState adheres to the appropriate interfaces.
var (
	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*PolicyState)(nil)
)

// PolicyStateSpec holds the desired state of the PolicyState (from the client).
type PolicyStateSpec struct {
	// TODO: implement policy state spec
}

// PolicyStateStatusFields holds the fields of PolicyState's status that
// are not generally shared.  This is defined separately and inlined so that
// other types can readily consume these fields via duck typing.
type PolicyStateStatusFields struct {
	// TODO: implement policy state status
}

// PolicyStateStatus communicates the observed state of the PolicyState (from the controller).
type PolicyStateStatus struct {
	duckv1.Status `json:",inline"`

	PolicyStateStatusFields `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyStateList is a list of PolicyState resources
type PolicyStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PolicyState `json:"items"`
}

// GetStatus retrieves the status of the PolicyState. Implements the KRShaped interface.
func (t *PolicyState) GetStatus() *duckv1.Status {
	return &t.Status.Status
}
