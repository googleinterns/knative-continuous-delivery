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
	v1alpha1 "github.com/googleinterns/knative-continuous-delivery/pkg/apis/delivery/v1alpha1"
	fakedeliveryclientset "github.com/googleinterns/knative-continuous-delivery/pkg/client/clientset/versioned/fake"
	deliverylisters "github.com/googleinterns/knative-continuous-delivery/pkg/client/listers/delivery/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/reconciler/testing"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	fakeservingclientset "knative.dev/serving/pkg/client/clientset/versioned/fake"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"
)

var clientSetSchemes = []func(*runtime.Scheme) error{
	fakekubeclientset.AddToScheme,
	fakeservingclientset.AddToScheme,
	fakedeliveryclientset.AddToScheme,
}

// Listers holds sorters
type Listers struct {
	sorter testing.ObjectSorter
}

// NewListers creates a Listers object with objs
func NewListers(objs []runtime.Object) Listers {
	scheme := NewScheme()

	ls := Listers{
		sorter: testing.NewObjectSorter(scheme),
	}

	ls.sorter.AddObjects(objs...)

	return ls
}

// NewScheme creates a scheme with a set client set schemes
func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	for _, addTo := range clientSetSchemes {
		addTo(scheme)
	}
	return scheme
}

// NewScheme returns a new Scheme
func (*Listers) NewScheme() *runtime.Scheme {
	return NewScheme()
}

// IndexerFor returns the indexer for the given object.
func (l *Listers) IndexerFor(obj runtime.Object) cache.Indexer {
	return l.sorter.IndexerForObjectType(obj)
}

// GetKubeObjects returns the kube objects
func (l *Listers) GetKubeObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakekubeclientset.AddToScheme)
}

// GetServingObjects returns the serving objects
func (l *Listers) GetServingObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakeservingclientset.AddToScheme)
}

// GetDeliveryObjects returns the delivery objects
func (l *Listers) GetDeliveryObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakedeliveryclientset.AddToScheme)
}

// GetRouteLister returns the RouteLister
func (l *Listers) GetRouteLister() servinglisters.RouteLister {
	return servinglisters.NewRouteLister(l.IndexerFor(&v1.Route{}))
}

// GetConfigurationLister returns the ConfigurationLister
func (l *Listers) GetConfigurationLister() servinglisters.ConfigurationLister {
	return servinglisters.NewConfigurationLister(l.IndexerFor(&v1.Configuration{}))
}

// GetRevisionLister returns the RevisionLister
func (l *Listers) GetRevisionLister() servinglisters.RevisionLister {
	return servinglisters.NewRevisionLister(l.IndexerFor(&v1.Revision{}))
}

// GetPolicyStateLister returns the PolicyStateLister
func (l *Listers) GetPolicyStateLister() deliverylisters.PolicyStateLister {
	return deliverylisters.NewPolicyStateLister(l.IndexerFor(&v1alpha1.PolicyState{}))
}
