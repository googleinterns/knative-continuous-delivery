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
)

func TestPolicyDefaulting(t *testing.T) {
	var tests = []struct {
		name string
		in   *Policy
		want *Policy
	}{{
		name: "nothing should change",
		in: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: PolicySpec{
				Mode:             "time",
				DefaultThreshold: 50,
				Stages:           []Stage{{10, intptr(20)}, {20, intptr(30)}, {50, nil}},
			},
		},
		want: &Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: PolicySpec{
				Mode:             "time",
				DefaultThreshold: 50,
				Stages:           []Stage{{10, intptr(20)}, {20, intptr(30)}, {50, nil}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			got.SetDefaults(context.Background())
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("Policy object is incorrect (-want, +got): %s", diff)
			}
		})
	}
}