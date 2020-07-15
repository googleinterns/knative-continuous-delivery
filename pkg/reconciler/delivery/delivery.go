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
	"time"

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
	// AnnotationKey is the string used in ObjectMeta.Annotations map for any Route object
	AnnotationKey = "KCDLastRouteUpdate"
	// TimeFormat specifies the format used by time.Parse and time.Format
	TimeFormat = time.RFC3339
)

// Reconciler implements controller.Reconciler
type Reconciler struct {
	client        clientset.Interface
	routeLister   listers.RouteLister
	followup      enqueueFunc
	timeProvider  timeSnapshotFunc
}

// private aliases for the types in Reconciler
type enqueueFunc func(*v1.Configuration, time.Duration)
type timeSnapshotFunc func() time.Time

// Check that our Reconciler implements ksvcreconciler.Interface
var _ configurationreconciler.Interface = (*Reconciler)(nil)

var (
	// we use a global variable for now because we assume for simplicity that all Configurations
	// use the same policy; in the future, we might want to associate a policy to each Configuration
	policy Policy = Policy{
		Mode: "time",
		Percents: []Stage{{0, nil}, {10, nil}, {50, nil}, {90, nil}},
		DefaultThreshold: 20,
	}
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
	return latestReady == latestCreated && latestReady != ""
}

// updateRoute figures out if the Route object needs any update and updates it as needed
func (c *Reconciler) updateRoute(ctx context.Context, cfg *v1.Configuration) error {
	logger := logging.FromContext(ctx)

	r, err := c.routeLister.Routes(cfg.Namespace).Get(cfg.Name)
	if err != nil {
		return err
	}
	route := r.DeepCopy()
	latestReady := cfg.Status.LatestReadyRevisionName

	if isRouteStatusUpToDate(route, latestReady, &policy) {
		return nil
	}
	route, err = modifyRouteSpec(route, latestReady, &policy)
	if err != nil {
		return err
	}

	logger.Info("Applying updated Route object")

	// record the timestamp for the current udpate to the Route object before actually pushing it
	// this is used later when determining if Route status is up to date
	if route.Annotations == nil {
		route.Annotations = make(map[string]string)
	}
	route.Annotations[AnnotationKey] = c.timeProvider().Format(TimeFormat)
	_, err = c.client.ServingV1().Routes(cfg.Namespace).Update(route)
	if err != nil {
		return err
	}

	// when we have latestRevision = true, we know that we don't need to queue future events
	if *route.Spec.Traffic[0].LatestRevision {
		logger.Info("Progressive rollout completed!")
		return nil
	}
	t, e := getThreshold(&policy, int(*route.Spec.Traffic[1].Percent))
	if e != nil {
		return e
	}
	logger.Infof("Queueing event for %v seconds later", t)
	c.followup(cfg, time.Duration(t) * time.Second)

	return nil
}

// isRouteStatusUpToDate determines if the current Route status already matches our desired state
func isRouteStatusUpToDate(route *v1.Route, newRevName string, policy *Policy) bool {
	// the Route status is up to date if:
	// 1. the new Revision is listed in the status traffic targets, AND
	// 2. the Route time stamp hasn't expired
	// OR if:
	// 3. the new Revision is listed in the status traffic targets, AND
	// 4. the new Revision already reached 100%
	nameListed := false
	for _, t := range route.Status.Traffic {
		if t.RevisionName == newRevName {
			nameListed = true
			break
		}
	}
	if !nameListed {
		return false
	}
	if len(route.Status.Traffic) == 1 || *route.Status.Traffic[1].Percent == 100 {
		return true
	}
	// by design, accessing route.Annotations[AnnotationKey] should not cause error
	previousTime, err := time.Parse(TimeFormat, route.Annotations[AnnotationKey])
	if err != nil {
		// we shouldn't be able to reach this because timestamp is always formatted using TimeFormat
		panic(fmt.Sprintf("failed to parse timestamp for %v", AnnotationKey))
	}
	return !isTimestampExpired(previousTime, policy, int(*route.Status.Traffic[1].Percent))
}

// isTimestampExpired determines if enough time has elapsed since the last Route update
// ltt = Last Transition Time, i.e. the timestamp for the last Route update
func isTimestampExpired(ltt time.Time, policy *Policy, cp int) bool {
	t, e := getThreshold(policy, cp)
	// we can ignore error handling here, because returning true will cause a Route update
	// modifyRouteSpec will discover the exact same error, and it can report that error more conveniently
	if e != nil {
		return true
	}
	return !time.Now().Before(ltt.Add(time.Duration(t) * time.Second))
}

// modifyRouteSpec is a toy function that is designed specifically for the proof-of-concept
// it modifies the Route spec field to accommodate the new Revision, if necessary
func modifyRouteSpec(route *v1.Route, newRevName string, policy *Policy) (*v1.Route, error) {
	// if there is currently zero traffic targets, then set the Configuration's
	// latest ready Revision as the default traffic target
	// if there is currently one traffic target, then split a certain % off that target and
	// direct it to the new Revision
	// if there are 2 traffic targets, update the percentage split, or report error if the 
	// new Revision name doesn't match with either target
	// Note: when there are > 1 traffic targets, it is assumed that they are ordered from oldest to newest
	newPercent := 100
	var err error

	if len(route.Status.Traffic) == 1 {
		if route.Status.Traffic[0].RevisionName == newRevName {
			return route, nil
		}
		newPercent, err = computeNewPercent(policy, 0)
		if err != nil {
			return route, err
		}
	} else if len(route.Status.Traffic) == 2 {
		if route.Status.Traffic[0].RevisionName != newRevName && route.Status.Traffic[1].RevisionName != newRevName {
			return nil, fmt.Errorf("unsupported use case: current implementation only supports 2 Revisions at once")
		}
		newPercent, err = computeNewPercent(policy, int(*route.Status.Traffic[1].Percent))
		if err != nil {
			return route, err
		}
	}

	if newPercent == 100 {
		route.Spec.Traffic = []v1.TrafficTarget{{
				ConfigurationName: route.Name, // assume namespace/name matches for Route & Config
				LatestRevision: ptr.Bool(true),
				Percent: ptr.Int64(100),
			}}
		return route, nil
	}
	
	route.Spec.Traffic = []v1.TrafficTarget{{
			RevisionName: route.Status.Traffic[0].RevisionName,
			LatestRevision: ptr.Bool(false),
			Percent: ptr.Int64(int64(100 - newPercent)),
		},{
			RevisionName: newRevName,
			LatestRevision: ptr.Bool(false),
			Percent: ptr.Int64(int64(newPercent)),
		}}

	return route, nil
}
