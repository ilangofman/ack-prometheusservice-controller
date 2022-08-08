// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package alert_manager_definition

import (
	"context"
	"errors"
	"time"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go/service/prometheusservice"

	"github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

var (
	ErrAlertManagerDefinitionCreating = errors.New("Alert Manager Definition in 'CREATING' state, cannot be modified or deleted")
	ErrAlertManagerDefinitionDeleting = errors.New("Alert Manager Definition in 'DELETING' state, cannot be modified or deleted")
	ErrAlertManagerDefinitionUpdating = errors.New("Alert Manager Definition in 'UPDATING' state, cannot be modified or deleted")
)

var (
	requeueWaitWhileDeleting = ackrequeue.NeededAfter(
		ErrAlertManagerDefinitionDeleting,
		10*time.Second,
	)
	requeueWaitWhileCreating = ackrequeue.NeededAfter(
		ErrAlertManagerDefinitionCreating,
		15*time.Second,
	)
	requeueWaitWhileUpdating = ackrequeue.NeededAfter(
		ErrAlertManagerDefinitionUpdating,
		10*time.Second,
	)
	requeueWaitWhileUpdatingWithoutError = ackrequeue.NeededAfter(
		nil,
		10*time.Second,
	)
)

// alertManagerDefinitionCreating returns true if the supplied definition
// is in the process of being created
func alertManagerDefinitionCreating(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(v1alpha1.AlertManagerDefinitionStatusCode_CREATING)
}

// alertManagerDefinitionCreating returns true if the supplied definition
// is in the process of being deleted
func alertManagerDefinitionDeleting(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(v1alpha1.AlertManagerDefinitionStatusCode_DELETING)
}

// alertManagerDefinitionCreating returns true if the supplied definition
// is in the process of being updated
func alertManagerDefinitionUpdating(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(v1alpha1.AlertManagerDefinitionStatusCode_UPDATING)
}

// AlertManagerDefinitionStatusCode_CREATING returns true if the supplied
// definition is in an active state
func alertManagerDefinitionActive(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(v1alpha1.AlertManagerDefinitionStatusCode_ACTIVE)
}

// alertManagerDefinitionCreating returns true if the supplied definition
// resulted in a failed creation or failed update
func alertManagerDefinitionStatusFailed(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(v1alpha1.AlertManagerDefinitionStatusCode_CREATION_FAILED) || ws == string(v1alpha1.AlertManagerDefinitionStatusCode_UPDATE_FAILED)
}

func (rm *resourceManager) customUpdateAlertManagerDefinition(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateAlertManagerDefinition")
	defer exit(err)

	// Check if the state is being currently created, updated or deleted.
	// For failed states (create & update) and active states, the user can
	// still update the alert manager definition.
	if alertManagerDefinitionCreating(latest) {
		return desired, requeueWaitWhileCreating
	}
	if alertManagerDefinitionUpdating(latest) {
		return desired, requeueWaitWhileUpdating
	}
	if alertManagerDefinitionDeleting(latest) {
		return desired, requeueWaitWhileDeleting
	}

	if delta.DifferentAt("Spec.AlertmanagerConfig") {
		err = rm.customUpdateAlertManagerDefinitionData(ctx, desired)
		if err != nil {
			return nil, err
		}
	}

	readOneLatest, err := rm.ReadOne(ctx, desired)
	if err != nil {
		return nil, err
	}
	r := rm.concreteResource(readOneLatest)

	return r, nil

}
func (rm *resourceManager) customUpdateAlertManagerDefinitionData(
	ctx context.Context,
	desired *resource,
) error {

	// Convert the string version of the definition to a byte slice
	// because the API expects a base64 encoding. The conversion to base64
	// is handled automatically by k8s.
	if desired.ko.Spec.AlertmanagerConfig != nil {
		desired.ko.Spec.Data = []byte(*desired.ko.Spec.AlertmanagerConfig)
	}

	input := &svcsdk.PutAlertManagerDefinitionInput{
		Data:        desired.ko.Spec.Data,
		WorkspaceId: desired.ko.Spec.WorkspaceID,
	}

	resp, err := rm.sdkapi.PutAlertManagerDefinitionWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "putAlertManagerDefinition", err)
	if err != nil {
		return err
	}

	// Check the status of the alert manager definition
	if resp.Status != nil {
		if resp.Status.StatusCode != nil {
			desired.ko.Status.StatusCode = resp.Status.StatusCode
		} else {
			desired.ko.Status.StatusCode = nil
		}
		if resp.Status.StatusReason != nil {
			desired.ko.Status.StatusReason = resp.Status.StatusReason
		} else {
			desired.ko.Status.StatusReason = nil
		}
	} else {
		desired.ko.Status.StatusCode = nil
		desired.ko.Status.StatusReason = nil

	}

	return nil
}
