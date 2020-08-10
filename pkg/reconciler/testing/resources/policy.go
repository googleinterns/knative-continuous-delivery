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
	"github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyOption enables further configuration of a Policy.
type PolicyOption func(*v1alpha1.Policy)

// MakePolicy returns a new Policy
// it's not named Policy in order to avoid "redeclared during import" errors due to name conflict
func MakePolicy(namespace, name string, po ...PolicyOption) *v1alpha1.Policy {
	p := &v1alpha1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec:   v1alpha1.PolicySpec{},
		Status: v1alpha1.PolicyStatus{},
	}
	for _, opt := range po {
		opt(p)
	}
	return p
}

// WithMode sets the Spec.Mode of a Policy
func WithMode(mode string) PolicyOption {
	return func(p *v1alpha1.Policy) {
		p.Spec.Mode = mode
	}
}

// WithDefaultThreshold sets the Spec.DefaultThreshold of a Policy
func WithDefaultThreshold(defaultThreshold int) PolicyOption {
	return func(p *v1alpha1.Policy) {
		p.Spec.DefaultThreshold = defaultThreshold
	}
}

// WithStages sets the Spec.Stages of a Policy
func WithStages(stages ...v1alpha1.Stage) PolicyOption {
	return func(p *v1alpha1.Policy) {
		p.Spec.Stages = stages
	}
}
