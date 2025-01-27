/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespace

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"

	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

// PatchLabels updates the given label for a specific namespace with a given value
func PatchLabels(name, key, value, operation string, ctx context.Context, f *framework.Framework) error {
	if operation != utils.JSONAddOperation && operation != utils.JSONRemoveOperation {
		return fmt.Errorf("unknown operation type, known values are %s, %s", utils.JSONAddOperation, utils.JSONRemoveOperation)
	}

	patches, err := json.Marshal(
		[]utils.JsonPatch{
			utils.NewJsonPatch(
				operation,
				"/metadata/labels",
				key,
				value,
			),
		},
	)
	if err != nil {
		return err
	}

	_, err = f.ClientSet.CoreV1().Namespaces().Patch(ctx, name, types.JSONPatchType, patches, metav1.PatchOptions{})
	return err
}
