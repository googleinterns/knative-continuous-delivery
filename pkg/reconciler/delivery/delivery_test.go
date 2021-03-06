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
	"strconv"
	"testing"
	"time"

	"github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery"
	. "github.com/googleinterns/knative-continuous-delivery/pkg/reconciler/testing/resources"
	"k8s.io/apimachinery/pkg/util/clock"
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
	var now = time.Now()
	var timer = clock.NewFakeClock(now)
	var tests = []struct {
		name        string
		route       *v1.Route
		revMap      map[string]*v1.Revision
		policy      *Policy
		clock       clock.Clock
		want        time.Duration
		errExpected bool
	}{{
		name:        "empty route returns MAX time duration",
		route:       Route("default", "test"),
		revMap:      nil,
		policy:      &pa,
		clock:       timer,
		want:        time.Duration(math.MaxInt32) * time.Second,
		errExpected: false,
	}, {
		name:  "route status has unknown target Revision (error)",
		route: Route("default", "test", withTraffic(WithSpecTraffic, pair{"unknown-1", 50}, pair{"unknown-2", 50})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1"),
			"R2": Revision("default", "R2"),
		},
		policy:      &pa,
		clock:       timer,
		want:        0,
		errExpected: true,
	}, {
		name:  "policy A, very old + redundant Revisions",
		route: Route("default", "test", withTraffic(WithSpecTraffic, pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(now.Add(-500*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(now.Add(-450*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(now.Add(-400*time.Second))),
			"R4": Revision("default", "R4", WithCreationTimestamp(now.Add(-350*time.Second))),
			"R5": Revision("default", "R5", WithCreationTimestamp(now.Add(-300*time.Second))),
		},
		policy:      &pa,
		clock:       timer,
		want:        time.Duration(math.MaxInt32) * time.Second,
		errExpected: false,
	}, {
		name:  "policy A, all Revisions in progress but must ignore R1",
		route: Route("default", "test", withTraffic(WithSpecTraffic, pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(now.Add(-24500*time.Millisecond))),
			"R2": Revision("default", "R2", WithCreationTimestamp(now.Add(-18500*time.Millisecond))),
			"R3": Revision("default", "R3", WithCreationTimestamp(now.Add(-12500*time.Millisecond))),
		},
		policy:      &pa,
		clock:       timer,
		want:        2 * time.Second,
		errExpected: false,
	}, {
		name:  "policy A, at least one Revision is very old",
		route: Route("default", "test", withTraffic(WithSpecTraffic, pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(now.Add(-500*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(now.Add(-18500*time.Millisecond))),
			"R3": Revision("default", "R3", WithCreationTimestamp(now.Add(-12*time.Second))),
		},
		policy:      &pa,
		clock:       timer,
		want:        2 * time.Second,
		errExpected: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans, e := timeTillNextEvent(tt.route, tt.revMap, tt.policy, tt.clock)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
			if (tt.errExpected && e == nil) || (!tt.errExpected && e != nil) {
				t.Errorf("error output doesn't match")
			}
		})
	}
}

func TestIsNameListed(t *testing.T) {
	var tests = []struct {
		name       string
		route      *v1.Route
		newRevName string
		want       bool
	}{{
		name:       "list does not contain newRevName",
		route:      Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 95}, pair{"R2", 5})),
		newRevName: "R3",
		want:       false,
	}, {
		name:       "list contains newRevName",
		route:      Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 95}, pair{"R2", 5})),
		newRevName: "R2",
		want:       true,
	}, {
		name:       "list is empty",
		route:      Route("default", "test"),
		newRevName: "DNE",
		want:       false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ans := isNameListed(test.route, test.newRevName)
			if ans != test.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, test.want)
			}
		})
	}
}

