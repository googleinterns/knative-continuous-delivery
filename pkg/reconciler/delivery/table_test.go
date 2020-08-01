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
	"context"
	"fmt"
	"testing"
	"time"

	deliveryclient "github.com/googleinterns/knative-continuous-delivery/pkg/client/injection/client"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"knative.dev/serving/pkg/apis/serving"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclient "knative.dev/serving/pkg/client/injection/client"
	configurationreconciler "knative.dev/serving/pkg/client/injection/reconciler/serving/v1/configuration"

	. "github.com/googleinterns/knative-continuous-delivery/pkg/reconciler/testing"
	. "github.com/googleinterns/knative-continuous-delivery/pkg/reconciler/testing/resources"
	. "knative.dev/pkg/reconciler/testing"
	. "knative.dev/serving/pkg/testing/v1"

	"github.com/google/go-cmp/cmp"
)

func TestReconcile(t *testing.T) {
	now := time.Now()
	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "does nothing when event refers to KCD",
		Key:  "default/test1",
		Objects: []runtime.Object{
			Route("default", "test1", WithConfigTarget("test1"), WithRouteGeneration(1)),
			Configuration(KCDNamespace, KCDName),
		},
		PostConditions: []func(*testing.T, *TableRow){
			assertNoEventQueued("default/test1"),
		},
	}, {
		Name: "does nothing when latest created is not ready",
		Key:  "default/test2",
		Objects: []runtime.Object{
			Route("default", "test2", WithConfigTarget("test2"), WithRouteGeneration(1)),
			Configuration("default", "test2", WithLatestCreated("rev-2"), WithLatestReady("rev-1")),
		},
		PostConditions: []func(*testing.T, *TableRow){
			assertEventQueued("default/test2", 5*time.Second),
		},
	}, {
		Name: "degenerate to simple logic with only 2 Revisions",
		Key:  "default/test3",
		Objects: []runtime.Object{
			Route("default", "test3", withTraffic(WithStatusTraffic, pair{"R1", 99}, pair{"R2", 1})),
			Configuration("default", "test3", WithLatestCreated("R2"), WithLatestReady("R2")),
			Revision("default", "R1", WithCreationTimestamp(now.Add(-125*time.Second)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test3")),
			Revision("default", "R2", WithCreationTimestamp(now.Add(-61100*time.Millisecond)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test3")),
			PolicyState("default", "test3"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: Route("default", "test3", withTraffic(WithStatusTraffic, pair{"R1", 99}, pair{"R2", 1}),
				withTraffic(WithSpecTraffic, pair{"R1", 90}, pair{"R2", 10})),
		}, {
			Object: PolicyState("default", "test3", withPSTraffic(WithPSSpecTraffic, pair{"R1", 90}, pair{"R2", 10})),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			assertEventQueued("default/test3", 59*time.Second),
		},
	}, {
		Name: "many Revisions squeeze out the oldest one",
		Key:  "default/test4",
		Objects: []runtime.Object{
			Route("default", "test4", withTraffic(WithStatusTraffic, pair{"R1", 58}, pair{"R2", 10}, pair{"R3", 10}, pair{"R4", 10}, pair{"R5", 10}, pair{"R6", 1}, pair{"R7", 1})),
			Configuration("default", "test4", WithLatestCreated("R7"), WithLatestReady("R7")),
			Revision("default", "R1", WithCreationTimestamp(now.Add(-125*time.Second)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test4")),
			Revision("default", "R2", WithCreationTimestamp(now.Add(-121500*time.Millisecond)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test4")),
			Revision("default", "R3", WithCreationTimestamp(now.Add(-121500*time.Millisecond)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test4")),
			Revision("default", "R4", WithCreationTimestamp(now.Add(-121500*time.Millisecond)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test4")),
			Revision("default", "R5", WithCreationTimestamp(now.Add(-121500*time.Millisecond)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test4")),
			Revision("default", "R6", WithCreationTimestamp(now.Add(-62100*time.Millisecond)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test4")),
			Revision("default", "R7", WithCreationTimestamp(now.Add(-61500*time.Millisecond)),
				WithRevisionLabel(serving.ConfigurationLabelKey, "test4")),
			PolicyState("default", "test4"),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: Route("default", "test4", withTraffic(WithStatusTraffic, pair{"R1", 58}, pair{"R2", 10}, pair{"R3", 10}, pair{"R4", 10}, pair{"R5", 10}, pair{"R6", 1}, pair{"R7", 1}),
				withTraffic(WithSpecTraffic, pair{"R2", 20}, pair{"R3", 20}, pair{"R4", 20}, pair{"R5", 20}, pair{"R6", 10}, pair{"R7", 10})),
		}, {
			Object: PolicyState("default", "test4",
				withPSTraffic(WithPSSpecTraffic, pair{"R2", 20}, pair{"R3", 20}, pair{"R4", 20}, pair{"R5", 20}, pair{"R6", 10}, pair{"R7", 10})),
		}},
		PostConditions: []func(*testing.T, *TableRow){
			assertEventQueued("default/test4", 58*time.Second),
		},
	}}
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, tr *TableRow) controller.Reconciler {
		tr.OtherTestData = make(map[string]interface{})
		r := &Reconciler{
			client:            servingclient.Get(ctx),
			psclient:          deliveryclient.Get(ctx),
			routeLister:       listers.GetRouteLister(),
			revisionLister:    listers.GetRevisionLister(),
			policystateLister: listers.GetPolicyStateLister(),
			clock:             clock.NewFakeClock(now),
			// note that we manually, systematically assigned unique namespace/name strings to each test Configuration
			// we use those strings for each test
			followup: func(cfg *v1.Configuration, t time.Duration) {
				key := cfg.GetNamespace() + "/" + cfg.GetName()
				tr.OtherTestData[key] = fmt.Sprintf("%v", t)
			},
		}
		return configurationreconciler.NewReconciler(ctx, logging.FromContext(ctx), servingclient.Get(ctx),
			listers.GetConfigurationLister(), controller.GetEventRecorder(ctx), r)
	}))
}

// this type is simply a convenient alias, see withTraffic funtion below for its purpose
type pair struct {
	name  string
	value int64
}

type roTrafficFunc func(...v1.TrafficTarget) RouteOption
type psoTrafficFunc func(...v1.TrafficTarget) PolicyStateOption

// withTraffic extracts some verbiage from the table tests to make them more concise
func withTraffic(rf roTrafficFunc, nameValuePairs ...pair) RouteOption {
	tt := makeTrafficTargetList(nameValuePairs...)
	return rf(tt...)
}

// withPSTraffic is similar to withTraffic, but it serves PolicyStates
func withPSTraffic(pf psoTrafficFunc, nameValuePairs ...pair) PolicyStateOption {
	tt := makeTrafficTargetList(nameValuePairs...)
	return pf(tt...)
}

func makeTrafficTargetList(nameValuePairs ...pair) []v1.TrafficTarget {
	tt := make([]v1.TrafficTarget, len(nameValuePairs))
	for i, pair := range nameValuePairs {
		tt[i] = v1.TrafficTarget{
			RevisionName:   pair.name,
			LatestRevision: ptr.Bool(false),
			Percent:        ptr.Int64(pair.value),
		}
	}
	if len(nameValuePairs) == 1 {
		tt[0].LatestRevision = ptr.Bool(true)
	}
	return tt
}

// assertEventQueued returns a function that is used for PostConditions checking
// its main purpose is to test whether events are properly enqueued
func assertEventQueued(key string, want time.Duration) func(*testing.T, *TableRow) {
	return func(t *testing.T, r *TableRow) {
		got, ok := r.OtherTestData[key]
		if !ok {
			t.Errorf("expected event to be enqueued, but none found")
			return
		}
		if diff := cmp.Diff(fmt.Sprintf("%v", want), got); diff != "" {
			t.Errorf("event is not correctly enqueued (-want, +got) %v", diff)
		}
	}
}

// assertNoEventQueued is used for tests where events should NOT be enqueued
func assertNoEventQueued(key string) func(*testing.T, *TableRow) {
	return func(t *testing.T, r *TableRow) {
		got, ok := r.OtherTestData[key]
		if ok {
			t.Errorf("no events should be enqueued, but got %v", got)
		}
	}
}
