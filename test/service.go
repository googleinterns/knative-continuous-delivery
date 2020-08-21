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

package test

import (
	"fmt"

	pkgTest "knative.dev/pkg/test"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	serviceresourcenames "knative.dev/serving/pkg/reconciler/service/resources/names"
	rtesting "knative.dev/serving/pkg/testing/v1"
	"knative.dev/serving/test"
	v1test "knative.dev/serving/test/v1"
)

// UpdateServiceReady updates a Service in 'Ready' status.
func UpdateServiceReady(t pkgTest.T, clients *test.Clients, names *test.ResourceNames, fopt ...rtesting.ServiceOption) (*v1test.ResourceObjects, error) {
	if names.Image == "" {
		return nil, fmt.Errorf("expected non-empty Image name; got Image=%v", names.Image)
	}
	t.Log("Configuring Service", "service", names.Service)
	svc, err := UpdateService(t, clients, *names, fopt...)
	if err != nil {
		return nil, err
	}
	return getResourceObjects(t, clients, names, svc)
}

// UpdateService updates a service
func UpdateService(t pkgTest.T, clients *test.Clients, names test.ResourceNames, fopt ...rtesting.ServiceOption) (*v1.Service, error) {
	svc := v1test.Service(names, fopt...)
	return updateService(t, clients, svc)
}

func updateService(t pkgTest.T, clients *test.Clients, service *v1.Service) (*v1.Service, error) {
	test.AddTestAnnotation(t, service.ObjectMeta)
	v1test.LogResourceObject(t, v1test.ResourceObjects{Service: service})
	return clients.ServingClient.Services.Update(service)
}

func getResourceObjects(t pkgTest.T, clients *test.Clients, names *test.ResourceNames, svc *v1.Service) (*v1test.ResourceObjects, error) {
	// Populate Route and Configuration Objects with name
	names.Route = serviceresourcenames.Route(svc)
	names.Config = serviceresourcenames.Configuration(svc)

	// If the Service name was not specified, populate it
	if names.Service == "" {
		names.Service = svc.Name
	}

	t.Log("Waiting for Service to transition to Ready.", "service", names.Service)
	if err := v1test.WaitForServiceState(clients.ServingClient, names.Service, v1test.IsServiceReady, "ServiceIsReady"); err != nil {
		return nil, err
	}

	t.Log("Checking to ensure Service Status is populated for Ready service")
	err := validateCreatedServiceStatus(clients, names)
	if err != nil {
		return nil, err
	}

	t.Log("Getting latest objects Created by Service")
	resources, err := v1test.GetResourceObjects(clients, *names)
	if err == nil {
		t.Log("Successfully created Service", names.Service)
	}
	return resources, err
}

func validateCreatedServiceStatus(clients *test.Clients, names *test.ResourceNames) error {
	return v1test.CheckServiceState(clients.ServingClient, names.Service, func(s *v1.Service) (bool, error) {
		if s.Status.URL == nil || s.Status.URL.Host == "" {
			return false, fmt.Errorf("url is not present in Service status: %v", s)
		}
		names.URL = s.Status.URL.URL()
		if s.Status.LatestCreatedRevisionName == "" {
			return false, fmt.Errorf("lastCreatedRevision is not present in Service status: %v", s)
		}
		names.Revision = s.Status.LatestCreatedRevisionName
		if s.Status.LatestReadyRevisionName == "" {
			return false, fmt.Errorf("lastReadyRevision is not present in Service status: %v", s)
		}
		if s.Status.ObservedGeneration != 1 {
			return false, fmt.Errorf("observedGeneration is not 1 in Service status: %v", s)
		}
		return true, nil
	})
}
