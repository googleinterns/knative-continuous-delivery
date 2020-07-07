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
		policy     *Policy
		cp          int
		want        int
		errExpected bool
	}{
		{"PA_present", &pa, 3, 4, false},
		{"PA_not_present", &pa, 10, 0, true},
		{"PB_last", &pb, 99, 100, false},
		{"PB_present", &pb, 0, 90, false},
		{"PC_last", &pc, 95, 100, false},
		{"PC_not_present", &pc, 50, 80, false},
		{"PC_100", &pc, 100, 0, true},
		{"P0_empty", &p0, 0, 0, true},
		// the last test should have undefined behavior because policy Percents field must be sorted in increasing order
		// this test is considered passed as long as the test driver doesn't crash
		{"PX_unsorted", &pX, 90, -1, true},
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
		policy     *Policy
		cp          int
		want        int
		errExpected bool
	}{
		{"PA_use_default", &pa, 3, 5, false},
		{"PA_not_present", &pa, 10, 0, true},
		{"P0_empty", &p0, 0, 0, true},
		{"PX_unsorted", &pX, 90, -1, true},
		{"PD_use_threshold", &pd, 7, 50, false},
		{"PD_last_default", &pd, 10, 100, false},
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
