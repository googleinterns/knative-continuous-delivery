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
	"math"
	"testing"
	"time"

	"knative.dev/pkg/ptr"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	. "knative.dev/serving/pkg/testing/v1"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldSkipConfig(t *testing.T) {
	var tests = []struct {
		name string
		cfg  *v1.Configuration
		want bool
	}{
		{name: "namespace matches, but name doesn't", cfg: Configuration(KCDNamespace, "random"), want: false},
		{name: "name matches, but namespace doesn't", cfg: Configuration("random", KCDName), want: false},
		{name: "namespace and name both match", cfg: Configuration(KCDNamespace, KCDName), want: true},
		{name: "neither namespace nor name matches", cfg: Configuration("random_namespace", "random_name"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := shouldSkipConfig(tt.cfg)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
		})
	}
}

func TestConfigReady(t *testing.T) {
	var tests = []struct {
		name string
		cfg  *v1.Configuration
		want bool
	}{
		{name: "latestReady and latestCreated don't exist", cfg: Configuration("default", "test"), want: false},
		{name: "latestCreated present without latestReady", cfg: Configuration("default", "test", WithLatestCreated("not-ready")), want: false},
		{name: "latestCreated and latestReady are different", cfg: Configuration("default", "test", WithLatestCreated("new"), WithLatestReady("old")), want: false},
		{name: "latestCreated is also ready", cfg: Configuration("default", "test", WithLatestCreated("ok"), WithLatestReady("ok")), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := configReady(tt.cfg)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	var tests = []struct {
		name  string
		items []int
		want  int
	}{
		{name: "return the only item when only 1 item is present", items: []int{5}, want: 5},
		{name: "many diverse items", items: []int{9, 10, -2, 7, -5, 0, 3, -13, 8}, want: -13},
		{name: "return MAX INT when it is the smallest", items: []int{math.MaxInt32, math.MaxInt32}, want: math.MaxInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := min(tt.items...)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
		})
	}
}

func TestTimeTillNextEvent(t *testing.T) {
	var tests = []struct {
		name        string
		route       *v1.Route
		revMap      map[string]*v1.Revision
		policy      *Policy
		want        time.Duration
		errExpected bool
	}{{
		name:        "empty route returns MAX time duration",
		route:       Route("default", "test"),
		revMap:      nil,
		policy:      &pa,
		want:        time.Duration(math.MaxInt32) * time.Second,
		errExpected: false,
	}, {
		name:  "route status has unknown target Revision (error)",
		route: Route("default", "test", withTraffic("spec", pair{"unknown-1", 50}, pair{"unknown-2", 50})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1"),
			"R2": Revision("default", "R2"),
		},
		policy:      &pa,
		want:        0,
		errExpected: true,
	}, {
		name:  "policy A, very old + redundant Revisions",
		route: Route("default", "test", withTraffic("spec", pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(time.Now().Add(-500*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(time.Now().Add(-450*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(time.Now().Add(-400*time.Second))),
			"R4": Revision("default", "R4", WithCreationTimestamp(time.Now().Add(-350*time.Second))),
			"R5": Revision("default", "R5", WithCreationTimestamp(time.Now().Add(-300*time.Second))),
		},
		policy:      &pa,
		want:        time.Duration(math.MaxInt32) * time.Second,
		errExpected: false,
	}, {
		name:  "policy A, all Revisions in progress",
		route: Route("default", "test", withTraffic("spec", pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(time.Now().Add(-24500*time.Millisecond))),
			"R2": Revision("default", "R2", WithCreationTimestamp(time.Now().Add(-18*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(time.Now().Add(-12*time.Second))),
		},
		policy:      &pa,
		want:        time.Second,
		errExpected: false,
	}, {
		name:  "policy A, at least one Revision is very old",
		route: Route("default", "test", withTraffic("spec", pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(time.Now().Add(-500*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(time.Now().Add(-18500*time.Millisecond))),
			"R3": Revision("default", "R3", WithCreationTimestamp(time.Now().Add(-12*time.Second))),
		},
		policy:      &pa,
		want:        2 * time.Second,
		errExpected: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans, e := timeTillNextEvent(tt.route, tt.revMap, tt.policy)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
			if (tt.errExpected && e == nil) || (!tt.errExpected && e != nil) {
				t.Errorf("error output doesn't match")
			}
		})
	}
}

func TestModifyRouteSpec(t *testing.T) {
	var tests = []struct {
		name        string
		route       *v1.Route
		revMap      map[string]*v1.Revision
		newRevName  string
		policy      *Policy
		want        *v1.Route
		errExpected bool
	}{{
		name:  "newRevName is the only thing in the pool",
		route: Route("default", "test"),
		revMap: map[string]*v1.Revision{
			"new": Revision("default", "new", withOwnerReferences([]metav1.OwnerReference{{
				Kind: "Configuration",
				Name: "new",
			}})),
		},
		newRevName: "new",
		policy:     &pa,
		want: Route("default", "test", WithSpecTraffic(v1.TrafficTarget{
			ConfigurationName: "new",
			LatestRevision:    ptr.Bool(true),
			Percent:           ptr.Int64(100),
		})),
		errExpected: false,
	}, {
		name:  "newRevName is new, adds to an existing pool",
		route: Route("default", "test", withTraffic("status", pair{"R1", 95}, pair{"R2", 5})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(time.Now().Add(-10000*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(time.Now().Add(-21*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(time.Now())),
		},
		newRevName: "R3",
		policy:     &pa,
		want: Route("default", "test", withTraffic("status", pair{"R1", 95}, pair{"R2", 5}),
			withTraffic("spec", pair{"R1", 94}, pair{"R2", 5}, pair{"R3", 1})),
		errExpected: false,
	}, {
		name:  "promotion, but pool size doesn't change",
		route: Route("default", "test", withTraffic("status", pair{"R1", 94}, pair{"R2", 5}, pair{"R3", 1})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(time.Now().Add(-10000*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(time.Now().Add(-26*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(time.Now().Add(-2*time.Second))),
		},
		newRevName: "R3",
		policy:     &pa,
		want: Route("default", "test", withTraffic("status", pair{"R1", 94}, pair{"R2", 5}, pair{"R3", 1}),
			withTraffic("spec", pair{"R1", 93}, pair{"R2", 6}, pair{"R3", 1})),
		errExpected: false,
	}, {
		name:  "promotion, and pool size shrinks",
		route: Route("default", "test", withTraffic("status", pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(time.Now().Add(-10000*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(time.Now().Add(-41*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(time.Now().Add(-33*time.Second))),
		},
		newRevName: "R3",
		policy:     &pa,
		want: Route("default", "test", withTraffic("status", pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7}),
			withTraffic("spec", pair{"R2", 93}, pair{"R3", 7})),
		errExpected: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans, e := modifyRouteSpec(tt.route, tt.revMap, tt.newRevName, tt.policy)
			if diff := cmp.Diff(tt.want, ans); diff != "" {
				t.Errorf("Route object is incorrect (-want, +got): %s", diff)
			}
			if (tt.errExpected && e == nil) || (!tt.errExpected && e != nil) {
				t.Errorf("error output doesn't match")
			}
		})
	}
}

// withOwnerReferences sets the OwnerReferences of a Revision
func withOwnerReferences(references []metav1.OwnerReference) RevisionOption {
	return func(rev *v1.Revision) {
		rev.ObjectMeta.SetOwnerReferences(references)
	}
}
