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
	"strings"
	"time"

	deliveryclientset "github.com/googleinterns/knative-continuous-delivery/pkg/client/clientset/versioned"
	clientset "knative.dev/serving/pkg/client/clientset/versioned"
	configurationreconciler "knative.dev/serving/pkg/client/injection/reconciler/serving/v1/configuration"

	"github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery"
	v1alpha1 "github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery/v1alpha1"
	pslisters "github.com/googleinterns/knative-continuous-delivery/pkg/client/listers/delivery/v1alpha1"
	"github.com/googleinterns/knative-continuous-delivery/pkg/reconciler/delivery/resources"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/serving/pkg/apis/serving"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	listers "knative.dev/serving/pkg/client/listers/serving/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/clock"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Delivery"
	// KCDNamespace is the namespace of this project (KCD = Knative Continuous Delivery)
	KCDNamespace = "knative-serving"
	// KCDName is the name of this project
	KCDName = "knative-continuous-delivery"
	// WaitForReady makes sure that when a newly created Revision becomes ready, it triggers the reconciler
	WaitForReady = 5 * time.Second
	// TimeFormat specifies the format used by time.Parse and time.Format
	TimeFormat = time.RFC3339
)

// Reconciler implements controller.Reconciler
type Reconciler struct {
	client            clientset.Interface
	psclient          deliveryclientset.Interface
	routeLister       listers.RouteLister
	revisionLister    listers.RevisionLister
	policyLister      pslisters.PolicyLister
	policystateLister pslisters.PolicyStateLister
	followup          enqueueFunc
	clock             clock.Clock
}

// private aliases for the types in Reconciler
type enqueueFunc func(*v1.Configuration, time.Duration)

