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
	"knative.dev/pkg/apis"

	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Policy is used to specify traffic behavior during progressive rollout
// reconciler will use Policy to compute the routing states
type Policy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds info about the desired traffic behavior
	// +optional
	Spec PolicySpec `json:"spec,omitempty"`

	// Status holds info about the current traffic behavior
	// +optional
	Status PolicyStatus `json:"status,omitempty"`
}

// Verify that Policy adheres to the appropriate interfaces.
var (
	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*Policy)(nil)

	// Check that Policy may be validated and defaulted.
	_ apis.Validatable = (*Policy)(nil)
	_ apis.Defaultable = (*Policy)(nil)
)

// PolicySpec holds info about the desired traffic behavior
// These fields are exactly the same as those defined in pkg/reconciler/delivery/policy.go
type PolicySpec struct {
	// Mode specifies the metric that the policy is based on
	// Possible values are: "time", "request", "error"
	Mode string `json:"mode"`

	// DefaultThreshold is the threshold value that is used when a rollout stage doesn't specify
	// a threshold of its own; this can be useful when the threshold is a constant value across
	// all rollout stages, in which case there is no need to copy paste the same value in all entries
	// The interpretation of DefaultThreshold depends on the value of Mode
	DefaultThreshold int `json:"defaultThreshold"`

	// Stages specifies the traffic percentages that the NEW Revision is expected to have
	// at successive rollout stages; the list of integers must start at 0
	// all entries must be in the range [0, 100), and must be sorted in increasing order
	// Technically the final rollout percentage is 100, but this is implicitly understood,
	// and should NOT be explicitly specified in Stages
	// In addition to the traffic percentages, each stage can OPTIONALLY specify its own threshold
	// this gives greater flexibility to policy design
	// The threshold value for stage N is the value that must be achieved BEFORE moving to stage N+1
	Stages []Stage `json:"stages,omitempty"`
}

// Stage specifies a single rollout stage
type Stage struct {
	// Percent is the percentage of traffic that should go to the new Revision at this stage
	Percent int `json:"percent"`

	// Threshold tells the condition for progressing to the next rollout stage
	// This field is optional; if not specified, then the threshold value defaults to PolicySpec.DefaultThreshold
	Threshold *int `json:"threshold,omitempty"`
}

// PolicyStatusFields is the fields in PolicyStatus
// This is empty for now because nothing is needed here
type PolicyStatusFields struct{}

// PolicyStatus holds info about the current traffic behavior
type PolicyStatus struct {
	duckv1.Status `json:",inline"`

	PolicyStatusFields `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyList is a list of Policy resources
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Policy `json:"items"`
}

// GetStatus retrieves the status of the Policy. Implements the KRShaped interface.
func (t *Policy) GetStatus() *duckv1.Status {
	return &t.Status.Status
}
