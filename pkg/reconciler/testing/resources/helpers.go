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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	testingv1 "knative.dev/serving/pkg/testing/v1"
)

// Configuration creates a configuration with ConfigOptions
func Configuration(namespace, name string, co ...testingv1.ConfigOption) *v1.Configuration {
	c := &v1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range co {
		opt(c)
	}
	c.SetDefaults(context.Background())
	return c
}

// Revision creates a revision with RevisionOptions
func Revision(namespace, name string, ro ...testingv1.RevisionOption) *v1.Revision {
	r := &v1.Revision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range ro {
		opt(r)
	}
	r.SetDefaults(context.Background())
	return r
}
