// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package cf_test

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/cf/fakes"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp"
	"github.com/pivotal-cf/on-demand-service-broker/mockhttp/mockcfapi"
)

var _ = Describe("Client", func() {
	var server *mockhttp.Server
	var testLogger *log.Logger
	var logBuffer *gbytes.Buffer
	var authHeaderBuilder *fakes.FakeAuthHeaderBuilder

	const (
		cfAuthorizationHeader = "auth-header"
		serviceGUID           = "06df08f9-5a58-4d33-8097-32d0baf3ce1e"
	)

	BeforeEach(func() {
		authHeaderBuilder = new(fakes.FakeAuthHeaderBuilder)
		authHeaderBuilder.BuildReturns(cfAuthorizationHeader, nil)
		server = mockcfapi.New()
		logBuffer = gbytes.NewBuffer()
		testLogger = log.New(io.MultiWriter(logBuffer, GinkgoWriter), "my-app", log.LstdFlags)
	})

	AfterEach(func() {
		server.VerifyMocks()
	})

	Describe("GetServiceOfferingGUID", func() {
		It("returns the broker guid", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_1_response.json")),
				mockcfapi.ListServiceBrokersForPage(2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			var brokerGUID string
			brokerGUID, err = client.GetServiceOfferingGUID("service-broker-name-2", testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(brokerGUID).To(Equal("service-broker-guid-2-guid"))

		})

		It("returns an error if it fails to get service brokers", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetServiceOfferingGUID("service-broker-name-2", testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("returns an error if it fails to find a broker with the corect name", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBrokers().
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_1_response.json")),
				mockcfapi.ListServiceBrokersForPage(2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_brokers_page_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetServiceOfferingGUID("not-a-real-broker", testLogger)
			Expect(err).To(MatchError("Failed to find broker with name: not-a-real-broker"))
		})
	})

	Describe("DisableServiceAccess", func() {
		const offeringID = "D94A086D-203D-4966-A6F1-60A9E2300F72"

		It("disables all the plans across pages", func() {

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
				mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
				mockcfapi.DisablePlanAccess("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsCreated(),
				mockcfapi.DisablePlanAccess("2777ad05-8114-4169-8188-2ef5f39e0c6b").RespondsCreated(),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccess(offeringID, testLogger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error if it fails to get plans for service offering", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccess(offeringID, testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("returns an error if it fails to update the service plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
				mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
				mockcfapi.DisablePlanAccess("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			err = client.DisableServiceAccess(offeringID, testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})
	})

	Describe("Deregister", func() {
		const brokerGUID = "broker-guid"

		It("does not return an error", func() {
			server.VerifyAndMock(
				mockcfapi.DeregisterBroker(brokerGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsNoContent(),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeregisterBroker(brokerGUID, testLogger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error when the deregister fails", func() {
			server.VerifyAndMock(
				mockcfapi.DeregisterBroker(brokerGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("failed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			err = client.DeregisterBroker(brokerGUID, testLogger)
			Expect(err).To(MatchError(ContainSubstring("failed")))

		})
	})

	Describe("CountInstancesOfServiceOffering", func() {
		It("fetches instance counts per plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[string]int{
				"11789210-D743-4C65-9D38-C80B29F4D9C8": 1,
				"22789210-D743-4C65-9D38-C80B29F4D9C8": 2,
			}))
		})

		It("finds no instances when the service is not registered with cf", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_empty_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[string]int{}))
		})

		It("fails if getting a new token fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsUnauthorizedWith(`{"code": 1000,"description": "Invalid Auth Token","error_code": "CF-InvalidAuthToken"}`),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Invalid Auth Token")))
		})

		It("reuses tokens", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[string]int{
				"11789210-D743-4C65-9D38-C80B29F4D9C8": 1,
				"22789210-D743-4C65-9D38-C80B29F4D9C8": 2,
			}))

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[string]int{
				"11789210-D743-4C65-9D38-C80B29F4D9C8": 1,
				"22789210-D743-4C65-9D38-C80B29F4D9C8": 2,
			}))
		})

		It("fetches instance counts per plan, across service pages", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_1.json")),
				mockcfapi.ListServiceOfferingsForPage(2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_2.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[string]int{
				"11789210-D743-4C65-9D38-C80B29F4D9C8": 1,
				"22789210-D743-4C65-9D38-C80B29F4D9C8": 2,
			}))
		})

		It("fetches instance counts per plan, across plan pages", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
				mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)).To(Equal(map[string]int{
				"11789210-D743-4C65-9D38-C80B29F4D9C8": 1,
				"22789210-D743-4C65-9D38-C80B29F4D9C8": 2,
			}))
		})

		It("fails, if fetching auth token fails", func() {
			authHeaderBuilder.BuildReturns("", errors.New("niet goed"))

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.GetInstancesOfServiceOffering("some-offering", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching services fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching services fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching service plans fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})

		It("fails if fetching service instances fails", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsInternalServerErrorWith("niet goed"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.CountInstancesOfServiceOffering("D94A086D-203D-4966-A6F1-60A9E2300F72", testLogger)
			Expect(err).To(MatchError(ContainSubstring("niet goed")))
		})
	})

	Describe("CountInstancesOfPlan", func() {
		It("fetches instance counts for the plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)).To(Equal(2))
		})

		It("fail if service instance not found", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "does-not-exist", testLogger)
			Expect(err).To(MatchError(ContainSubstring("service plan does-not-exist not found")))
			Expect(count).To(BeZero())
		})

		It("fails when it can't retrieve services", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("no services for you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)
			Expect(err).To(MatchError(ContainSubstring("no services for you")))
			Expect(count).To(BeZero())
		})

		It("fails when it can't retrieve service plans", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).RespondsInternalServerErrorWith("no service plans for you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)
			Expect(err).To(MatchError(ContainSubstring("no service plans for you")))
			Expect(count).To(BeZero())
		})

		It("fails when it can't retrieve service instnaces for the plan", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").RespondsInternalServerErrorWith("no instances for you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			count, err := client.CountInstancesOfPlan("D94A086D-203D-4966-A6F1-60A9E2300F72", "22789210-D743-4C65-9D38-C80B29F4D9C8", testLogger)
			Expect(err).To(MatchError(ContainSubstring("no instances for you")))
			Expect(count).To(BeZero())
		})
	})

	Describe("GetInstance", func() {
		It("fetches the instance", func() {
			server.VerifyAndMock(
				mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
					RespondsOKWith(fixture("get_service_instance_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			instance, err := client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(instance.LastOperation.Type).To(Equal(cf.OperationType("create")))
			Expect(instance.LastOperation.State).To(Equal(cf.OperationState("succeeded")))
		})

		Context("when the service instance does not exist", func() {
			It("returns a not found error with the API error description", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsNotFoundWith(`{
						"code": 60004,
   					"description": "The service instance could not be found: 783f8645-1ded-4161-b457-73f59423f9eb",
   					"error_code": "CF-ServiceInstanceNotFound"
   				}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
				Expect(err).To(MatchError("The service instance could not be found: 783f8645-1ded-4161-b457-73f59423f9eb"))
			})
		})

		Context("when the request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsInternalServerErrorWith("er ma gerd"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(MatchError(ContainSubstring("er ma gerd")))
				Expect(err).NotTo(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
			})
		})

		Context("when the request is unauthorized", func() {
			Context("when the response is a CF API error response", func() {
				It("returns an unauthorized error with the API error description", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsUnauthorizedWith(`{
								"code": 10002,
								"description": "Authentication error",
								"error_code": "CF-NotAuthenticated"
							}`),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("Authentication error")))
					Expect(err).To(BeAssignableToTypeOf(cf.UnauthorizedError{}))
				})
			})

			Context("when the response is invalid json", func() {
				It("returns an unauthorized error with the response body", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsUnauthorizedWith("not valid json"),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("not valid json")))
					Expect(err).To(BeAssignableToTypeOf(cf.UnauthorizedError{}))
				})
			})
		})

		Context("when the request is forbidden", func() {
			Context("when the response is a CF API error response", func() {
				It("returns an unauthorized error with the API error description", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsForbiddenWith(`{
								"code": 10003,
								"description": "You are not authorized to perform the requested action",
								"error_code": "CF-NotAuthorized"
							}`),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("You are not authorized to perform the requested action")))
					Expect(err).To(BeAssignableToTypeOf(cf.ForbiddenError{}))
				})
			})

			Context("when the response is invalid json", func() {
				It("returns an unauthorized error with the response body", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsForbiddenWith("not valid json"),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(MatchError(ContainSubstring("not valid json")))
					Expect(err).To(BeAssignableToTypeOf(cf.ForbiddenError{}))
				})
			})
		})

		Context("when the request succeeds with an invalid response body", func() {
			It("returns an invalid response error", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
						RespondsOKWith("not valid json"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstance("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(MatchError(ContainSubstring("Invalid response body")))
				Expect(err).To(MatchError(ContainSubstring("invalid character 'o'")))
				Expect(err).To(BeAssignableToTypeOf(cf.InvalidResponseError{}))
			})
		})
	})

	Describe("GetInstanceState", func() {
		It("fetches the state of an instance", func() {
			server.VerifyAndMock(
				mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsOKWith(fixture("get_service_instance_response.json")),
				mockcfapi.GetServicePlan("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(fixture("get_service_plan_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			state, err := client.GetInstanceState("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
			Expect(state.PlanID).To(Equal("11789210-D743-4C65-9D38-C80B29F4D9C8"))
			Expect(state.OperationInProgress).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns that an operation is in progress when an operation is in progress", func() {
			server.VerifyAndMock(
				mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsOKWith(fixture("get_service_instance_operation_in_progress_response.json")),
				mockcfapi.GetServicePlan("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsOKWith(fixture("get_service_plan_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			state, err := client.GetInstanceState("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
			Expect(state.PlanID).To(Equal("11789210-D743-4C65-9D38-C80B29F4D9C8"))
			Expect(state.OperationInProgress).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the service instance request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsInternalServerErrorWith("er ma gerd"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstanceState("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(MatchError(ContainSubstring("er ma gerd")))
				Expect(err).NotTo(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
			})
		})

		Context("when the service instance does not exist", func() {
			Context("when the response is a CF API error response", func() {
				It("returns a not found error with the API error description", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsNotFoundWith(`{
              "code": 60004,
              "description": "The service instance could not be found: 783f8645-1ded-4161-b457-73f59423f9eb",
              "error_code": "CF-ServiceInstanceNotFound"
            }`),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetInstanceState("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
					Expect(err).To(MatchError("The service instance could not be found: 783f8645-1ded-4161-b457-73f59423f9eb"))
				})
			})

			Context("when the response is invalid json", func() {
				It("returns a not found error with the response body", func() {
					server.VerifyAndMock(
						mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").
							RespondsNotFoundWith("not valid json"),
					)

					client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
					Expect(err).NotTo(HaveOccurred())

					_, err = client.GetInstanceState("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
					Expect(err).To(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
					Expect(err).To(MatchError("not valid json"))
				})
			})
		})

		Context("when the service plan request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsOKWith(fixture("get_service_instance_response.json")),
					mockcfapi.GetServicePlan("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsInternalServerErrorWith("er ma gerd"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstanceState("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(MatchError(ContainSubstring("er ma gerd")))
				Expect(err).NotTo(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
			})
		})

		Context("when the service plan does not exist", func() {
			It("returns a not found error", func() {
				server.VerifyAndMock(
					mockcfapi.GetServiceInstance("783f8645-1ded-4161-b457-73f59423f9eb").RespondsOKWith(fixture("get_service_instance_response.json")),
					mockcfapi.GetServicePlan("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").RespondsNotFoundWith(`{
						"code": 110003,
   					"description": "The service plan could not be found: 783f8645-1ded-4161-b457-73f59423f9eb",
   					"error_code": "CF-ServicePlanNotFound"
   				}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstanceState("783f8645-1ded-4161-b457-73f59423f9eb", testLogger)
				Expect(err).To(BeAssignableToTypeOf(cf.ResourceNotFoundError{}))
				Expect(err).To(MatchError("The service plan could not be found: 783f8645-1ded-4161-b457-73f59423f9eb"))
			})
		})
	})

	Describe("GetInstancesOfServiceOffering", func() {
		It("returns a list of instance IDs", func() {
			offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

			server.VerifyAndMock(
				mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
				mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
				mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
				mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			instances, err := client.GetInstancesOfServiceOffering(offeringID, testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(ConsistOf("520f8566-b727-4c67-8be8-d9285645e936", "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", "2f759033-04a4-426b-bccd-01722036c152"))
		})

		Context("when the list of services spans multiple pages", func() {
			It("returns a list of instance IDs", func() {
				offeringID := "D94A086D-203D-4966-A6F1-60A9E2300F72"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_1.json")),
					mockcfapi.ListServiceOfferingsForPage(2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response_page_2.json")),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
					mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetInstancesOfServiceOffering(offeringID, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(ConsistOf("520f8566-b727-4c67-8be8-d9285645e936", "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", "2f759033-04a4-426b-bccd-01722036c152"))
			})
		})

		Context("when the list of plans spans multiple pages", func() {
			It("returns a list of instance IDs", func() {
				offeringID := "D94A086D-203D-4966-A6F1-60A9E2300F72"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_1.json")),
					mockcfapi.ListServicePlansForPage(serviceGUID, 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response_page_2.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
					mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_response.json")),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetInstancesOfServiceOffering(offeringID, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(ConsistOf("520f8566-b727-4c67-8be8-d9285645e936", "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", "2f759033-04a4-426b-bccd-01722036c152"))
			})
		})

		Context("when the list of instances spans multiple pages", func() {
			It("returns a list of instance IDs", func() {
				offeringID := "D94A086D-203D-4966-A6F1-60A9E2300F72"

				server.VerifyAndMock(
					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans(serviceGUID).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_1_response.json")),
					mockcfapi.ListServiceInstances("2777ad05-8114-4169-8188-2ef5f39e0c6b").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_page_1.json")),
					mockcfapi.ListServiceInstancesForPage("2777ad05-8114-4169-8188-2ef5f39e0c6b", 2).WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_instances_for_plan_2_page_2.json")),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				instances, err := client.GetInstancesOfServiceOffering(offeringID, testLogger)
				Expect(err).NotTo(HaveOccurred())
				Expect(instances).To(ConsistOf("520f8566-b727-4c67-8be8-d9285645e936", "f897f40d-0b2d-474a-a5c9-98426a2cb4b8", "2f759033-04a4-426b-bccd-01722036c152"))
			})
		})

		Context("when the list of services cannot be retrieved", func() {
			It("returns an error", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(

					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstancesOfServiceOffering(offeringID, testLogger)
				Expect(err).To(MatchError(ContainSubstring("oops")))
			})
		})

		Context("when the list of plans for the service cannot be retrieved", func() {
			It("returns an error", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(

					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstancesOfServiceOffering(offeringID, testLogger)
				Expect(err).To(MatchError(ContainSubstring("oops")))
			})
		})

		Context("when the list of instances for a plan cannot be retrieved", func() {
			It("returns an error", func() {
				offeringID := "8F3E8998-5FD0-4F32-924A-5478DC390A5F"

				server.VerifyAndMock(

					mockcfapi.ListServiceOfferings().WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_services_response.json")),
					mockcfapi.ListServicePlans("34c08156-5b5d-4cc1-9af1-29cda9ec056f").WithAuthorizationHeader(cfAuthorizationHeader).RespondsOKWith(fixture("list_service_plans_response.json")),
					mockcfapi.ListServiceInstances("ff717e7c-afd5-4d0a-bafe-16c7eff546ec").WithAuthorizationHeader(cfAuthorizationHeader).RespondsInternalServerErrorWith("oops"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetInstancesOfServiceOffering(offeringID, testLogger)
				Expect(err).To(MatchError(ContainSubstring("oops")))
			})
		})
	})

	Describe("GetBindingsForInstance", func() {
		const serviceInstanceGUID = "92d707ce-c06c-421a-a1d2-ed1e750af650"

		It("returns a list of bindings", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceBindings(serviceInstanceGUID).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_bindings_response_page_1.json")),
				mockcfapi.ListServiceBindingsForPage(serviceInstanceGUID, 2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_bindings_response_page_2.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			bindings, err := client.GetBindingsForInstance(serviceInstanceGUID, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(bindings).To(HaveLen(2))
			Expect(bindings[0].GUID).To(Equal("83a87158-92b2-46ea-be66-9dad6b2cb116"))
			Expect(bindings[0].AppGUID).To(Equal("31809eda-4bdd-44fc-b804-eefe662b3a98"))

			Expect(bindings[1].GUID).To(Equal("9dad-6b2cb116-83a87158-92b2-46ea-be66"))
			Expect(bindings[1].AppGUID).To(Equal("eefe-662b3a98-31809eda-4bdd-44fcb804"))
		})

		Context("when the list bindings request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.ListServiceBindings(serviceInstanceGUID).RespondsInternalServerErrorWith("no bindings for you"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetBindingsForInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(`Unexpected reponse status 500, "no bindings for you"`))
			})
		})
	})

	Describe("GetServiceKeysForInstance", func() {
		const serviceInstanceGUID = "92d707ce-c06c-421a-a1d2-ed1e750af650"

		It("return a list of service keys", func() {
			server.VerifyAndMock(
				mockcfapi.ListServiceKeys(serviceInstanceGUID).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_service_keys_response_page_1.json")),
				mockcfapi.ListServiceKeysForPage(serviceInstanceGUID, 2).
					WithAuthorizationHeader(cfAuthorizationHeader).
					RespondsOKWith(fixture("list_service_keys_response_page_2.json")),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			serviceKeys, err := client.GetServiceKeysForInstance(serviceInstanceGUID, testLogger)
			Expect(err).NotTo(HaveOccurred())

			Expect(serviceKeys).To(HaveLen(2))
			Expect(serviceKeys[0].GUID).To(Equal("3c8076a6-0d85-11e7-811e-685b3585cc4e"))
			Expect(serviceKeys[1].GUID).To(Equal("23549ec8-0d85-11e7-82e1-685b3585cc4e"))
		})

		Context("when the list service keys request fails", func() {
			It("returns an error", func() {
				server.VerifyAndMock(
					mockcfapi.ListServiceKeys(serviceInstanceGUID).RespondsInternalServerErrorWith("no service keys for you"),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.GetServiceKeysForInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(`Unexpected reponse status 500, "no service keys for you"`))
			})
		})
	})

	Describe("DeleteBinding", func() {
		var binding cf.Binding

		BeforeEach(func() {
			binding = cf.Binding{
				GUID:    "596736f1-eee4-4249-a201-e21f00a55209",
				AppGUID: "65bdd3a3-f471-4108-a7e8-67627ba76d6a",
			}
		})

		Context("when the response is 204 No Content", func() {
			var err error

			BeforeEach(func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceBinding(binding.AppGUID, binding.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNoContent(),
				)

				var client cf.Client
				client, err = cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())
				err = client.DeleteBinding(binding, testLogger)
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("logs the delete request", func() {
				Expect(logBuffer).To(gbytes.Say("DELETE %s/v2/apps/%s/service_bindings/%s", server.URL, binding.AppGUID, binding.GUID))
			})
		})

		Context("when the response is 404 Not Found", func() {
			It("does not return an error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceBinding(binding.AppGUID, binding.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNotFoundWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteBinding(binding, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the auth header builder returns an error", func() {
			It("returns the error", func() {
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				authHeaderBuilder.BuildReturns(cfAuthorizationHeader, errors.New("no header for you"))

				err = client.DeleteBinding(binding, testLogger)
				Expect(err).To(MatchError("no header for you"))
			})
		})

		Context("when the response has an unexpected status code", func() {
			It("return the error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceBinding(binding.AppGUID, binding.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsForbiddenWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteBinding(binding, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected reponse status 403"))
				Expect(err.Error()).To(ContainSubstring(`"{\"foo\":\"bar\"}"`))
			})
		})
	})

	Describe("DeleteServiceKey", func() {
		var serviceKey cf.ServiceKey

		BeforeEach(func() {
			serviceKey = cf.ServiceKey{
				GUID: "596736f1-eee4-4249-a201-e21f00a55209",
			}
		})

		Context("when the response is 204 No Content", func() {
			var err error

			BeforeEach(func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceKey(serviceKey.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNoContent(),
				)

				var client cf.Client
				client, err = cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceKey(serviceKey, testLogger)
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("logs the delete request", func() {
				Expect(logBuffer).To(gbytes.Say("DELETE %s/v2/service_keys/%s", server.URL, serviceKey.GUID))
			})
		})

		Context("when the response is 404 Not Found", func() {
			It("does not return an error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceKey(serviceKey.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNotFoundWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceKey(serviceKey, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the auth header builder returns an error", func() {
			It("returns the error", func() {
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				authHeaderBuilder.BuildReturns(cfAuthorizationHeader, errors.New("no header for you"))

				err = client.DeleteServiceKey(serviceKey, testLogger)
				Expect(err).To(MatchError("no header for you"))
			})
		})

		Context("when the response has an unexpected status code", func() {
			It("return the error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceKey(serviceKey.GUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsForbiddenWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceKey(serviceKey, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected reponse status 403"))
				Expect(err.Error()).To(ContainSubstring(`"{\"foo\":\"bar\"}"`))
			})
		})
	})

	Describe("DeleteServiceInstance", func() {
		const serviceInstanceGUID = "596736f1-eee4-4249-a201-e21f00a55209"

		Context("when the request is accepted", func() {
			var err error

			BeforeEach(func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceInstance(serviceInstanceGUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsAcceptedWith(""),
				)

				var client cf.Client
				client, err = cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("logs the delete request", func() {
				Expect(logBuffer).To(gbytes.Say("DELETE %s/v2/service_instances/%s\\?accepts_incomplete\\=true", server.URL, serviceInstanceGUID))
			})
		})

		Context("when the response is 404 Not Found", func() {
			It("does not return an error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceInstance(serviceInstanceGUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsNotFoundWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the response has an unexpected status code", func() {
			It("return the error", func() {
				server.VerifyAndMock(
					mockcfapi.DeleteServiceInstance(serviceInstanceGUID).
						WithAuthorizationHeader(cfAuthorizationHeader).
						WithContentType("application/x-www-form-urlencoded").
						RespondsForbiddenWith(`{"foo":"bar"}`),
				)

				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected reponse status 403"))
				Expect(err.Error()).To(ContainSubstring(`"{\"foo\":\"bar\"}"`))
			})
		})

		Context("when the auth header builder returns an error", func() {
			It("returns the error", func() {
				client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
				Expect(err).NotTo(HaveOccurred())

				authHeaderBuilder.BuildReturns(cfAuthorizationHeader, errors.New("no header for you"))

				err = client.DeleteServiceInstance(serviceInstanceGUID, testLogger)
				Expect(err).To(MatchError("no header for you"))
			})
		})
	})

	Describe("GetAPIVersion", func() {
		It("gets cloudfoundry api version", func() {
			server.VerifyAndMock(
				mockcfapi.GetInfo().RespondsOKWith(
					`{
					  "name": "",
					  "build": "",
					  "support": "http://support.cloudfoundry.com",
					  "version": 0,
					  "description": "",
					  "authorization_endpoint": "https://login.services-enablement-bosh-lite-aws.cf-app.com",
					  "token_endpoint": "https://uaa.services-enablement-bosh-lite-aws.cf-app.com",
					  "min_cli_version": null,
					  "min_recommended_cli_version": null,
					  "api_version": "2.57.0",
					  "app_ssh_endpoint": "ssh.services-enablement-bosh-lite-aws.cf-app.com:2222",
					  "app_ssh_host_key_fingerprint": "a6:d1:08:0b:b0:cb:9b:5f:c4:ba:44:2a:97:26:19:8a",
					  "app_ssh_oauth_client": "ssh-proxy",
					  "logging_endpoint": "wss://loggregator.services-enablement-bosh-lite-aws.cf-app.com:443",
					  "doppler_logging_endpoint": "wss://doppler.services-enablement-bosh-lite-aws.cf-app.com:4443"
					}`,
				),
			)
			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			Expect(client.GetAPIVersion(testLogger)).To(Equal("2.57.0"))
		})

		It("fails, if get info fails", func() {
			server.VerifyAndMock(
				mockcfapi.GetInfo().RespondsInternalServerErrorWith("nothing today, thank you"),
			)

			client, err := cf.New(server.URL, authHeaderBuilder, nil, true)
			Expect(err).NotTo(HaveOccurred())

			_, getVersionErr := client.GetAPIVersion(testLogger)
			Expect(getVersionErr.Error()).To(ContainSubstring("nothing today, thank you"))
		})
	})
})

func fixture(filename string) string {
	file, err := os.Open(path.Join("fixtures", filename))
	Expect(err).NotTo(HaveOccurred())
	content, err := ioutil.ReadAll(file)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
}
