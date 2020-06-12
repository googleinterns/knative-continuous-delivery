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

	clientset "knative.dev/serving/pkg/client/clientset/versioned"
	configurationreconciler "knative.dev/serving/pkg/client/injection/reconciler/serving/v1/configuration"

	"knative.dev/pkg/logging"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	listers "knative.dev/serving/pkg/client/listers/serving/v1"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Delivery"
	// KCDNamespace is the namespace of this project (KCD = Knative Continuous Delivery)
	KCDNamespace = "knative-serving"
	// KCDName is the name of this project
	KCDName = "knative-continuous-delivery"
)

// Reconciler implements controller.Reconciler
type Reconciler struct {
	client clientset.Interface

	configurationLister listers.ConfigurationLister
	revisionLister      listers.RevisionLister 
	routeLister         listers.RouteLister 
}

// Check that our Reconciler implements ksvcreconciler.Interface
var _ configurationreconciler.Interface = (*Reconciler)(nil)
var (
	newPercent int64 = 50
	oldPercent int64 = 50
)

// ReconcileKind is a very simple proof-of-concept reconciliation method
// Assumes that there is one existing Revision and one new Revision
// when the new Revision arrives, split traffic between 2 Revisions
// according to newPercent/oldPercent
func (c *Reconciler) ReconcileKind(ctx context.Context, cfg *v1.Configuration) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// ignore changes triggered by continuous-delivery itself
	if cfg.Namespace == KCDNamespace && cfg.Name == KCDName {
		return nil
	}

	// retrieve information about latest ready and created Revisions
	// do nothing if latest created Revision is not ready
	latestReady := cfg.Status.LatestReadyRevisionName
	latestCreated := cfg.Status.LatestCreatedRevisionName
	if latestReady != latestCreated {
		return nil
	}

	// find the Route object
	r, err := c.routeLister.Routes(cfg.Namespace).Get(cfg.Name)
	if err != nil {
		return err
	}

	// determine if the current routing status includes the latest Revision
	// if yes, do nothing; if not, rewrite routing spec
	route := r.DeepCopy()
	for idx := range route.Status.Traffic {
		if route.Status.Traffic[idx].RevisionName == latestReady {
			return nil
		}
	}
	if len(route.Status.Traffic) >= 2 {
		logger.Info("Unsupported use case: current implementation only supports 2 Revisions at once")
		return nil
	}
	route = modifyRouteSpec(route, latestReady)

	// push the changed Route from Go memory to K8s
	route, err = c.client.ServingV1().Routes(cfg.Namespace).Update(route)
	return err
}

// modifyRouteSpec is a toy function that is designed specifically for the proof-of-concept
// it modifies the Route spec field to accommodate the new Revision, if necessary
func modifyRouteSpec(route *v1.Route, newRevName string) *v1.Route {
	// if there is currently zero traffic targets, then set the Configuration's
	// latest ready Revision as the default traffic target
	// if there is currently one traffic target, then split 50% off that target and
	// direct it to the new Revision
	if len(route.Status.Traffic) == 0 {
		route.Spec.Traffic = []v1.TrafficTarget{
			{
				ConfigurationName: route.Name, // assume namespace/name matches for Route & Config
				LatestRevision: boolPtr(true),
				Percent: int64Ptr(100),
			},
		}
	} else {
		route.Spec.Traffic = []v1.TrafficTarget{
			{
				RevisionName: newRevName,
				LatestRevision: boolPtr(false),
				Percent: int64Ptr(newPercent),
			},
			{
				RevisionName: route.Status.Traffic[0].RevisionName,
				LatestRevision: boolPtr(false),
				Percent: int64Ptr(oldPercent),
			},
		}
	}
	return route
}

func boolPtr(v bool) *bool {
	var b bool = v
	return &b
}

func int64Ptr(v int64) *int64 {
	var b int64 = v
	return &b
}
