// This file was generated by counterfeiter
package fakes

import (
	"log"
	"sync"

	"github.com/pivotal-cf/on-demand-service-broker/purger"
)

type FakeCloudFoundryClient struct {
	DisableServiceAccessStub        func(serviceOfferingID string, logger *log.Logger) error
	disableServiceAccessMutex       sync.RWMutex
	disableServiceAccessArgsForCall []struct {
		serviceOfferingID string
		logger            *log.Logger
	}
	disableServiceAccessReturns struct {
		result1 error
	}
	disableServiceAccessReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCloudFoundryClient) DisableServiceAccess(serviceOfferingID string, logger *log.Logger) error {
	fake.disableServiceAccessMutex.Lock()
	ret, specificReturn := fake.disableServiceAccessReturnsOnCall[len(fake.disableServiceAccessArgsForCall)]
	fake.disableServiceAccessArgsForCall = append(fake.disableServiceAccessArgsForCall, struct {
		serviceOfferingID string
		logger            *log.Logger
	}{serviceOfferingID, logger})
	fake.recordInvocation("DisableServiceAccess", []interface{}{serviceOfferingID, logger})
	fake.disableServiceAccessMutex.Unlock()
	if fake.DisableServiceAccessStub != nil {
		return fake.DisableServiceAccessStub(serviceOfferingID, logger)
	}
	if specificReturn {
		return ret.result1
	}
	return fake.disableServiceAccessReturns.result1
}

func (fake *FakeCloudFoundryClient) DisableServiceAccessCallCount() int {
	fake.disableServiceAccessMutex.RLock()
	defer fake.disableServiceAccessMutex.RUnlock()
	return len(fake.disableServiceAccessArgsForCall)
}

func (fake *FakeCloudFoundryClient) DisableServiceAccessArgsForCall(i int) (string, *log.Logger) {
	fake.disableServiceAccessMutex.RLock()
	defer fake.disableServiceAccessMutex.RUnlock()
	return fake.disableServiceAccessArgsForCall[i].serviceOfferingID, fake.disableServiceAccessArgsForCall[i].logger
}

func (fake *FakeCloudFoundryClient) DisableServiceAccessReturns(result1 error) {
	fake.DisableServiceAccessStub = nil
	fake.disableServiceAccessReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) DisableServiceAccessReturnsOnCall(i int, result1 error) {
	fake.DisableServiceAccessStub = nil
	if fake.disableServiceAccessReturnsOnCall == nil {
		fake.disableServiceAccessReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.disableServiceAccessReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeCloudFoundryClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.disableServiceAccessMutex.RLock()
	defer fake.disableServiceAccessMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeCloudFoundryClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ purger.CloudFoundryClient = new(FakeCloudFoundryClient)