func TestModifyRouteSpec(t *testing.T) {
	var now = time.Now()
	var timer = clock.NewFakeClock(now)

	// create the test setup for a very large test
	// 200 revisions R1-R200 are created at the same time, and we added R201 just now
	var largeTestRevMap = make(map[string]*v1.Revision)
	for i := 1; i <= 200; i++ {
		revName := "R" + strconv.Itoa(i)
		largeTestRevMap[revName] = Revision("default", revName, WithCreationTimestamp(now.Add(time.Duration(i-3000)*time.Millisecond)))
	}
	largeTestRevMap["R201"] = Revision("default", "R201", WithCreationTimestamp(now.Add(-1*time.Second)))
	var largeTestRouteTraffic = make([]pair, 100)
	var largeTestRouteTrafficNew = make([]pair, 100)
	for i := 101; i <= 200; i++ {
		revName := "R" + strconv.Itoa(i)
		largeTestRouteTraffic[i-101] = pair{revName, 1}
	}
	for i := 0; i < 99; i++ {
		largeTestRouteTrafficNew[i] = largeTestRouteTraffic[i+1]
	}
	largeTestRouteTrafficNew[99] = pair{"R201", 1}

	var tests = []struct {
		name        string
		route       *v1.Route
		revMap      map[string]*v1.Revision
		newRevName  string
		policy      *Policy
		clock       clock.Clock
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
		clock:      timer,
		want: Route("default", "test", WithSpecTraffic(v1.TrafficTarget{
			ConfigurationName: "new",
			LatestRevision:    ptr.Bool(true),
			Percent:           ptr.Int64(100),
		})),
		errExpected: false,
	}, {
		name:  "newRevName is new, adds to an existing pool",
		route: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 95}, pair{"R2", 5})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(now.Add(-10000*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(now.Add(-21*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(now)),
		},
		newRevName: "R3",
		policy:     &pa,
		clock:      timer,
		want: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 95}, pair{"R2", 5}),
			withTraffic(WithSpecTraffic, pair{"R1", 94}, pair{"R2", 5}, pair{"R3", 1})),
		errExpected: false,
	}, {
		name:  "promotion, but pool size doesn't change",
		route: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 94}, pair{"R2", 5}, pair{"R3", 1})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(now.Add(-10000*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(now.Add(-26*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(now.Add(-2*time.Second))),
		},
		newRevName: "R3",
		policy:     &pa,
		clock:      timer,
		want: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 94}, pair{"R2", 5}, pair{"R3", 1}),
			withTraffic(WithSpecTraffic, pair{"R1", 93}, pair{"R2", 6}, pair{"R3", 1})),
		errExpected: false,
	}, {
		name:  "promotion, and pool size shrinks",
		route: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(now.Add(-10000*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(now.Add(-41*time.Second))),
			"R3": Revision("default", "R3", WithCreationTimestamp(now.Add(-33*time.Second))),
		},
		newRevName: "R3",
		policy:     &pa,
		clock:      timer,
		want: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 85}, pair{"R2", 8}, pair{"R3", 7}),
			withTraffic(WithSpecTraffic, pair{"R2", 93}, pair{"R3", 7})),
		errExpected: false,
	}, {
		name:  "oldest revision always ignores progression/timer",
		route: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 99}, pair{"R2", 1})),
		revMap: map[string]*v1.Revision{
			"R1": Revision("default", "R1", WithCreationTimestamp(now.Add(-11*time.Second))),
			"R2": Revision("default", "R2", WithCreationTimestamp(now.Add(-5150*time.Millisecond))),
		},
		newRevName: "R2",
		policy:     &pa,
		clock:      timer,
		want: Route("default", "test", withTraffic(WithStatusTraffic, pair{"R1", 99}, pair{"R2", 1}),
			withTraffic(WithSpecTraffic, pair{"R1", 98}, pair{"R2", 2})),
		errExpected: false,
	}, {
		name:       "> 100 revisions split traffic at 1% granularity",
		route:      Route("default", "test", withTraffic(WithStatusTraffic, largeTestRouteTraffic...)),
		revMap:     largeTestRevMap,
		newRevName: "R201",
		policy:     &pa,
		clock:      timer,
		want: Route("default", "test", withTraffic(WithStatusTraffic, largeTestRouteTraffic...),
			withTraffic(WithSpecTraffic, largeTestRouteTrafficNew...)),
		errExpected: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans, e := modifyRouteSpec(tt.route, tt.revMap, tt.newRevName, tt.policy, tt.clock)
			if diff := cmp.Diff(tt.want, ans); diff != "" {
				t.Errorf("Route object is incorrect (-want, +got): %s", diff)
			}
			if (tt.errExpected && e == nil) || (!tt.errExpected && e != nil) {
				t.Errorf("error output doesn't match")
			}
		})
	}
}

