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

	"knative.dev/pkg/apis"
	"knative.dev/serving/pkg/apis/serving/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyState is used by KCD controller to communicate routing information to the
// mutating webhook in order to sideline the Service reconciler
type PolicyState struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds info about what the routing state SHOULD be
	// +optional
	Spec PolicyStateSpec `json:"spec,omitempty"`

	// Status holds info about what routing state has been written by the webhook
	// +optional
	Status PolicyStateStatus `json:"status,omitempty"`
}

// Verify that PolicyState adheres to the appropriate interfaces.
var (
	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*PolicyState)(nil)
)

const (
	// PolicyStateConditionRouteConfigured is set to false if any failure prevents
	// PolicyState.Spec from being written to Route.Spec
	PolicyStateConditionRouteConfigured apis.ConditionType = "RouteConfigured"
)

// PolicyStateSpec holds the desired routing spec computed by reconciler
// Should be set by reconciler, and set by webhook to write Route appropriately
type PolicyStateSpec struct {
	// Traffic specifies how to distribute traffic over a collection of
	// revisions and configurations.
	Traffic []v1.TrafficTarget `json:"traffic,omitempty"`
}

// PolicyStateStatusFields holds the fields of PolicyState's status that
// are not generally shared.  This is defined separately and inlined so that
// other types can readily consume these fields via duck typing.
type PolicyStateStatusFields struct {
	// NextUpdateTimestamp specifies the next time when this PolicyState spec should be updated
	// it is used in conjunction with EnqueueAfter to help reconciler enforce time-based policies
	// it also helps prevent unexpected rollout behavior when controller restarts, etc.
	// optional because when a rollout is completed there is no more future updates to be done
	NextUpdateTimestamp *metav1.Time `json:"nextUpdateTimestamp,omitempty"`

	// Traffic describes the current routing spec that the webhook has enforced
	// If this doesn't agree with Spec.Traffic, then the webhook SetDefaults must set them to agree with each other
	Traffic []v1.TrafficTarget `json:"traffic,omitempty"`
}

// PolicyStateStatus communicates the observed state of the PolicyState
// Should be set by the webhook
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
