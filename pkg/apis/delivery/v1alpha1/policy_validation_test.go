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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// knative.dev/pkg/ptr library doesn't have Int, so we need to implement it here
func intptr(x int) *int {
	return &x
}

func TestPolicyValidation(t *testing.T) {
	tests := []struct {
		name string
		p    *Policy
		want *apis.FieldError
	}{{
		name: "policy is ok",
		p: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
			},
			Spec: PolicySpec{
				Mode:             "time",
				DefaultThreshold: 100,
				Stages:           []Stage{{0, nil}},
			},
		},
		want: nil,
	}, {
		name: "invalid mode",
		p: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
			},
			Spec: PolicySpec{
				Mode:             "unknown",
				DefaultThreshold: 100,
				Stages:           []Stage{{0, nil}},
			},
		},
		want: apis.ErrInvalidValue("unknown", "spec.mode"),
	}, {
		name: "defaultThreshold missing",
		p: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
			},
			Spec: PolicySpec{
				Mode:   "time",
				Stages: []Stage{{0, nil}},
			},
		},
		want: apis.ErrGeneric("DefaultThreshold value is mandatory and must be a positive integer", "spec.defaultThreshold"),
	}, {
		name: "too few rollout Stages",
		p: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
			},
			Spec: PolicySpec{
				Mode:             "time",
				DefaultThreshold: 100,
				Stages:           []Stage{},
			},
		},
		want: apis.ErrGeneric("There must be at least one rollout stage in a Policy", "spec.stages"),
	}, {
		name: "unsorted stage percentages",
		p: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
			},
			Spec: PolicySpec{
				Mode:             "time",
				DefaultThreshold: 100,
				Stages:           []Stage{{0, nil}, {70, nil}, {50, nil}, {30, nil}},
			},
		},
		want: apis.ErrGeneric("Rollout percentages must be in increasing order", "spec.stages"),
	}, {
		name: "out of bounds percentage value",
		p: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
			},
			Spec: PolicySpec{
				Mode:             "time",
				DefaultThreshold: 100,
				Stages:           []Stage{{0, nil}, {101, nil}},
			},
		},
		want: apis.ErrOutOfBoundsValue(101, 0, 99, "spec.stages"),
	}, {
		name: "invalid optional threshold value",
		p: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "test",
			},
			Spec: PolicySpec{
				Mode:             "time",
				DefaultThreshold: 100,
				Stages:           []Stage{{0, nil}, {50, intptr(-1)}},
			},
		},
		want: apis.ErrGeneric("Optional threshold value must be a positive integer", "spec.stages"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.p.Validate(context.Background())
			if !cmp.Equal(test.want.Error(), got.Error()) {
				t.Errorf("Validate (-want, +got) = %v",
					cmp.Diff(test.want.Error(), got.Error()))
			}
		})
	}
}
