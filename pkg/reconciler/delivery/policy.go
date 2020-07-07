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
	"sort"
	"fmt"
)


// Policy represents the rollout strategy used to update Route objects
type Policy struct {
	// Mode specifies the metric that the policy is based on
	// Possible values are: "time", "request", "error"
	Mode string

	// Percents specifies the traffic percentages that the NEW Revision is expected to have
	// at successive rollout stages; the list of integers must start at 0
	// all entries must be in the range [0, 100), and must be sorted in increasing order
	// Technically the final rollout percentage is 100, but this is implicitly understood,
	// and should NOT be explicitly specified in Percents
	// In addition to the traffic percentages, each stage can OPTIONALLY specify its own threshold
	// this gives greater flexibility to policy design
	// The threshold value for stage N is the value that must be achieved BEFORE moving to stage N+1
	Percents []Stage

	// DefaultThreshold is the threshold value that is used when a rollout stage doesn't specify
	// a threshold of its own; this can be useful when the threshold is a constant value across 
	// all rollout stages, in which case there is no need to copy paste the same value in all entries
	// The interpretation of DefaultThreshold depends on the value of Mode
	DefaultThreshold int
}

// Stage contains information about a progressive rollout stage
type Stage struct {
	Percent    int
	Threshold *int
}

// computeNewPercent calculates, given a Policy and the current rollout stage,
// the traffic percentage for the NEW Revision in the next rollout stage
func computeNewPercent(p *Policy, currentPercent int) (int, error) {
	i := sort.Search(len(p.Percents), func(i int) bool {
		return p.Percents[i].Percent >= currentPercent
	})
	if i < len(p.Percents) && p.Percents[i].Percent == currentPercent {
		if i == len(p.Percents) - 1 {
			return 100, nil
		}
		return p.Percents[i + 1].Percent, nil
	}
	return 0, fmt.Errorf("invalid percentage for current rollout stage")
}

// getThreshold returns, given the percentage for a rollout stage, its corresponding threshold value
// if the threshold value isn't specified, DefaultThreshold is used
func getThreshold(p *Policy, currentPercent int) (int, error) {
	i := sort.Search(len(p.Percents), func(i int) bool {
		return p.Percents[i].Percent >= currentPercent
	})
	if i < len(p.Percents) && p.Percents[i].Percent == currentPercent {
		if p.Percents[i].Threshold != nil {
			return *p.Percents[i].Threshold, nil
		}
		return p.DefaultThreshold, nil
	}
	return 0, fmt.Errorf("invalid percentage for current rollout stage")
}
