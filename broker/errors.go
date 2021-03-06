// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package broker

import (
	"context"
	"errors"
	"fmt"

	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/brokercontext"
	"github.com/pivotal-cf/on-demand-service-broker/serviceadapter"
)

const (
	GenericErrorPrefix         = "There was a problem completing your request. Please contact your operations team providing the following information:"
	PendingChangesErrorMessage = "Service cannot be updated at this time, please try again later or contact your operator for more information"
	OperationInProgressMessage = "An operation is in progress for your service instance. Please try again later."

	UpdateLoggerAction = ""
)

type OperationInProgressError struct {
	error
}

func NewOperationInProgressError(e error) error {
	return OperationInProgressError{e}
}

var NilError = DisplayableError{nil, nil}

// TODO SF Remove by logging operator messages when raising the error?
type DisplayableError struct {
	errorForCFUser   error
	errorForOperator error
}

func (e DisplayableError) ErrorForCFUser() error {
	return e.errorForCFUser
}

func (e DisplayableError) Error() string {
	return fmt.Sprintf("error: %s. error for user: %s.", e.errorForOperator, e.errorForCFUser)
}

func (e DisplayableError) Occurred() bool {
	return e.errorForCFUser != nil && e.errorForOperator != nil
}

func NewDisplayableError(errorForCFUser, errForOperator error) DisplayableError {
	return DisplayableError{
		errorForCFUser,
		errForOperator,
	}
}

func NewBoshRequestError(action string, requestError error) DisplayableError {
	return DisplayableError{
		fmt.Errorf("Currently unable to %s service instance, please try again later", action),
		requestError,
	}
}

func NewGenericError(ctx context.Context, err error) DisplayableError {
	serviceName := brokercontext.GetServiceName(ctx)
	instanceID := brokercontext.GetInstanceID(ctx)
	reqID := brokercontext.GetReqID(ctx)
	operation := brokercontext.GetOperation(ctx)
	boshTaskID := brokercontext.GetBoshTaskID(ctx)

	message := fmt.Sprintf(
		"%s service: %s, service-instance-guid: %s, broker-request-id: %s",
		GenericErrorPrefix,
		serviceName,
		instanceID,
		reqID,
	)

	if boshTaskID != 0 {
		message += fmt.Sprintf(", task-id: %d", boshTaskID)
	}

	if operation != "" {
		message += fmt.Sprintf(", operation: %s", operation)
	}

	return DisplayableError{
		errorForCFUser:   errors.New(message),
		errorForOperator: err,
	}
}

// TODO test this logic
func adapterToAPIError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	switch err.(type) {
	case serviceadapter.BindingAlreadyExistsError:
		return brokerapi.ErrBindingAlreadyExists
	case serviceadapter.BindingNotFoundError:
		return brokerapi.ErrBindingDoesNotExist
	case serviceadapter.AppGuidNotProvidedError:
		return brokerapi.ErrAppGuidNotProvided
	case serviceadapter.UnknownFailureError:
		if err.Error() == "" {
			//Adapter returns an unknown error with no message
			err = NewGenericError(ctx, err).ErrorForCFUser()
		}

		return err
	default:
		return NewGenericError(ctx, err).ErrorForCFUser()
	}
}
