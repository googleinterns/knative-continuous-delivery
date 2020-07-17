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
	"testing"
	"time"
	"fmt"


	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	// clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclient "knative.dev/serving/pkg/client/injection/client"
	configurationreconciler "knative.dev/serving/pkg/client/injection/reconciler/serving/v1/configuration"

	. "github.com/googleinterns/knative-continuous-delivery/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
	. "knative.dev/serving/pkg/testing/v1"

	"github.com/google/go-cmp/cmp"
)

func TestReconcile(t *testing.T) {
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
	}, {
		Name: "does nothing when latest created is not ready",
		Key:  "default/test2",
		Objects: []runtime.Object{
			Route("default", "test2", WithConfigTarget("test2"), WithRouteGeneration(1)),
			Configuration("default", "test2", WithLatestCreated("rev-1")),
		},
	}}
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, tr *TableRow) controller.Reconciler {
		tr.OtherTestData = make(map[string]interface{})
		r := &Reconciler{
			client:      servingclient.Get(ctx),
			routeLister: listers.GetRouteLister(),
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

// Configuration creates a configuration with ConfigOptions
func Configuration(namespace, name string, co ...ConfigOption) *v1.Configuration {
	c := &v1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	for _, opt := range co {
		opt(c)
	}
	c.SetDefaults(context.Background())
	return c
}

// this type is simply a convenient alias, see withTraffic funtion below for its purpose
type pair struct {
	name  string
	value int64
}

// withTraffic extracts some verbiage from the table tests to make them more concise
func withTraffic(field string, nameValuePairs ...pair) RouteOption {
	tt := make([]v1.TrafficTarget, len(nameValuePairs))
	for i, pair := range nameValuePairs {
		tt[i] = v1.TrafficTarget{
			RevisionName: pair.name,
			LatestRevision: ptr.Bool(false),
			Percent: ptr.Int64(pair.value),
		}
	}
	if len(nameValuePairs) == 1 {
		tt[0].LatestRevision = ptr.Bool(true)
	}

	switch field {
	case "spec":
		return WithSpecTraffic(tt...)
	case "status":
		return WithStatusTraffic(tt...)
	default:
		panic(fmt.Errorf("withTraffic field can only be 'spec' or 'status'"))
	}
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
		if diff := cmp.Diff(got, fmt.Sprintf("%v", want)); diff != "" {
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
