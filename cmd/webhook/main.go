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

package main

import (
	"context"

	"github.com/googleinterns/knative-continuous-delivery/pkg/defaults"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	deliveryv1alpha1 "github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery/v1alpha1"
	defaultconfig "knative.dev/serving/pkg/apis/config"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	deliveryclient "github.com/googleinterns/knative-continuous-delivery/pkg/client/injection/client"
	policystate "github.com/googleinterns/knative-continuous-delivery/pkg/client/injection/informers/delivery/v1alpha1/policystate"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	servingv1.SchemeGroupVersion.WithKind("Route"):         &defaults.ContinuousDeploymentRoute{},
	deliveryv1alpha1.SchemeGroupVersion.WithKind("Policy"): &deliveryv1alpha1.Policy{},
}

func newDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.continuous-delivery.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(c context.Context) context.Context {
			inf := policystate.Get(ctx)
			clt := deliveryclient.Get(ctx)
			c = context.WithValue(c, policystate.Key{}, inf)
			c = context.WithValue(c, deliveryclient.Key{}, clt)
			return c
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func newValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	store.WatchConfigs(cmw)

	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.continuous-delivery.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "continuous-delivery-webhook",
		Port:        webhook.PortFromEnv(8443),
		SecretName:  "continuous-delivery-webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx,
		"continuous-delivery-webhook",
		certificates.NewController,
		newDefaultingAdmissionController,
		newValidationAdmissionController)
}
