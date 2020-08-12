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

	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Validate implements apis.Validatable
func (p *Policy) Validate(ctx context.Context) *apis.FieldError {
	logging.FromContext(ctx).Infof("Validate called for %v", *p)
	var err *apis.FieldError
	// validate that the mode value must be "time" ("request" and "error" not supported for now)
	if p.Spec.Mode != "time" {
		err = err.Also(apis.ErrInvalidValue(p.Spec.Mode, "spec.mode"))
	}
	// validate that the defaultThreshold must be present and positive
	if p.Spec.DefaultThreshold <= 0 {
		err = err.Also(apis.ErrGeneric("DefaultThreshold value is mandatory and must be a positive integer", "spec.defaultThreshold"))
	}
	// validate that there is at least 1 stage
	if len(p.Spec.Stages) < 1 {
		err = err.Also(apis.ErrGeneric("There must be at least one rollout stage in a Policy", "spec.stages"))
		return err // no need for further checking
	}
	// validate all stages and check:
	// (1) all percents are in increasing order
	// (2) all percents are within range [0, 100)
	// (3) the optional threshold, if specified, must be a positive integer
	prev := 0
	for _, s := range p.Spec.Stages {
		if s.Percent < prev {
			err = err.Also(apis.ErrGeneric("Rollout percentages must be in increasing order", "spec.stages"))
			break
		}
		if s.Percent < 0 || s.Percent >= 100 {
			err = err.Also(apis.ErrOutOfBoundsValue(s.Percent, 0, 99, "spec.stages"))
			break
		}
		if s.Threshold != nil && *s.Threshold <= 0 {
			err = err.Also(apis.ErrGeneric("Optional threshold value must be a positive integer", "spec.stages"))
			break
		}
		prev = s.Percent
	}
	return err
}
