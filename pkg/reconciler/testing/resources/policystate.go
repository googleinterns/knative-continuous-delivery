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

package resources

import (
	"time"

	psv1alpha1 "github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
)

// PolicyStateOption enables further configuration of a PolicyState.
type PolicyStateOption func(*psv1alpha1.PolicyState)

// PolicyState returns a new PolicyState
func PolicyState(namespace, name string, pso ...PolicyStateOption) *psv1alpha1.PolicyState {
	ps := &psv1alpha1.PolicyState{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec:   psv1alpha1.PolicyStateSpec{},
		Status: psv1alpha1.PolicyStateStatus{},
	}
	for _, opt := range pso {
		opt(ps)
	}
	return ps
}

// WithPSSpecTraffic sets the spec traffic of a PolicyState
func WithPSSpecTraffic(traffic ...v1.TrafficTarget) PolicyStateOption {
	return func(ps *psv1alpha1.PolicyState) {
		ps.Spec.Traffic = traffic
	}
}

// WithPSStatusTraffic sets the status traffic of a PolicyState
func WithPSStatusTraffic(traffic ...v1.TrafficTarget) PolicyStateOption {
	return func(ps *psv1alpha1.PolicyState) {
		ps.Status.Traffic = traffic
	}
}

// WithNextUpdateTimestamp sets the Status.NextUpdateTimestamp of a PolicyState
func WithNextUpdateTimestamp(t time.Time) PolicyStateOption {
	return func(ps *psv1alpha1.PolicyState) {
		ps.Status.NextUpdateTimestamp = &metav1.Time{t}
	}
}
