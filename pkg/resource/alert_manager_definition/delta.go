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
	"bytes"
	"reflect"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
)

// Hack to avoid import errors during build...
var (
	_ = &bytes.Buffer{}
	_ = &reflect.Method{}
)

// newResourceDelta returns a new `ackcompare.Delta` used to compare two
// resources
func newResourceDelta(
	a *resource,
	b *resource,
) *ackcompare.Delta {
	delta := ackcompare.NewDelta()
	if (a == nil && b != nil) ||
		(a != nil && b == nil) {
		delta.Add("", a, b)
		return delta
	}

	if !bytes.Equal(a.ko.Spec.Data, b.ko.Spec.Data) {
		delta.Add("Spec.Data", a.ko.Spec.Data, b.ko.Spec.Data)
	}
	if ackcompare.HasNilDifference(a.ko.Spec.WorkspaceID, b.ko.Spec.WorkspaceID) {
		delta.Add("Spec.WorkspaceID", a.ko.Spec.WorkspaceID, b.ko.Spec.WorkspaceID)
	} else if a.ko.Spec.WorkspaceID != nil && b.ko.Spec.WorkspaceID != nil {
		if *a.ko.Spec.WorkspaceID != *b.ko.Spec.WorkspaceID {
			delta.Add("Spec.WorkspaceID", a.ko.Spec.WorkspaceID, b.ko.Spec.WorkspaceID)
		}
	}
	if ackcompare.HasNilDifference(a.ko.Spec.AlertmanagerConfig, b.ko.Spec.AlertmanagerConfig) {
		delta.Add("Spec.AlertmanagerConfig", a.ko.Spec.AlertmanagerConfig, b.ko.Spec.AlertmanagerConfig)
	} else if a.ko.Spec.AlertmanagerConfig != nil && b.ko.Spec.AlertmanagerConfig != nil {
		if *a.ko.Spec.AlertmanagerConfig != *b.ko.Spec.AlertmanagerConfig {
			delta.Add("Spec.AlertmanagerConfig", a.ko.Spec.AlertmanagerConfig, b.ko.Spec.AlertmanagerConfig)
		}
	}

	return delta
}
