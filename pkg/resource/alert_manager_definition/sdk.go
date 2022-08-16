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

// Code generated by ack-generate. DO NOT EDIT.

package alert_manager_definition

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go/aws"
	svcsdk "github.com/aws/aws-sdk-go/service/prometheusservice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

// Hack to avoid import errors during build...
var (
	_ = &metav1.Time{}
	_ = strings.ToLower("")
	_ = &aws.JSONValue{}
	_ = &svcsdk.PrometheusService{}
	_ = &svcapitypes.AlertManagerDefinition{}
	_ = ackv1alpha1.AWSAccountID("")
	_ = &ackerr.NotFound
	_ = &ackcondition.NotManagedMessage
	_ = &reflect.Value{}
	_ = fmt.Sprintf("")
	_ = &ackrequeue.NoRequeue{}
)

// sdkFind returns SDK-specific information about a supplied resource
func (rm *resourceManager) sdkFind(
	ctx context.Context,
	r *resource,
) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkFind")
	defer func() {
		exit(err)
	}()
	// If any required fields in the input shape are missing, AWS resource is
	// not created yet. Return NotFound here to indicate to callers that the
	// resource isn't yet created.
	if rm.requiredFieldsMissingFromReadOneInput(r) {
		return nil, ackerr.NotFound
	}

	input, err := rm.newDescribeRequestPayload(r)
	if err != nil {
		return nil, err
	}

	var resp *svcsdk.DescribeAlertManagerDefinitionOutput
	resp, err = rm.sdkapi.DescribeAlertManagerDefinitionWithContext(ctx, input)
	rm.metrics.RecordAPICall("READ_ONE", "DescribeAlertManagerDefinition", err)
	if err != nil {
		if awsErr, ok := ackerr.AWSError(err); ok && awsErr.Code() == "ResourceNotFoundException" {
			return nil, ackerr.NotFound
		}
		return nil, err
	}

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := r.ko.DeepCopy()

	// Check the status of the alert manager definition
	if resp.AlertManagerDefinition.Status != nil {
		if resp.AlertManagerDefinition.Status.StatusCode != nil {
			ko.Status.StatusCode = resp.AlertManagerDefinition.Status.StatusCode
		} else {
			ko.Status.StatusCode = nil
		}
		if resp.AlertManagerDefinition.Status.StatusReason != nil {
			ko.Status.StatusReason = resp.AlertManagerDefinition.Status.StatusReason
		} else {
			ko.Status.StatusReason = nil
		}
	} else {
		ko.Status.StatusCode = nil
		ko.Status.StatusReason = nil

	}

	// When adding an invalid alert manager configuration, the AMP API has different behaviour
	// for different kinds of invalid input. For some invalid input, the API returns an error (e.g. ValidationException)
	// instantly in the http response and we set the controller to terminal state. The specified
	// spec remains the same.
	// For other invalid inputs, the API first accepts the http request with a 200 code, and proceeds to
	// create/update the configuration but ultimately fails after around a minute because of an invalid config. So it
	// is possible for there to be a validation failure in an asynchronous update.
	// For these cases, the status will end up being "UPDATE_FAILED" or "CREATION_FAILED".
	// The behaviour of the API is as follows:
	//         - For a "CREATION_FAILED", the configuration will be empty.
	//         - For an "UPDATE_FAILED", the configuration will be the last valid one (or empty if CREATION_FAILED -> UPDATING -> UPDATE_FAILED).

	// However, from a K8s point of view, this can be confusing when the desired configuration is not the same
	// as the one shown in the resource after creating/updating. For example, resource says "UPDATE_FAILED" and
	// the spec has the previous ACTIVE configuration instead of the one that caused the failed update.

	// Hence, we should treat the asynchronous validation errors similarly to how the regular http validation
	// exceptions are treated in ACK. So when there is a failed creation/update, we don't change the configuration in the spec
	// that caused this failed status, and also set to terminal error.

	// When a failed status occurs, we skip setting the configuration field to be what the API returns,
	// and instead keep it to be what the user desires. We only do this once right after a resource becomes
	// failed and not after because otherwise it would prevent update calls since the configuration wouldn't ever change from failed.
	// So we only want to prevent the configuration changing when the status changed from creating/updating to failed. After, when the user
	// updates the configuration, then it should update.

	// This is done by by checking if the returned status is failed while the current resource isn't.
	if (alertManagerDefinitionStatusFailed(&resource{ko}) && !alertManagerDefinitionStatusFailed(r) &&
		alertManagerDefinitionValidationError(&resource{ko})) {
		msg := "Alert Manager Definition is in '" + *ko.Status.StatusCode + "' status because of a validating error"
		rm.setStatusDefaults(ko)

		ackcondition.SetTerminal(&resource{ko}, corev1.ConditionTrue, &msg, nil)
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionTrue, nil, nil)

		return &resource{ko}, nil

	}

	// The data field stores the base64 encoding of the alert manager definition.
	// However, to make the CR's more user friendly, we convert the base64 encoding to a
	// string. We store it in a custom created field.
	if resp.AlertManagerDefinition.Data != nil {
		// Convert the base64 byte array to a human-readable string
		alertManagerDefinitionDataString := string(resp.AlertManagerDefinition.Data)

		ko.Spec.Configuration = &alertManagerDefinitionDataString
		if err != nil {
			return nil, err
		}
	} else {
		ko.Spec.Configuration = nil
	}

	// if there is a read call and the status has already failed before, then the if
	// statements above setting the config field would trigger an update call because the server response
	// will not match with the desired configuration. This is expected and needed for when a user updates the
	// resource once a status has become failed.

	// There is one edge case however where this isn't true. When a user creates a valid config but then an invalid update,
	// then the server-side resource will be "UPDATE_FAILED" but the server-side configuration will be the first valid configuration. In the scenario,
	// if a user changes updates the config in their spec back to the original valid one, then the desired and server response will be the same. With no difference,
	// no update call will be triggerred, and the resource will remain in UPDATE_FAILED. As a work around for this edge case, we set the config to nil to
	// force an update call.
	if alertManagerDefinitionStatusFailed(r) {
		ko.Spec.Configuration = nil
	}

	if alertManagerDefinitionUpdating(&resource{ko}) {
		// Setting resource synced condition to false will trigger a requeue of
		// the resource. No need to return a requeue error here.
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
		return &resource{ko}, nil
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}

