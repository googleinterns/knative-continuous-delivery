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

package testing

import (
	"context"
	"testing"

	fakedeliveryclient "github.com/googleinterns/knative-continuous-delivery/pkg/client/injection/client/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	fakeservingclient "knative.dev/serving/pkg/client/injection/client/fake"
	servingtesting "knative.dev/serving/pkg/reconciler/testing/v1"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/reconciler"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	rtesting "knative.dev/pkg/reconciler/testing"
)

const (
	// maxEventBufferSize is the estimated max number of event notifications that
	// can be buffered during reconciliation.
	maxEventBufferSize = 10
)

// Ctor functions create a k8s controller with given params.
type Ctor func(context.Context, *Listers, configmap.Watcher, *rtesting.TableRow) controller.Reconciler

// MakeFactory creates a reconciler factory with fake clients and controller created by `ctor`.
func MakeFactory(ctor Ctor) rtesting.Factory {
	return func(t *testing.T, r *rtesting.TableRow) (
		controller.Reconciler, rtesting.ActionRecorderList, rtesting.EventList) {
		ls := NewListers(r.Objects)

		ctx := r.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
		logger := logtesting.TestLogger(t)
		ctx = logging.WithLogger(ctx, logger)

		ctx, kubeClient := fakekubeclient.With(ctx, ls.GetKubeObjects()...)
		ctx, client := fakeservingclient.With(ctx, ls.GetServingObjects()...)
		ctx, deliveryclient := fakedeliveryclient.With(ctx, ls.GetDeliveryObjects()...)
		ctx, dynamicClient := fakedynamicclient.With(ctx, ls.NewScheme(), servingtesting.ToUnstructured(t, ls.NewScheme(), r.Objects)...)
		ctx = context.WithValue(ctx, TrackerKey, &rtesting.FakeTracker{})

		// The dynamic client's support for patching is BS.  Implement it
		// here via PrependReactor (this can be overridden below by the
		// provided reactors).
		dynamicClient.PrependReactor("patch", "*",
			func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, nil
			})

		eventRecorder := record.NewFakeRecorder(maxEventBufferSize)
		ctx = controller.WithEventRecorder(ctx, eventRecorder)

		// This is needed for the tests that use generated names and
		// the object cannot be created beforehand.
		kubeClient.PrependReactor("create", "*",
			func(action ktesting.Action) (bool, runtime.Object, error) {
				ca := action.(ktesting.CreateAction)
				ls.IndexerFor(ca.GetObject()).Add(ca.GetObject())
				return false, nil, nil
			},
		)
		// This is needed by the Configuration controller tests, which
		// use GenerateName to produce Revisions.
		rtesting.PrependGenerateNameReactor(&client.Fake)

		// Set up our Controller from the fakes.
		c := ctor(ctx, &ls, configmap.NewStaticWatcher(), r)
		// Update the context with the stuff we decorated it with.
		r.Ctx = ctx

		// The Reconciler won't do any work until it becomes the leader.
		if la, ok := c.(reconciler.LeaderAware); ok {
			la.Promote(reconciler.UniversalBucket(), func(reconciler.Bucket, types.NamespacedName) {})
		}

		for _, reactor := range r.WithReactors {
			kubeClient.PrependReactor("*", "*", reactor)
			client.PrependReactor("*", "*", reactor)
			deliveryclient.PrependReactor("*", "*", reactor)
			dynamicClient.PrependReactor("*", "*", reactor)
		}

		// Validate all Create operations through the serving client.
		client.PrependReactor("create", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			// TODO(n3wscott): context.Background is the best we can do at the moment, but it should be set-able.
			return rtesting.ValidateCreates(context.Background(), action)
		})
		client.PrependReactor("update", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			// TODO(n3wscott): context.Background is the best we can do at the moment, but it should be set-able.
			return rtesting.ValidateUpdates(context.Background(), action)
		})

		actionRecorderList := rtesting.ActionRecorderList{dynamicClient, client, kubeClient, deliveryclient}
		eventList := rtesting.EventList{Recorder: eventRecorder}

		return c, actionRecorderList, eventList
	}
}

type key struct{}

// TrackerKey is used to looking a FakeTracker in a context.Context
var TrackerKey key = struct{}{}
