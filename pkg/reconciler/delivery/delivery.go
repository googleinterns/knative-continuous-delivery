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
	"errors"
	"fmt"
	"math"
	"time"

	clientset "knative.dev/serving/pkg/client/clientset/versioned"
	configurationreconciler "knative.dev/serving/pkg/client/injection/reconciler/serving/v1/configuration"

	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	listers "knative.dev/serving/pkg/client/listers/serving/v1"

	"k8s.io/apimachinery/pkg/labels"
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
	// RevisionGenerationKey is the label key for querying how "old" a Revision is
	RevisionGenerationKey = "serving.knative.dev/configurationGeneration"
	// WaitForReady makes sure that when a newly created Revision becomes ready, it triggers the reconciler
	WaitForReady = 5 * time.Second
	// TimeFormat specifies the format used by time.Parse and time.Format
	TimeFormat = time.RFC3339
)

// Reconciler implements controller.Reconciler
type Reconciler struct {
	client         clientset.Interface
	routeLister    listers.RouteLister
	revisionLister listers.RevisionLister
	followup       enqueueFunc
	// TODO: use the k8s clock interface for time provider
}

// private aliases for the types in Reconciler
type enqueueFunc func(*v1.Configuration, time.Duration)

// Check that our Reconciler implements ksvcreconciler.Interface
var _ configurationreconciler.Interface = (*Reconciler)(nil)

var (
	// we use a global variable for now because we assume for simplicity that all Configurations
	// use the same policy; in the future, we might want to associate a policy to each Configuration
	policy Policy = Policy{
		Mode:             "time",
		Stages:           []Stage{{0, nil}, {1, nil}, {10, nil}, {20, nil}, {90, nil}},
		DefaultThreshold: 60,
	}
)

// ReconcileKind is triggered to enforce the rollout policy
func (c *Reconciler) ReconcileKind(ctx context.Context, cfg *v1.Configuration) pkgreconciler.Event {
	// ignore changes triggered by continuous-delivery itself
	if shouldSkipConfig(cfg) {
		return nil
	}

	// wait for latest created Revision to be ready
	if !configReady(cfg) {
		c.followup(cfg, WaitForReady)
		return nil
	}

	// TODO: return delay from updateRoute and pull out the followup
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

// updateRoute assigns traffic to active Revisions, applies new Route, and enqueues future events
func (c *Reconciler) updateRoute(ctx context.Context, cfg *v1.Configuration) error {
	logger := logging.FromContext(ctx)

	r, err := c.routeLister.Routes(cfg.Namespace).Get(cfg.Name)
	if err != nil {
		return err
	}
	route := r.DeepCopy()
	latestReady := cfg.Status.LatestReadyRevisionName
	// TODO: do not list ALL revisions in namespace; list only those for the cfg
	// selector := labels.SelectorFromSet(labels.Set{serving.ConfigurationLabelKey: config.Name})
	revisionList, err := c.revisionLister.Revisions(cfg.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	revisionMap := make(map[string]*v1.Revision) // mapping Revision names to objects
	for _, rev := range revisionList {
		revisionMap[rev.Name] = rev
	}

	route, err = modifyRouteSpec(route, revisionMap, latestReady, &policy)
	if err != nil {
		return err
	}

	logger.Info("Applying Route object")
	_, err = c.client.ServingV1().Routes(cfg.Namespace).Update(route)
	if err != nil {
		return err
	}

	// when we have latestRevision = true, we know that we don't need to queue future events
	if *route.Spec.Traffic[0].LatestRevision {
		logger.Info("Routing state has stabilized!")
		return nil
	}

	delay, err := timeTillNextEvent(route, revisionMap, &policy)
	if err != nil {
		return err
	}
	if delay == 0 {
		return nil
	}
	logger.Infof("Enqueueing event after %v", delay)
	c.followup(cfg, delay)

	return nil
}

// min is a helper that returns the minimum of an arbitrary number of integers
func min(items ...int) int {
	if len(items) == 0 {
		panic(errors.New("min must have at least one argument"))
	}
	result := items[0]
	for _, i := range items[1:] {
		if i < result {
			result = i
		}
	}
	return result
}

// timeTillNextEvent calculates the time to wait before enqueueing the next event
func timeTillNextEvent(route *v1.Route, r map[string]*v1.Revision, policy *Policy) (time.Duration, error) {
	result := math.MaxInt32
	// compute how long each Revision would like to wait, and then take the minimum
	for _, t := range route.Spec.Traffic {
		revision, ok := r[t.RevisionName]
		if !ok {
			return 0, fmt.Errorf("cannot find Revision %s in indexer", t.RevisionName)
		}
		if revision.Labels[RevisionGenerationKey] == "1" {
			continue
		}
		timeElapsed := time.Since(revision.CreationTimestamp.Time)
		result = min(metricTillNextStage(policy, timeElapsed), result)
	}
	return time.Duration(result) * time.Second, nil
}
