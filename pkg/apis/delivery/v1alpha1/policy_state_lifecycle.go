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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

var policyStateCondSet = apis.NewLivingConditionSet()

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*PolicyState) GetConditionSet() apis.ConditionSet {
	return policyStateCondSet
}

// GetGroupVersionKind returns the GroupVersionKind.
func (ps *PolicyState) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("PolicyState")
}

// IsReady returns if the route is ready to serve the requested configuration.
func (pss *PolicyStateStatus) IsReady() bool {
	return policyStateCondSet.Manage(pss).IsHappy()
}

// InitializeConditions sets the initial values to the conditions.
func (pss *PolicyStateStatus) InitializeConditions() {
	policyStateCondSet.Manage(pss).InitializeConditions()
}
