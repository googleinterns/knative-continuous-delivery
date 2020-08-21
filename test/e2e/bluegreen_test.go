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

package e2e

import (
	"testing"

	servinge2e "knative.dev/serving/test/e2e"

	kcdtest "github.com/googleinterns/knative-continuous-delivery/test"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/serving/test"
	v1test "knative.dev/serving/test/v1"
)

// TODO: make sure testing environment is correct so this test doesn't fail
// TODO: add DeliveryClient and Policies to test traffic splitting
func Test100WithoutPolicy(t *testing.T) {
	t.Parallel()

	clients := servinge2e.Setup(t)

	var ksvcname = test.ObjectNameForTest(t)

	blue := test.ResourceNames{
		Service: ksvcname,
		Image:   "blue",
	}
	green := test.ResourceNames{
		Service: ksvcname,
		Image:   "green",
	}

	test.EnsureTearDown(t, clients, &blue)
	test.EnsureTearDown(t, clients, &green)

	t.Log("Creating a new Service")
	_, err := v1test.CreateServiceReady(t, clients, &blue)
	if err != nil {
		t.Fatalf("Failed to create initial Service: %v: %v", blue.Service, err)
	}

	t.Log("Configuring Service with new version")
	resources, err := kcdtest.UpdateServiceReady(t, clients, &green)
	if err != nil {
		t.Fatalf("Failed to configure Service with new version: %v: %v", green.Service, err)
	}

	url := resources.Route.Status.URL.URL()
	if _, err := pkgTest.WaitForEndpointState(
		clients.KubeClient,
		t.Logf,
		url,
		v1test.RetryingRouteInconsistency(pkgTest.MatchesAllOf(pkgTest.IsStatusOK, pkgTest.MatchesBody("<title>Knative Routing Demo</title>"))),
		"BlueGreenResponseHTML",
		test.ServingFlags.ResolvableDomain,
		test.AddRootCAtoTransport(t.Logf, clients, test.ServingFlags.Https),
	); err != nil {
		t.Fatalf("The endpoint %s for Route %s didn't serve the expected text %q: %v", url, green.Route, "<title>Knative Routing Demo</title>", err)
	}

	route := resources.Route
	if val := *route.Status.Traffic[0].Percent; val != 100 {
		t.Fatalf("Got route percentage=%v, want=%v ", val, 100)
	}
}