// Check that our Reconciler implements ksvcreconciler.Interface
var _ configurationreconciler.Interface = (*Reconciler)(nil)

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

	// ignore if no policy is specified
	if _, ok := cfg.Annotations[delivery.PolicyNameKey]; !ok {
		logging.FromContext(ctx).Infof("No policy specified for %v, skipping", cfg.Namespace+"/"+cfg.Name)
		return nil
	}

	// check for existing NextUpdateTimestamp to prevent event leaks in case of KCD controller restart, etc.
	if ps, err := c.fetchPolicyState(cfg); err != nil {
		return err
	} else if ps.Status.NextUpdateTimestamp != nil && ps.Status.NextUpdateTimestamp.Time.After(c.clock.Now()) {
		c.followup(cfg, ps.Status.NextUpdateTimestamp.Time.Sub(c.clock.Now()))
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

// fetchRoute queries the indexer to retrieve a Route object
func (c *Reconciler) fetchRoute(ctx context.Context, cfg *v1.Configuration) (*v1.Route, error) {
	r, err := c.routeLister.Routes(cfg.Namespace).Get(cfg.Name)
	if err != nil {
		logging.FromContext(ctx).Info("Failed to find Route object, potentially due to namespace/name mismatch between Configuration and Route")
		return nil, err
	}
	return r.DeepCopy(), nil
}

// fetchRevisions queries the indexer to find the Revisions and return a map from Revision names to objects
func (c *Reconciler) fetchRevisions(cfg *v1.Configuration) (map[string]*v1.Revision, error) {
	selector := labels.SelectorFromSet(labels.Set{serving.ConfigurationLabelKey: cfg.Name})
	revisionList, err := c.revisionLister.Revisions(cfg.Namespace).List(selector)
	if err != nil {
		return nil, err
	}
	revisionMap := make(map[string]*v1.Revision) // mapping Revision names to objects
	for _, rev := range revisionList {
		revisionMap[rev.Name] = rev
	}
	return revisionMap, nil
}

// fetchPolicy queries the indexer to retrieve a Policy object and return its translated version
// if annotations don't specify a Policy or if the specified Policy cannot be found, an error is returned
func (c *Reconciler) fetchPolicy(cfg *v1.Configuration) (*Policy, error) {
	policyNamespace, policyName := identifyPolicy(cfg)
	p, err := c.policyLister.Policies(policyNamespace).Get(policyName)
	if err != nil {
		return nil, err
	}
	return translatePolicy(p.DeepCopy()), nil
}

// fetchPolicyState queries the indexer to retrieve a PolicyState object whose namespace/name match with cfg
// it creates one if a PolicyState object doesn't already exist for the given namespace/name
func (c *Reconciler) fetchPolicyState(cfg *v1.Configuration) (*v1alpha1.PolicyState, error) {
	ps, err := c.policystateLister.PolicyStates(cfg.Namespace).Get(cfg.Name)
	if apierrs.IsNotFound(err) {
		ps = resources.MakePolicyState(cfg)
		ps, err = c.psclient.DeliveryV1alpha1().PolicyStates(cfg.Namespace).Create(ps)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return ps.DeepCopy(), nil
}

// applyChanges applies the newly create Route and PolicyState objects and wraps up the reconciliation
func (c *Reconciler) applyChanges(ctx context.Context, cfg *v1.Configuration, route *v1.Route, ps *v1alpha1.PolicyState, revisionMap map[string]*v1.Revision, p *Policy) error {
	logger := logging.FromContext(ctx)

	// first compute whether or not we need to enqueue events for future rollout stages
	if *route.Spec.Traffic[0].LatestRevision {
		logger.Info("Routing state has stabilized!")
		ps.Status.NextUpdateTimestamp = nil
	} else {
		delay, err := timeTillNextEvent(route, revisionMap, p, c.clock)
		if err != nil {
			return err
		}
		if delay != 0 {
			logger.Infof("Enqueueing event after %v", delay)
			c.followup(cfg, delay)
		}
		ps.Status.NextUpdateTimestamp = &metav1.Time{
			c.clock.Now().Add(delay),
		}
	}

	logger.Info("Applying PolicyState object")
	_, err := c.psclient.DeliveryV1alpha1().PolicyStates(cfg.Namespace).Update(ps)
	if err != nil {
		return err
	}
	logger.Info("Applying Route object")
	_, err = c.client.ServingV1().Routes(cfg.Namespace).Update(route)
	if err != nil {
		return err
	}
	return nil
}

// updateRoute assigns traffic to active Revisions, applies new Route, and enqueues future events
func (c *Reconciler) updateRoute(ctx context.Context, cfg *v1.Configuration) error {
	route, err := c.fetchRoute(ctx, cfg)
	if err != nil {
		return err
	}

	policy, err := c.fetchPolicy(cfg)
	if err != nil {
		return err
	}

	revisionMap, err := c.fetchRevisions(cfg)
	if err != nil {
		return err
	}

	ps, err := c.fetchPolicyState(cfg)
	if err != nil {
		return err
	}

	route, err = modifyRouteSpec(route, revisionMap, cfg.Status.LatestReadyRevisionName, policy, c.clock)
	if err != nil {
		return err
	}
	ps.Spec = v1alpha1.PolicyStateSpec{
		Traffic: route.Spec.Traffic,
	}

	return c.applyChanges(ctx, cfg, route, ps, revisionMap, policy)
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
func timeTillNextEvent(route *v1.Route, r map[string]*v1.Revision, policy *Policy, clock clock.Clock) (time.Duration, error) {
	result := math.MaxInt32
	oldest := oldestRevision(r)
	// compute how long each Revision would like to wait, and then take the minimum
	for _, t := range route.Spec.Traffic {
		revision, ok := r[t.RevisionName]
		if !ok {
			return 0, fmt.Errorf("cannot find Revision %s in indexer", t.RevisionName)
		}
		if revision == oldest {
			continue
		}
		timeElapsed := clock.Since(revision.CreationTimestamp.Time)
		result = min(metricTillNextStage(policy, timeElapsed), result)
	}
	return time.Duration(result) * time.Second, nil
}

// oldestRevision returns the oldest revision (as determined by creation timestamp)
func oldestRevision(r map[string]*v1.Revision) *v1.Revision {
	var result *v1.Revision
	earliest := time.Unix(1<<63-62135596801, 999999999) // max possible time representable using time.Time
	for _, rev := range r {
		if rev.CreationTimestamp.Time.Before(earliest) {
			earliest = rev.CreationTimestamp.Time
			result = rev
		}
	}
	return result
}

// identifyPolicy returns a Policy's namespace and name given a configuration and proper annotations
func identifyPolicy(cfg *v1.Configuration) (policyNamespace, policyName string) {
	// there's no need for defensive map query check, because it would have been taken care of in ReconcileKind
	policyNamespace = cfg.Namespace
	policyName = cfg.Annotations[delivery.PolicyNameKey]
	if s := strings.SplitN(policyName, "/", 2); len(s) > 1 {
		policyNamespace = s[0]
		policyName = s[1]
	}
	return
}
