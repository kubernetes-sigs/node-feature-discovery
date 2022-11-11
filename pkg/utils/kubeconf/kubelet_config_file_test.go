/*
Copyright 2019-2021 The Kubernetes Authors.

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

package kubeconf

import (
	"path/filepath"
	"testing"
)

type testCaseData struct {
	path     string
	tmPolicy string
}

func TestGetKubeletConfigFromLocalFile(t *testing.T) {
	tCases := []testCaseData{
		{
			path:     filepath.Join("..", "..", "..", "test", "data", "kubeletconf.yaml"),
			tmPolicy: "single-numa-node",
		},
	}

	for _, tCase := range tCases {
		cfg, err := GetKubeletConfigFromLocalFile(tCase.path)
		if err != nil {
			t.Errorf("failed to read config from %q: %v", tCase.path, err)
		}
		if cfg.TopologyManagerPolicy != tCase.tmPolicy {
			t.Errorf("TM policy mismatch, found %q expected %q", cfg.TopologyManagerPolicy, tCase.tmPolicy)
		}
	}
}
