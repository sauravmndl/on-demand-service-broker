// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_new_tests

import (
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/pivotal-cf/on-demand-service-broker/config"
)

var (
	brokerPath         = NewBinary("github.com/pivotal-cf/on-demand-service-broker/cmd/on-demand-service-broker")
	serviceAdapterPath = NewBinary("github.com/pivotal-cf/on-demand-service-broker/integration_new_tests/mock/adapter")
)

type Requestifier func(*BrokerEnvironment) *http.Request
type ResponseChecker func(*http.Response)
type LogChecker func(*BrokerEnvironment)
type ConfigUpdater func(config *config.Config) *config.Config

func DefaultConfig(source *config.Config) *config.Config { return source }

type TestSetup struct {
	credhub             Credhub
	serviceAdapterSetup func(*ServiceAdapter, ServiceInstanceID)
	setup               func(*BrokerEnvironment)
	requestifier        Requestifier
	updatedConfig       ConfigUpdater
}

func When(requestifier Requestifier) Requestifier {
	return requestifier
}

func (r Requestifier) With(updatedConfig ConfigUpdater, credhub Credhub, serviceAdapterSetup func(*ServiceAdapter, ServiceInstanceID), setup func(*BrokerEnvironment)) *TestSetup {
	return &TestSetup{
		credhub:             credhub,
		serviceAdapterSetup: serviceAdapterSetup,
		setup:               setup,
		requestifier:        r,
		updatedConfig:       updatedConfig,
	}
}

func (ts *TestSetup) theBroker(checkResponse ResponseChecker, checkLogs ...LogChecker) {
	env := NewBrokerEnvironment(NewBosh(), NewCloudFoundry(), NewServiceAdapter(serviceAdapterPath.Path()), ts.credhub, brokerPath.Path())
	defer env.Close()

	ts.serviceAdapterSetup(env.ServiceAdapter, env.serviceInstanceID)
	env.Start(ts.updatedConfig)
	ts.setup(env)

	response := responseTo(ts.requestifier(env))
	checkResponse(response)
	for _, check := range checkLogs {
		check(env)
	}
	env.Verify()
}

func RespondsWith(expectedStatus int, matchBody types.GomegaMatcher) ResponseChecker {
	return func(response *http.Response) {
		Expect(response.StatusCode).To(Equal(expectedStatus))
		Expect(bodyOf(response)).To(matchBody)
	}
}

func Logs(expectedMessage string) LogChecker {
	return func(env *BrokerEnvironment) {
		env.Broker.HasLogged(expectedMessage)
	}
}

func LogsWithServiceId(expectedMessageTemplate string) LogChecker {
	return func(env *BrokerEnvironment) {
		env.Broker.HasLogged(fmt.Sprintf(expectedMessageTemplate, env.serviceInstanceID))
	}
}

func LogsWithDeploymentName(expectedMessageTemplate string) LogChecker {
	return func(env *BrokerEnvironment) {
		env.Broker.HasLogged(fmt.Sprintf(expectedMessageTemplate, env.DeploymentName()))
	}
}

func responseTo(request *http.Request) *http.Response {
	response, err := http.DefaultClient.Do(request)
	Expect(err).ToNot(HaveOccurred())
	return response
}

func bodyOf(response *http.Response) []byte {
	body, err := ioutil.ReadAll(response.Body)
	Expect(err).NotTo(HaveOccurred())
	return body
}
