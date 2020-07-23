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
)

var (
	pa = Policy{"time", []Stage{{0, nil}, {1, nil}, {2, nil}, {3, nil}, {4, nil}, {5, nil}, {6, nil}, {7, nil}, {8, nil}, {99, nil}}, 5}
	pb = Policy{"request", []Stage{{0, nil}, {90, nil}, {91, nil}, {92, nil}, {93, nil}, {94, nil}, {95, nil}, {96, nil}, {97, nil}, {98, nil}, {99, nil}}, 500}
	pc = Policy{"error", []Stage{{0, nil}, {5, nil}, {20, nil}, {50, nil}, {80, nil}, {95, nil}}, 3}
	pd = Policy{"time", []Stage{
		{0, intptr(5)},
		{4, intptr(10)},
		{7, intptr(50)},
		{10, nil},
	}, 100}
	p0 = Policy{"time", []Stage{}, 10}
	pX = Policy{"request", []Stage{{90, nil}, {80, nil}, {70, nil}}, 5}
)

// knative.dev/pkg/ptr library doesn't have Int, so we need to implement it here
func intptr(x int) *int {
	return &x
}

func TestComputeNewPercent(t *testing.T) {
	var tests = []struct {
		name        string
		policy      *Policy
		cp          int // (c)urrent (p)ercent
		want        int
		errExpected bool
	}{
		{name: "policy A, current percent present", policy: &pa, cp: 3, want: 4, errExpected: false},
		{name: "policy A, current percent not present", policy: &pa, cp: 10, want: 0, errExpected: true},
		{name: "policy B, input is last stage", policy: &pb, cp: 99, want: 100, errExpected: false},
		{name: "policy B, current percent present", policy: &pb, cp: 0, want: 90, errExpected: false},
		{name: "policy C, input is last stage", policy: &pc, cp: 95, want: 100, errExpected: false},
		{name: "policy C, current percent present", policy: &pc, cp: 50, want: 80, errExpected: false},
		{name: "policy C, input 100 (error)", policy: &pc, cp: 100, want: 0, errExpected: true},
		{name: "empty policy (error)", policy: &p0, cp: 0, want: 0, errExpected: true},
		// the last test should have undefined behavior because policy Percents field must be sorted in increasing order
		// this test is considered passed as long as the test driver doesn't crash
		{name: "unsorted policy (undefined)", policy: &pX, cp: 90, want: -1, errExpected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans, e := computeNewPercent(tt.policy, tt.cp)
			if tt.want == -1 {
				return
			}
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
			if (tt.errExpected && e == nil) || (!tt.errExpected && e != nil) {
				t.Errorf("error output doesn't match")
			}
		})
	}
}

func TestGetThreshold(t *testing.T) {
	var tests = []struct {
		name        string
		policy      *Policy
		cp          int
		want        int
		errExpected bool
	}{
		{name: "policy A, return default threshold", policy: &pa, cp: 3, want: 5, errExpected: false},
		{name: "policy A, current percent not present", policy: &pa, cp: 10, want: 0, errExpected: true},
		{name: "empty policy (error)", policy: &p0, cp: 0, want: 0, errExpected: true},
		{name: "unsorted policy (undefined)", policy: &pX, cp: 90, want: -1, errExpected: true},
		{name: "policy D, stage specifies own threshold", policy: &pd, cp: 7, want: 50, errExpected: false},
		{name: "policy D, last stage with default threshold", policy: &pd, cp: 10, want: 100, errExpected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans, e := getThreshold(tt.policy, tt.cp)
			if tt.want == -1 {
				return
			}
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
			if (tt.errExpected && e == nil) || (!tt.errExpected && e != nil) {
				t.Errorf("error output doesn't match")
			}
		})
	}
}

func TestComputeNewPercentExplicit(t *testing.T) {
	var tests = []struct {
		name    string
		policy  *Policy
		elapsed time.Duration
		want    int
	}{
		{name: "policy A, elapsed time is halfway across a stage", policy: &pa, elapsed: 17 * time.Second, want: 4},
		{name: "policy D, elapsed halfway across non-uniform stages", policy: &pd, elapsed: 45 * time.Second, want: 7},
		{name: "policy B, very long elapsed time", policy: &pb, elapsed: 10000000 * time.Second, want: 100},
		{name: "policy A, elapsed time lies spot-on stage boundary", policy: &pa, elapsed: 25 * time.Second, want: 6},
		{name: "policy D, elapsed time lies spot-on final boundary", policy: &pd, elapsed: 160 * time.Second, want: 100},
		{name: "Empty policy always return 100", policy: &p0, elapsed: 0, want: 100},
		{name: "Unsorted policy doesn't affect result", policy: &pX, elapsed: 7 * time.Second, want: 70},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := computeNewPercentExplicit(tt.policy, tt.elapsed)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
		})
	}
}

func TestMetricTillNextStage(t *testing.T) {
	var tests = []struct {
		name    string
		policy  *Policy
		elapsed time.Duration
		want    int
	}{
		{name: "policy A, elapsed time is halfway across a stage", policy: &pa, elapsed: 17 * time.Second, want: 4},
		{name: "policy D, elapsed halfway across non-uniform stages", policy: &pd, elapsed: 45 * time.Second, want: 16},
		{name: "policy B, very long elapsed time", policy: &pb, elapsed: 10000000 * time.Second, want: math.MaxInt32},
		{name: "policy A, elapsed time lies spot-on stage boundary", policy: &pa, elapsed: 25 * time.Second, want: 6},
		{name: "policy D, elapsed time lies spot-on final boundary", policy: &pd, elapsed: 160 * time.Second, want: math.MaxInt32},
		{name: "Empty policy always return MAX INT", policy: &p0, elapsed: 0, want: math.MaxInt32},
		{name: "Unsorted policy doesn't affect result", policy: &pX, elapsed: 7 * time.Second, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ans := metricTillNextStage(tt.policy, tt.elapsed)
			if ans != tt.want {
				t.Errorf("wrong answer (got %v, want %v)", ans, tt.want)
			}
		})
	}
}
