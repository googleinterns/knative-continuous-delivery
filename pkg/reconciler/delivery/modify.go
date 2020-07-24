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

package delivery

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/clock"
	"knative.dev/pkg/ptr"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
)

/****************************************************************************************************************
   modifyRouteSpec assigns traffic to all active Revisions according to the algorithm on page 8 of go/mydesigndoc
   arguments:
   - route: the current Route object
   - r: a lister to query the Revisions by name
   - newRevName: name string of the latest ready Revision
   - policy: pointer to the Policy struct that commands the rollout process
   return values:
   - 1st value: a new route object whose spec field has been written with the desired state
   - 2nd value: error if anything goes wrong
****************************************************************************************************************/
func modifyRouteSpec(route *v1.Route, r map[string]*v1.Revision, newRevName string, policy *Policy, clock clock.Clock) (*v1.Route, error) {
	// assumption 1: the current Route Status traffic % are all non-zero (any zero entries would not have been written)
	// assumption 2: the current Route Status traffic entries are ordered from oldest to newest Revision
	// first identify whether or not newRevName is already in the pool
	nameListed := false
	for _, t := range route.Status.Traffic {
		if t.RevisionName == newRevName {
			nameListed = true
			break
		}
	}

	// make a slice container to hold the new traffic assignments, and an ordered, lightweight roster of the pool
	// that contains all current Revision names, INCLUDING the newest one
	ln := len(route.Status.Traffic)
	if !nameListed {
		ln = ln + 1
	}
	if ln == 1 {
		// when there's only 1 traffic target it can only be the newest Revision
		newRevision, ok := r[newRevName]
		if !ok {
			return route, fmt.Errorf("cannot find Revision %s in indexer", newRevName)
		}
		route.Spec.Traffic = []v1.TrafficTarget{{
			ConfigurationName: newRevision.OwnerReferences[0].Name,
			LatestRevision:    ptr.Bool(true),
			Percent:           ptr.Int64(100),
		}}
		return route, nil
	}
	traffic := make([]v1.TrafficTarget, ln) // container for holding traffic assignments
	roster := make([]string, ln)            // ordered list of all Revision names in the pool
	for i, t := range route.Status.Traffic {
		roster[i] = t.RevisionName
	}
	if len(route.Status.Traffic) < len(roster) {
		roster[len(roster)-1] = newRevName
	}

	// go through the roster in reverse order (newest to oldest) and assign traffic to each Revision
	alreadyAssigned := 0
	oldest := oldestRevision(r)
	for i := len(roster) - 1; i >= 0; i-- {
		revision, ok := r[roster[i]]
		if !ok {
			return route, fmt.Errorf("cannot find Revision %s in indexer", roster[i])
		}
		// exception for the oldest Revision
		if revision == oldest {
			traffic[i] = v1.TrafficTarget{
				RevisionName:   roster[i],
				LatestRevision: ptr.Bool(false),
				Percent:        ptr.Int64(int64(100 - alreadyAssigned)),
			}
			break
		}
		timeElapsed := clock.Since(revision.CreationTimestamp.Time)
		want := computeNewPercentExplicit(policy, timeElapsed)
		actual := min(want, 100-alreadyAssigned)
		alreadyAssigned += actual
		traffic[i] = v1.TrafficTarget{
			RevisionName:   roster[i],
			LatestRevision: ptr.Bool(false),
			Percent:        ptr.Int64(int64(actual)),
		}
		if alreadyAssigned >= 100 {
			traffic = traffic[i:] // eliminate all redundant 0 entries
			break
		}
	}

	// this deals with the case e.g. a 10/90 split progressing to 0/100 leaving only one traffic target behind
	// if we don't take care of this, then we might violate assumption 1 for future calls
	if len(traffic) == 1 {
		traffic[0] = v1.TrafficTarget{
			ConfigurationName: route.Name,
			LatestRevision:    ptr.Bool(true),
			Percent:           ptr.Int64(100),
		}
	}

	route.Spec.Traffic = traffic
	return route, nil
}
