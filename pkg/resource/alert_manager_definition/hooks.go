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

	"github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go/service/prometheusservice"

	corev1 "k8s.io/api/core/v1"
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
		10*time.Second,
	)
	requeueWaitWhileUpdating = ackrequeue.NeededAfter(
		ErrAlertManagerDefinitionUpdating,
		10*time.Second,
	)
)

var (
	// TerminalStatuses are the status strings that are terminal states for an
	// alert manager definition
	TerminalStatuses = []v1alpha1.AlertManagerDefinitionStatusCode{
		v1alpha1.AlertManagerDefinitionStatusCode_CREATION_FAILED,
		v1alpha1.AlertManagerDefinitionStatusCode_DELETING,
	}
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

// tableHasTerminalStatus returns whether the supplied Dynamodb table is in a
// terminal state
func alertManagerDefinitionHasTerminalStatus(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ts := *r.ko.Status.StatusCode
	for _, s := range TerminalStatuses {
		if ts == string(s) {
			return true
		}
	}
	return false
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

	println("ILAN --- -----------ENTERING UPDATE")

	if alertManagerDefinitionHasTerminalStatus(latest) {
		msg := "Alert Manager Definition is in '" + *latest.ko.Status.StatusCode + "' status"
		ackcondition.SetTerminal(desired, corev1.ConditionTrue, &msg, nil)
		ackcondition.SetSynced(desired, corev1.ConditionTrue, nil, nil)
		return desired, nil
	}

	// If status=update/create failed should still be able to create it??

	// Check if the state is active before updated
	if !alertManagerDefinitionActive(latest) {
		msg := "Cannot update alert manager definition as current status=" + string(*latest.ko.Status.StatusCode)
		ackcondition.SetSynced(desired, corev1.ConditionFalse, &msg, nil)
		return desired, ackrequeue.NeededAfter(
			errors.New(msg),
			ackrequeue.DefaultRequeueAfterDuration,
		)
	}

	// maybe not handled by us? Should be called from the runtime?
	// desired = rm.handleImmutableFieldsChangedCondition(desired, delta)

	input := &svcsdk.PutAlertManagerDefinitionInput{
		Data:        desired.ko.Spec.Data,
		WorkspaceId: desired.ko.Spec.WorkspaceID,
	}

	var resp *svcsdk.PutAlertManagerDefinitionOutput
	resp, err = rm.sdkapi.PutAlertManagerDefinitionWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "putAlertManagerDefinition", err)
	if err != nil {
		return nil, err
	}

	ko := desired.ko.DeepCopy()

	rm.setStatusDefaults(ko)

	// Check the status of the alert manager definition
	if resp.Status != nil {
		if resp.Status.StatusCode != nil {
			ko.Status.StatusCode = resp.Status.StatusCode
		} else {
			ko.Status.StatusCode = nil
		}
		if resp.Status.StatusReason != nil {
			ko.Status.StatusReason = resp.Status.StatusReason
		} else {
			ko.Status.StatusReason = nil
		}
	} else {
		ko.Status.StatusCode = nil
		ko.Status.StatusReason = nil

	}

	return &resource{ko}, nil
}
