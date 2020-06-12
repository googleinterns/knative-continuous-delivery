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

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	servingclient "knative.dev/serving/pkg/client/injection/client"
	configurationinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/configuration"
	revisioninformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/revision"
	routeinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/route"
	configurationreconciler "knative.dev/serving/pkg/client/injection/reconciler/serving/v1/configuration"

	"k8s.io/client-go/tools/cache"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	servingreconciler "knative.dev/serving/pkg/reconciler"
)

const (
	controllerAgentName = "delivery-controller"
)

// NewController returns a controller to be called by generated knative pkg main.
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	ctx = servingreconciler.AnnotateLoggerWithName(ctx, controllerAgentName)
	logger := logging.FromContext(ctx)
	routeInformer := routeinformer.Get(ctx)
	configurationInformer := configurationinformer.Get(ctx)
	revisionInformer := revisioninformer.Get(ctx)

	c := &Reconciler{
		client:              servingclient.Get(ctx),
		configurationLister: configurationInformer.Lister(),
		revisionLister:      revisionInformer.Lister(),
		routeLister:         routeInformer.Lister(),
	}
	impl := configurationreconciler.NewImpl(ctx, c)

	// set up event handlers to put things in the work queue of impl
	logger.Info("Setting up event handlers")

	handleControllerOf := cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1.Kind("Configuration")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	}

	revisionInformer.Informer().AddEventHandler(handleControllerOf)

	return impl
}
