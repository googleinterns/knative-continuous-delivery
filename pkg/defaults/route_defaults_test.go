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

package defaults

import (
	"context"
	"testing"

	"github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
	"knative.dev/serving/pkg/apis/serving/v1"

	"github.com/google/go-cmp/cmp"
)

func TestCopyRouteSpec(t *testing.T) {
	tests := []struct {
		name string
		ps   *v1alpha1.PolicyState
		in   *ContinuousDeploymentRoute
		want *ContinuousDeploymentRoute
	}{{
		name: "simple copy pasting of PolicyState spec",
		ps: &v1alpha1.PolicyState{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: v1alpha1.PolicyStateSpec{
				Traffic: []v1.TrafficTarget{{
					ConfigurationName: "test",
					LatestRevision:    ptr.Bool(true),
					Percent:           ptr.Int64(100),
				}},
			},
		},
		in: &ContinuousDeploymentRoute{v1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
		}},
		want: &ContinuousDeploymentRoute{v1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: v1.RouteSpec{
				Traffic: []v1.TrafficTarget{{
					ConfigurationName: "test",
					LatestRevision:    ptr.Bool(true),
					Percent:           ptr.Int64(100),
				}},
			},
		}},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			got.copyRouteSpec(test.ps)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("ContinuousDeploymentRoute object is incorrect (-want, +got): %s", diff)
			}
		})
	}
}

// we aren't implementing Validate but we still "test" it for the sake of consistency
func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		in   *ContinuousDeploymentRoute
		want *apis.FieldError
	}{{
		name: "return nil directly (not doing validation)",
		in:   &ContinuousDeploymentRoute{},
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in.Validate(context.Background())
			if got != test.want {
				t.Errorf("No error expected but got %v", got.Error())
			}
		})
	}
}
