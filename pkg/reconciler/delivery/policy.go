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
	"math"
	"sort"
	"time"
)

// Policy represents the rollout strategy used to update Route objects
type Policy struct {
	// Mode specifies the metric that the policy is based on
	// Possible values are: "time", "request", "error"
	Mode string

	// Stages specifies the traffic percentages that the NEW Revision is expected to have
	// at successive rollout stages; the list of integers must start at 0
	// all entries must be in the range [0, 100), and must be sorted in increasing order
	// Technically the final rollout percentage is 100, but this is implicitly understood,
	// and should NOT be explicitly specified in Stages
	// In addition to the traffic percentages, each stage can OPTIONALLY specify its own threshold
	// this gives greater flexibility to policy design
	// The threshold value for stage N is the value that must be achieved BEFORE moving to stage N+1
	Stages []Stage

	// DefaultThreshold is the threshold value that is used when a rollout stage doesn't specify
	// a threshold of its own; this can be useful when the threshold is a constant value across
	// all rollout stages, in which case there is no need to copy paste the same value in all entries
	// The interpretation of DefaultThreshold depends on the value of Mode
	DefaultThreshold int
}

// Stage contains information about a progressive rollout stage
type Stage struct {
	Percent   int
	Threshold *int
}

// computeNewPercent calculates, given a Policy and the current rollout stage,
// the traffic percentage for the NEW Revision in the next rollout stage
func computeNewPercent(p *Policy, currentPercent int) (int, error) {
	i := sort.Search(len(p.Stages), func(i int) bool {
		return p.Stages[i].Percent >= currentPercent
	})
	if i < len(p.Stages) && p.Stages[i].Percent == currentPercent {
		if i == len(p.Stages)-1 {
			return 100, nil
		}
		return p.Stages[i+1].Percent, nil
	}
	return 0, fmt.Errorf("invalid percentage for current rollout stage")
}

// getThreshold returns, given the percentage for a rollout stage, its corresponding threshold value
// if the threshold value isn't specified, DefaultThreshold is used
func getThreshold(p *Policy, currentPercent int) (int, error) {
	i := sort.Search(len(p.Stages), func(i int) bool {
		return p.Stages[i].Percent >= currentPercent
	})
	if i < len(p.Stages) && p.Stages[i].Percent == currentPercent {
		if p.Stages[i].Threshold != nil {
			return *p.Stages[i].Threshold, nil
		}
		return p.DefaultThreshold, nil
	}
	return 0, fmt.Errorf("invalid percentage for current rollout stage")
}

// computeNewPercentExplicit is an explicit way of computing a percentage without relying on the previous stage
// elapsed is the total time duration since the beginning of the rollout
// this function doesn't return an error because an error is impossible
func computeNewPercentExplicit(p *Policy, elapsed time.Duration) int {
	// when no stages are specified, we assume everything is automatically promoted to 100
	if len(p.Stages) == 0 {
		return 100
	}
	metric := float64(elapsed) / float64(time.Second)
	metricCumulative := 0
	for _, s := range p.Stages[1:] {
		extra := p.DefaultThreshold
		if s.Threshold != nil {
			extra = *s.Threshold
		}
		metricCumulative += extra
		if float64(metricCumulative) > metric {
			return s.Percent
		}
	}
	return 100
}

// metricTillNextStage computes how much time (full seconds) to wait before progressing to the next stage
// the returned result in full seconds MUST be STRICTLY bigger than the actual time to wait
func metricTillNextStage(p *Policy, elapsed time.Duration) int {
	// when no stages are specified, we assume that the final stage is reached immediately after initiation
	if len(p.Stages) == 0 {
		return math.MaxInt32
	}
	metric := float64(elapsed) / float64(time.Second)
	metricCumulative := 0
	for _, s := range p.Stages[1:] {
		extra := p.DefaultThreshold
		if s.Threshold != nil {
			extra = *s.Threshold
		}
		metricCumulative += extra
		if float64(metricCumulative) > metric {
			return nextBiggerInt(float64(metricCumulative) - metric)
		}
	}
	return math.MaxInt32
}

// nextBiggerInt computes the next STRICTLY bigger int for a float64 number
func nextBiggerInt(f float64) int {
	return int(f) + 1
}
