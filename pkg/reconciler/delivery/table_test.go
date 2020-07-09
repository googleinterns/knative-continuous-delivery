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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
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
)

func TestReconcile(t *testing.T) {
	now := time.Now()
	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "does nothing when latest created is not ready",
		Key:  "default/test",
		Objects: []runtime.Object{
			Route("default", "test", WithConfigTarget("test"), WithRouteGeneration(1)),
			Configuration("default", "test", WithLatestCreated("rev-1")),
		},
	}, {
		Name: "sets the route to 100% if the configuration is ready",
		Key:  "default/test",
		Objects: []runtime.Object{
			Route("default", "test", WithConfigTarget("test"), WithRouteGeneration(1)),
			Configuration("default", "test", WithLatestCreated("rev-1"), WithLatestReady("rev-1")),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{
			{Object: Route("default", "test",
				WithConfigTarget("test"),
				WithRouteGeneration(1),
				// whenever Route is changed, Annotation will receive a new timestamp
				WithRouteAnnotation(map[string]string{AnnotationKey: now.Format(TimeFormat)}),
				WithSpecTraffic(v1.TrafficTarget{ConfigurationName: "test", Percent: ptr.Int64(100)}),
			)},
		},
	}}
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			client:      servingclient.Get(ctx),
			routeLister: listers.GetRouteLister(),
			// followup:    
			timeProvider: func() time.Time { return now },
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
