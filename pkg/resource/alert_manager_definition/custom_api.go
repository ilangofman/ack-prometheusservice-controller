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
	"encoding/base64"
	b64 "encoding/base64"

	svcsdk "github.com/aws/aws-sdk-go/service/prometheusservice"
)

func customSetAlermManagerDefinitionInput(input *svcsdk.CreateAlertManagerDefinitionInput) {
	// Convert the data field to a base64 encoding

	println("Input string: ")
	println(string(input.Data))
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(input.Data)))
	b64.StdEncoding.Encode(dst, input.Data)
	input.Data = dst
	print("Ouput string: ", string(input.Data))

}
