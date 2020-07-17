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
