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
	"testing"
	"time"

	v1 "knative.dev/serving/pkg/apis/serving/v1"
	. "knative.dev/serving/pkg/testing/v1"
)

func TestShouldSkipConfig(t *testing.T) {
	var tests = []struct {
		name  string
		cfg  *v1.Configuration
		want  bool
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
		name  string
		cfg  *v1.Configuration
		want  bool
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

// for the below tests we are reusing the toy policies defined in policy_test.go

func TestIsTimestampExpired(t *testing.T) {
	var tests = []struct {
		name    string
		ltt     time.Time
		policy *Policy
		cp      int
		want    bool
	}{
		{name: "ltt is old enough, want true", ltt: time.Now().Add(-5 * time.Second), policy: &pa, cp: 99, want: true},
		{name: "ltt not quite old enough, want false", ltt: time.Now().Add(-4 * time.Second), policy: &pa, cp: 99, want: false},
		{name: "ltt exists in the future, want false", ltt: time.Now().Add(500 * time.Second), policy: &pd, cp: 7, want: false},
		{name: "getThreshold error, should return true", ltt: time.Now().Add(-20 * time.Second), policy: &p0, cp: 50, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := isTimestampExpired(tt.ltt, tt.policy, tt.cp)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
		})
	}
}

func TestIsRouteStatusUpToDate(t *testing.T) {
	var tests = []struct {
		name       string
		route     *v1.Route
		newRevName string
		policy    *Policy
		want       bool
	}{{
		name: "new R is not listed as target in current status",
		route: Route("default", "test", withTraffic("status", pair{"old1", 50}, pair{"old2", 50})),
		newRevName: "new",
		policy: &pa,
		want: false,
	}, {
		name: "new R is listed but timestamp has expired",
		route: Route("default", "test", withTraffic("status", pair{"old", 1}, pair{"new", 99}),
			WithRouteAnnotation(map[string]string{AnnotationKey: time.Now().Add(-10 * time.Second).Format(TimeFormat)})),
		newRevName: "new",
		policy: &pa,
		want: false,
	}, {
		name: "new R is listed and timestamp is still valid",
		route: Route("default", "test", withTraffic("status", pair{"old", 1}, pair{"new", 99}),
			WithRouteAnnotation(map[string]string{AnnotationKey: time.Now().Add(-1 * time.Second).Format(TimeFormat)})),
		newRevName: "new",
		policy: &pa,
		want: true,
	}, {
		name: "new R is listed and has reached 100 even though timestamp is old",
		route: Route("default", "test", withTraffic("status", pair{"new", 100}),
			WithRouteAnnotation(map[string]string{AnnotationKey: time.Now().Add(-1000 * time.Second).Format(TimeFormat)})),
		newRevName: "new",
		policy: &pa,
		want: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := isRouteStatusUpToDate(tt.route, tt.newRevName, tt.policy)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
		})
	}
}
