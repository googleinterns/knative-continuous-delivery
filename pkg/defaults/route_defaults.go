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

	"knative.dev/pkg/logging"

	"github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery/v1alpha1"
	deliveryclient "github.com/googleinterns/knative-continuous-delivery/pkg/client/injection/client"
	policystateinformer "github.com/googleinterns/knative-continuous-delivery/pkg/client/injection/informers/delivery/v1alpha1/policystate"
	"knative.dev/pkg/apis"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContinuousDeploymentRoute is a wrapper around Route for setting Continuous Deployment values
type ContinuousDeploymentRoute struct {
	servingv1.Route `json:",inline"`
}

var (
	// Check that the wrapper route can be defaulted.
	_ apis.Defaultable = (*ContinuousDeploymentRoute)(nil)
	_ apis.Validatable = (*ContinuousDeploymentRoute)(nil)
)

// SetDefaults implements apis.Defaultable
func (cdr *ContinuousDeploymentRoute) SetDefaults(ctx context.Context) {
	logger := logging.FromContext(ctx)
	policyStateInformer := policystateinformer.Get(ctx)
	policyStateLister := policyStateInformer.Lister()
	ps, err := policyStateLister.PolicyStates(cdr.Namespace).Get(cdr.Name)
	if err != nil {
		return
	}
	logger.Infof("Received PolicyState %v", *ps)

	cdr.copyRouteSpec(ps)

	// update PolicyState status field
	ps.Status.Traffic = ps.Spec.Traffic
	_, err = deliveryclient.Get(ctx).DeliveryV1alpha1().PolicyStates(cdr.Namespace).Update(ps)
	if err != nil {
		logger.Infof("Failed to update PolicyState")
	}
}

func (cdr *ContinuousDeploymentRoute) copyRouteSpec(ps *v1alpha1.PolicyState) {
	cdr.Spec = servingv1.RouteSpec{
		Traffic: ps.Spec.Traffic,
	}
}

// Validate returns nil due to no need for validation
func (cdr *ContinuousDeploymentRoute) Validate(ctx context.Context) *apis.FieldError {
	logger := logging.FromContext(ctx)
	logger.Infof("Validate called for %v", *cdr)
	return nil
}