// requiredFieldsMissingFromReadOneInput returns true if there are any fields
// for the ReadOne Input shape that are required but not present in the
// resource's Spec or Status
func (rm *resourceManager) requiredFieldsMissingFromReadOneInput(
	r *resource,
) bool {
	return r.ko.Spec.WorkspaceID == nil

}

// newDescribeRequestPayload returns SDK-specific struct for the HTTP request
// payload of the Describe API call for the resource
func (rm *resourceManager) newDescribeRequestPayload(
	r *resource,
) (*svcsdk.DescribeAlertManagerDefinitionInput, error) {
	res := &svcsdk.DescribeAlertManagerDefinitionInput{}

	if r.ko.Spec.WorkspaceID != nil {
		res.SetWorkspaceId(*r.ko.Spec.WorkspaceID)
	}

	return res, nil
}

// sdkCreate creates the supplied resource in the backend AWS service API and
// returns a copy of the resource with resource fields (in both Spec and
// Status) filled in with values from the CREATE API operation's Output shape.
func (rm *resourceManager) sdkCreate(
	ctx context.Context,
	desired *resource,
) (created *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkCreate")
	defer func() {
		exit(err)
	}()
	input, err := rm.newCreateRequestPayload(ctx, desired)
	if err != nil {
		return nil, err
	}

	// Convert the string version of the definition to a byte slice
	// because the API expects a base64 encoding. The conversion to base64
	// is handled automatically by k8s.
	if desired.ko.Spec.Configuration != nil {
		input.Data = []byte(*desired.ko.Spec.Configuration)
	}

	var resp *svcsdk.CreateAlertManagerDefinitionOutput
	_ = resp
	resp, err = rm.sdkapi.CreateAlertManagerDefinitionWithContext(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "CreateAlertManagerDefinition", err)
	if err != nil {
		return nil, err
	}
	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

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

	rm.setStatusDefaults(ko)

	// We expect the workspace to be in 'creating' status since we just
	// issued the call to create it, but I suppose it doesn't hurt to check
	// here.
	if alertManagerDefinitionCreating(&resource{ko}) {
		// Setting resource synced condition to false will trigger a requeue of
		// the resource. No need to return a requeue error here.
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
		return &resource{ko}, nil
	}

	return &resource{ko}, nil
}

// newCreateRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Create API call for the resource
func (rm *resourceManager) newCreateRequestPayload(
	ctx context.Context,
	r *resource,
) (*svcsdk.CreateAlertManagerDefinitionInput, error) {
	res := &svcsdk.CreateAlertManagerDefinitionInput{}

	if r.ko.Spec.WorkspaceID != nil {
		res.SetWorkspaceId(*r.ko.Spec.WorkspaceID)
	}

	return res, nil
}

// sdkUpdate patches the supplied resource in the backend AWS service API and
// returns a new resource with updated fields.
func (rm *resourceManager) sdkUpdate(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	return rm.customUpdateAlertManagerDefinition(ctx, desired, latest, delta)
}

// sdkDelete deletes the supplied resource in the backend AWS service API
func (rm *resourceManager) sdkDelete(
	ctx context.Context,
	r *resource,
) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkDelete")
	defer func() {
		exit(err)
	}()
	// Can't delete alert manager definition in non-(ACTIVE/CREATION_FAILED/UPDATE_FAILED) state
	// Otherwise, API will return a 409 and ConflictException
	if !alertManagerDefinitionStatusFailed(r) && !alertManagerDefinitionActive(r) {
		msg := "Cannot delete alert manager definition as the status is not ACTIVE/CREATION_FAILED/UPDATE_FAILED, current status=" + string(*r.ko.Status.StatusCode)
		ackcondition.SetSynced(r, corev1.ConditionFalse, &msg, nil)
		return r, ackrequeue.NeededAfter(
			errors.New(msg),
			ackrequeue.DefaultRequeueAfterDuration,
		)
	}
	input, err := rm.newDeleteRequestPayload(r)
	if err != nil {
		return nil, err
	}
	var resp *svcsdk.DeleteAlertManagerDefinitionOutput
	_ = resp
	resp, err = rm.sdkapi.DeleteAlertManagerDefinitionWithContext(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "DeleteAlertManagerDefinition", err)
	return nil, err
}

// newDeleteRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Delete API call for the resource
func (rm *resourceManager) newDeleteRequestPayload(
	r *resource,
) (*svcsdk.DeleteAlertManagerDefinitionInput, error) {
	res := &svcsdk.DeleteAlertManagerDefinitionInput{}

	if r.ko.Spec.WorkspaceID != nil {
		res.SetWorkspaceId(*r.ko.Spec.WorkspaceID)
	}

	return res, nil
}

// setStatusDefaults sets default properties into supplied custom resource
func (rm *resourceManager) setStatusDefaults(
	ko *svcapitypes.AlertManagerDefinition,
) {
	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if ko.Status.ACKResourceMetadata.Region == nil {
		ko.Status.ACKResourceMetadata.Region = &rm.awsRegion
	}
	if ko.Status.ACKResourceMetadata.OwnerAccountID == nil {
		ko.Status.ACKResourceMetadata.OwnerAccountID = &rm.awsAccountID
	}
	if ko.Status.Conditions == nil {
		ko.Status.Conditions = []*ackv1alpha1.Condition{}
	}
}

// updateConditions returns updated resource, true; if conditions were updated
// else it returns nil, false
func (rm *resourceManager) updateConditions(
	r *resource,
	onSuccess bool,
	err error,
) (*resource, bool) {
	ko := r.ko.DeepCopy()
	rm.setStatusDefaults(ko)

	// Terminal condition
	var terminalCondition *ackv1alpha1.Condition = nil
	var recoverableCondition *ackv1alpha1.Condition = nil
	var syncCondition *ackv1alpha1.Condition = nil
	for _, condition := range ko.Status.Conditions {
		if condition.Type == ackv1alpha1.ConditionTypeTerminal {
			terminalCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeRecoverable {
			recoverableCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeResourceSynced {
			syncCondition = condition
		}
	}
	var termError *ackerr.TerminalError
	if rm.terminalAWSError(err) || err == ackerr.SecretTypeNotSupported || err == ackerr.SecretNotFound || errors.As(err, &termError) {
		if terminalCondition == nil {
			terminalCondition = &ackv1alpha1.Condition{
				Type: ackv1alpha1.ConditionTypeTerminal,
			}
			ko.Status.Conditions = append(ko.Status.Conditions, terminalCondition)
		}
		var errorMessage = ""
		if err == ackerr.SecretTypeNotSupported || err == ackerr.SecretNotFound || errors.As(err, &termError) {
			errorMessage = err.Error()
		} else {
			awsErr, _ := ackerr.AWSError(err)
			errorMessage = awsErr.Error()
		}
		terminalCondition.Status = corev1.ConditionTrue
		terminalCondition.Message = &errorMessage
	} else {
		// Clear the terminal condition if no longer present
		if terminalCondition != nil {
			terminalCondition.Status = corev1.ConditionFalse
			terminalCondition.Message = nil
		}
		// Handling Recoverable Conditions
		if err != nil {
			if recoverableCondition == nil {
				// Add a new Condition containing a non-terminal error
				recoverableCondition = &ackv1alpha1.Condition{
					Type: ackv1alpha1.ConditionTypeRecoverable,
				}
				ko.Status.Conditions = append(ko.Status.Conditions, recoverableCondition)
			}
			recoverableCondition.Status = corev1.ConditionTrue
			awsErr, _ := ackerr.AWSError(err)
			errorMessage := err.Error()
			if awsErr != nil {
				errorMessage = awsErr.Error()
			}
			recoverableCondition.Message = &errorMessage
		} else if recoverableCondition != nil {
			recoverableCondition.Status = corev1.ConditionFalse
			recoverableCondition.Message = nil
		}
	}
	// Required to avoid the "declared but not used" error in the default case
	_ = syncCondition
	if terminalCondition != nil || recoverableCondition != nil || syncCondition != nil {
		return &resource{ko}, true // updated
	}
	return nil, false // not updated
}

// terminalAWSError returns awserr, true; if the supplied error is an aws Error type
// and if the exception indicates that it is a Terminal exception
// 'Terminal' exception are specified in generator configuration
func (rm *resourceManager) terminalAWSError(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := ackerr.AWSError(err)
	if !ok {
		return false
	}
	switch awsErr.Code() {
	case "ValidationException":
		return true
	default:
		return false
	}
}

// getImmutableFieldChanges returns list of immutable fields from the
func (rm *resourceManager) getImmutableFieldChanges(
	delta *ackcompare.Delta,
) []string {
	var fields []string
	if delta.DifferentAt("Spec.workspaceID") {
		fields = append(fields, "workspaceID")
	}

	return fields
}