func TestOldestRevision(t *testing.T) {
	var now = time.Now()
	var rev1 = Revision("default", "R1", WithCreationTimestamp(now.Add(-500*time.Second)))
	var rev2 = Revision("default", "R2", WithCreationTimestamp(now.Add(200*time.Second)))
	var rev3 = Revision("default", "R3", WithCreationTimestamp(now.Add(-100*time.Second)))
	var rev4 = Revision("default", "R4", WithCreationTimestamp(now))
	var tests = []struct {
		name   string
		revMap map[string]*v1.Revision
		want   *v1.Revision
	}{{
		name: "simple test with 4 revisions",
		revMap: map[string]*v1.Revision{
			"R1": rev1,
			"R2": rev2,
			"R3": rev3,
			"R4": rev4,
		},
		want: rev1,
	}, {
		name:   "empty map, return nil",
		revMap: map[string]*v1.Revision{},
		want:   nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := oldestRevision(tt.revMap)
			if diff := cmp.Diff(tt.want, ans); diff != "" {
				t.Errorf("wrong answer (-want, +got): %s", diff)
			}
		})
	}
}

func TestIdentifyPolicy(t *testing.T) {
	var tests = []struct {
		name          string
		cfg           *v1.Configuration
		wantNamespace string
		wantName      string
	}{{
		name: "no prefix, defaults to cfg namespace",
		cfg: &v1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfg-namespace",
				Name:      "cfg-name",
				Annotations: map[string]string{
					delivery.PolicyNameKey: "policy-name",
				},
			},
		},
		wantNamespace: "cfg-namespace",
		wantName:      "policy-name",
	}, {
		name: "prefix with one slash, splits in half",
		cfg: &v1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfg-namespace",
				Name:      "cfg-name",
				Annotations: map[string]string{
					delivery.PolicyNameKey: "policy-namespace/policy-name",
				},
			},
		},
		wantNamespace: "policy-namespace",
		wantName:      "policy-name",
	}, {
		name: "prefix with one slash, namespace == name",
		cfg: &v1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfg-namespace",
				Name:      "cfg-name",
				Annotations: map[string]string{
					delivery.PolicyNameKey: "same/same",
				},
			},
		},
		wantNamespace: "same",
		wantName:      "same",
	}, {
		name: "begins with slash, empty namespace",
		cfg: &v1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfg-namespace",
				Name:      "cfg-name",
				Annotations: map[string]string{
					delivery.PolicyNameKey: "/policy-name",
				},
			},
		},
		wantNamespace: "",
		wantName:      "policy-name",
	}, {
		name: "multiple slashes, only the first one is delimiter",
		cfg: &v1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "cfg-namespace",
				Name:      "cfg-name",
				Annotations: map[string]string{
					delivery.PolicyNameKey: "policy-namespace/something/random/in/between/policy-name",
				},
			},
		},
		wantNamespace: "policy-namespace",
		wantName:      "something/random/in/between/policy-name",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotNamespace, gotName := identifyPolicy(test.cfg)
			if gotNamespace != test.wantNamespace {
				t.Errorf("incorrect namespace (got %v, want %v)", gotNamespace, test.wantNamespace)
			}
			if gotName != test.wantName {
				t.Errorf("incorrect name (got %v, want %v)", gotName, test.wantName)
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
