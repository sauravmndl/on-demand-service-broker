// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package mgmtapi_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/cf"
	"github.com/pivotal-cf/on-demand-service-broker/config"
	"github.com/pivotal-cf/on-demand-service-broker/loggerfactory"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi"
	"github.com/pivotal-cf/on-demand-service-broker/mgmtapi/fake_manageable_broker"
	"github.com/pivotal-cf/on-demand-service-broker/task"
)

var _ = Describe("Management API", func() {
	var (
		server           *httptest.Server
		manageableBroker *fake_manageable_broker.FakeManageableBroker
		logs             *gbytes.Buffer
		loggerFactory    *loggerfactory.LoggerFactory
		serviceOffering  config.ServiceOffering
	)

	BeforeEach(func() {
		serviceOffering = config.ServiceOffering{
			ID:    "some_service_offering-id",
			Name:  "some_service_offering",
			Plans: []config.Plan{{ID: "foo_id", Name: "foo_plan"}, {ID: "bar_id", Name: "bar_plan"}},
		}
		logs = gbytes.NewBuffer()
		loggerFactory = loggerfactory.New(io.MultiWriter(GinkgoWriter, logs), "mgmtapi-unit-tests", log.LstdFlags)
		manageableBroker = new(fake_manageable_broker.FakeManageableBroker)
	})

	JustBeforeEach(func() {
		router := mux.NewRouter()
		mgmtapi.AttachRoutes(router, manageableBroker, serviceOffering, loggerFactory)
		server = httptest.NewServer(router)
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("listing all instances", func() {
		var listResp *http.Response

		JustBeforeEach(func() {
			var err error
			listResp, err = http.Get(fmt.Sprintf("%s/mgmt/service_instances", server.URL))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("of which there are three", func() {
			var (
				instance1 = mgmtapi.Instance{
					InstanceID: "instance-guid-1",
				}
				instance2 = mgmtapi.Instance{
					InstanceID: "instance-guid-2",
				}
				instance3 = mgmtapi.Instance{
					InstanceID: "instance-guid-3",
				}
			)

			BeforeEach(func() {
				instances := []string{instance1.InstanceID, instance2.InstanceID, instance3.InstanceID}
				manageableBroker.InstancesReturns(instances, nil)
			})

			It("returns HTTP 200", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns a list of all three instances", func() {
				var instances []mgmtapi.Instance
				Expect(json.NewDecoder(listResp.Body).Decode(&instances)).To(Succeed())
				Expect(instances).To(ConsistOf(instance1, instance2, instance3))
			})
		})

		Context("but failing to do so", func() {
			BeforeEach(func() {
				manageableBroker.InstancesReturns(nil, errors.New("error getting instances"))
			})

			It("returns HTTP 500", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("logs the error", func() {
				Eventually(logs).Should(gbytes.Say("error occurred querying instances: error getting instances"))
			})
		})
	})

	Describe("upgrading an instance", func() {
		var (
			instanceID = "283974"
			taskID     = 54321

			upgradeResp *http.Response
		)

		JustBeforeEach(func() {
			var err error
			upgradeResp, err = Patch(fmt.Sprintf("%s/mgmt/service_instances/"+instanceID, server.URL))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when it succeeds", func() {
			contextID := "some-context-id"
			planID := "some-plan-id"

			BeforeEach(func() {
				manageableBroker.UpgradeReturns(broker.OperationData{
					BoshTaskID:    taskID,
					BoshContextID: contextID,
					PlanID:        planID,
					OperationType: broker.OperationTypeUpgrade,
				}, nil)
			})

			It("upgrades the instance using the broker", func() {
				Expect(manageableBroker.UpgradeCallCount()).To(Equal(1))
				_, actualInstanceID, _ := manageableBroker.UpgradeArgsForCall(0)
				Expect(actualInstanceID).To(Equal(instanceID))
			})

			It("responds with HTTP 202", func() {
				Expect(upgradeResp.StatusCode).To(Equal(http.StatusAccepted))
			})

			It("responds with operation data", func() {
				var upgradeRespBody broker.OperationData
				Expect(json.NewDecoder(upgradeResp.Body).Decode(&upgradeRespBody)).To(Succeed())
				Expect(upgradeRespBody.BoshTaskID).To(Equal(taskID))
				Expect(upgradeRespBody.BoshContextID).To(Equal(contextID))
				Expect(upgradeRespBody.PlanID).To(Equal(planID))
				Expect(upgradeRespBody.OperationType).To(Equal(broker.OperationTypeUpgrade))
			})
		})

		Context("when the CF service instance is not found", func() {
			BeforeEach(func() {
				manageableBroker.UpgradeReturns(broker.OperationData{}, cf.ResourceNotFoundError{})
			})

			It("responds with HTTP 404 Not Found", func() {
				Expect(upgradeResp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when the bosh deployment is not found", func() {
			BeforeEach(func() {
				manageableBroker.UpgradeReturns(broker.OperationData{}, task.NewDeploymentNotFoundError(errors.New("error finding deployment")))
			})

			It("responds with HTTP 410 Gone", func() {
				Expect(upgradeResp.StatusCode).To(Equal(http.StatusGone))
			})
		})

		Context("when there is an operation in progress", func() {
			BeforeEach(func() {
				manageableBroker.UpgradeReturns(broker.OperationData{}, broker.NewOperationInProgressError(errors.New("operation in progress error")))
			})

			It("responds with HTTP 409 Conflict", func() {
				Expect(upgradeResp.StatusCode).To(Equal(http.StatusConflict))
			})
		})

		Context("when it fails", func() {
			BeforeEach(func() {
				manageableBroker.UpgradeReturns(broker.OperationData{}, errors.New("upgrade error"))
			})

			It("responds with HTTP 500", func() {
				Expect(upgradeResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("includes the upgrade error in the response", func() {
				Expect(ioutil.ReadAll(upgradeResp.Body)).To(MatchJSON(`{"description": "upgrade error"}`))
			})

			It("logs the error", func() {
				Eventually(logs).Should(gbytes.Say(fmt.Sprintf("error occurred upgrading instance %s: upgrade error", instanceID)))
			})
		})
	})

	Describe("producing service metrics", func() {
		var instancesForPlanResponse *http.Response

		JustBeforeEach(func() {
			var err error
			instancesForPlanResponse, err = http.Get(fmt.Sprintf("%s/mgmt/metrics", server.URL))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when no quota is set", func() {
			Context("when there is one plan with instance count", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[string]int{"foo_id": 2}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})

				It("returns the correct number of instances", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 2,
							Unit:  "count",
						},
					))
				})
			})

			Context("when there are multiple plans with instance counts", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[string]int{"foo_id": 2, "bar_id": 3}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})

				It("returns the correct number of instances", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/bar_plan/total_instances",
							Value: 3,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 5,
							Unit:  "count",
						},
					))
				})
			})

			Context("when the instance count cannot be retrieved", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(nil, errors.New("error counting instances"))
				})

				It("returns HTTP 500", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the broker is not registered with CF", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[string]int{}, nil)
				})

				It("returns HTTP 503", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusServiceUnavailable))
				})
			})
		})

		Context("when a plan quota is set", func() {
			BeforeEach(func() {
				limit := 7
				serviceOffering.Plans[0].Quotas = config.Quotas{ServiceInstanceLimit: &limit}
			})

			Context("when the instance count can be retrieved", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[string]int{"foo_id": 2}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the correct number of instances and quota", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/quota_remaining",
							Value: 5,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 2,
							Unit:  "count",
						},
					))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})
			})
		})

		Context("when a global quota is set", func() {
			BeforeEach(func() {
				limit := 12
				serviceOffering.GlobalQuotas = config.Quotas{ServiceInstanceLimit: &limit}
			})

			Context("when the instance count can be retrieved", func() {
				BeforeEach(func() {
					manageableBroker.CountInstancesOfPlansReturns(map[string]int{"foo_id": 2, "bar_id": 3}, nil)
				})

				It("returns HTTP 200", func() {
					Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the correct number of instances", func() {
					defer instancesForPlanResponse.Body.Close()
					var brokerMetrics []mgmtapi.Metric

					Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
					Expect(brokerMetrics).To(ConsistOf(
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
							Value: 2,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/bar_plan/total_instances",
							Value: 3,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/total_instances",
							Value: 5,
							Unit:  "count",
						},
						mgmtapi.Metric{
							Key:   "/on-demand-broker/some_service_offering/quota_remaining",
							Value: 7,
							Unit:  "count",
						},
					))
				})

				It("counts instances for the plan", func() {
					Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
				})
			})
		})

		Context("when there are no service instances", func() {
			BeforeEach(func() {
				manageableBroker.CountInstancesOfPlansReturns(map[string]int{"foo_id": 0, "bar_id": 0}, nil)
			})

			It("returns HTTP 200", func() {
				Expect(instancesForPlanResponse.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns the correct number of instances", func() {
				defer instancesForPlanResponse.Body.Close()
				var brokerMetrics []mgmtapi.Metric

				Expect(json.NewDecoder(instancesForPlanResponse.Body).Decode(&brokerMetrics)).To(Succeed())
				Expect(brokerMetrics).To(ConsistOf(
					mgmtapi.Metric{
						Key:   "/on-demand-broker/some_service_offering/foo_plan/total_instances",
						Value: 0,
						Unit:  "count",
					},
					mgmtapi.Metric{
						Key:   "/on-demand-broker/some_service_offering/bar_plan/total_instances",
						Value: 0,
						Unit:  "count",
					},
					mgmtapi.Metric{
						Key:   "/on-demand-broker/some_service_offering/total_instances",
						Value: 0,
						Unit:  "count",
					},
				))
			})

			It("counts instances for the plan", func() {
				Expect(manageableBroker.CountInstancesOfPlansCallCount()).To(Equal(1))
			})
		})
	})

	Describe("listing orphan service deployments", func() {
		var listResp *http.Response

		JustBeforeEach(func() {
			var err error
			listResp, err = http.Get(fmt.Sprintf("%s/mgmt/orphan_deployments", server.URL))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are no orphans", func() {
			It("returns HTTP 200", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns no deployments", func() {
				defer listResp.Body.Close()
				body, err := ioutil.ReadAll(listResp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body).To(MatchJSON(`[]`))
			})
		})

		Context("when there are some orphans", func() {
			var (
				orphan1 = mgmtapi.Deployment{
					Name: "orphan1",
				}
				orphan2 = mgmtapi.Deployment{
					Name: "orphan2",
				}
			)

			BeforeEach(func() {
				manageableBroker.OrphanDeploymentsReturns([]string{orphan1.Name, orphan2.Name}, nil)
			})

			It("returns HTTP 200", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns some deployments", func() {
				var orphans []mgmtapi.Deployment
				Expect(json.NewDecoder(listResp.Body).Decode(&orphans)).To(Succeed())
				Expect(orphans).To(ConsistOf(orphan1, orphan2))
			})
		})

		Context("when broker returns an error", func() {
			BeforeEach(func() {
				manageableBroker.OrphanDeploymentsReturns([]string{}, errors.New("Broker errored."))
			})

			It("returns HTTP 500", func() {
				Expect(listResp.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("returns an empty body", func() {
				defer listResp.Body.Close()
				body, err := ioutil.ReadAll(listResp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body).To(BeEmpty())
			})

			It("logs an error", func() {
				Eventually(logs).Should(gbytes.Say("error occurred querying orphan deployments: Broker errored."))
			})
		})
	})
})

func Patch(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("PATCH", url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
