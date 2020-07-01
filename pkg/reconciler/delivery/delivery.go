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

	clientset "knative.dev/serving/pkg/client/clientset/versioned"
	configurationreconciler "knative.dev/serving/pkg/client/injection/reconciler/serving/v1/configuration"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
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
	// ignore changes triggered by continuous-delivery itself
	if shouldSkipConfig(cfg) {
		return nil
	}

	// wait for latest created Revision to be ready
	if !configReady(cfg) {
		return nil
	}

	return c.updateRoute(ctx, cfg)
}

// shouldSkipConfig determines if we should do a no-op because the reconciler is triggered
// by changes in KCD itself
func shouldSkipConfig(cfg *v1.Configuration) bool {
	return cfg.Namespace == KCDNamespace && cfg.Name == KCDName
}

// configReady determines if the given Configuration's latest created Revision is ready
func configReady(cfg *v1.Configuration) bool {
	latestReady := cfg.Status.LatestReadyRevisionName
	latestCreated := cfg.Status.LatestCreatedRevisionName
	return latestReady == latestCreated
}

// updateRoute figures out if the Route object needs any update and updates it as needed
func (c *Reconciler) updateRoute(ctx context.Context, cfg *v1.Configuration) error {
	logger := logging.FromContext(ctx)

	// find the Route object
	r, err := c.routeLister.Routes(cfg.Namespace).Get(cfg.Name)
	if err != nil {
		return err
	}
	route := r.DeepCopy()
	latestReady := cfg.Status.LatestReadyRevisionName

	// update the Route if necessary
	if routeStatusUptodate(route, latestReady) {
		return nil
	}
	route, err = modifyRouteSpec(route, latestReady)
	if err != nil {
		return err
	}
	logger.Info("Applying updated Route object")
	_, err = c.client.ServingV1().Routes(cfg.Namespace).Update(route)

	return err
}

// routeStatusUptodate determines if the current Route status already matches our desired state
func routeStatusUptodate(route *v1.Route, newRevName string) bool {
	for idx := range route.Status.Traffic {
		if route.Status.Traffic[idx].RevisionName == newRevName {
			return true
		}
	}
	return false
}

// modifyRouteSpec is a toy function that is designed specifically for the proof-of-concept
// it modifies the Route spec field to accommodate the new Revision, if necessary
func modifyRouteSpec(route *v1.Route, newRevName string) (*v1.Route, error) {
	// if there is currently zero traffic targets, then set the Configuration's
	// latest ready Revision as the default traffic target
	// if there is currently one traffic target, then split 50% off that target and
	// direct it to the new Revision
	// if there are 2 or more traffic targets, return an error (unexpected use case)
	if len(route.Status.Traffic) == 0 {
		route.Spec.Traffic = []v1.TrafficTarget{
			{
				ConfigurationName: route.Name, // assume namespace/name matches for Route & Config
				LatestRevision: ptr.Bool(true),
				Percent: ptr.Int64(100),
			},
		}
	} else if len(route.Status.Traffic) == 1 {
		route.Spec.Traffic = []v1.TrafficTarget{
			{
				RevisionName: newRevName,
				LatestRevision: ptr.Bool(false),
				Percent: ptr.Int64(newPercent),
			},
			{
				RevisionName: route.Status.Traffic[0].RevisionName,
				LatestRevision: ptr.Bool(false),
				Percent: ptr.Int64(oldPercent),
			},
		}
	} else {
		return nil, fmt.Errorf("Unsupported use case: current implementation only supports 2 Revisions at once")
	}
	return route, nil
}
