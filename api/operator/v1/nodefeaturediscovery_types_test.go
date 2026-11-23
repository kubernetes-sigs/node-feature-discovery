/*
Copyright 2026 The Kubernetes Authors.

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

package v1

import (
	"encoding/json"
	"testing"
)

// TestWorkerSecurityContextCarriesOnlyLSMFields checks the JSON contract of
// spec.operand.workerSecurityContext: the AppArmor profile and SELinux
// options round-trip, and securityContext fields that would weaken the
// worker's hardened context are dropped on decode.
func TestWorkerSecurityContextCarriesOnlyLSMFields(t *testing.T) {
	const doc = `{"spec":{"operand":{"workerSecurityContext":{
		"privileged": true,
		"allowPrivilegeEscalation": true,
		"readOnlyRootFilesystem": false,
		"runAsNonRoot": false,
		"capabilities": {"add": ["SYS_ADMIN"]},
		"seccompProfile": {"type": "Unconfined"},
		"appArmorProfile": {"type": "Localhost", "localhostProfile": "nfd-worker"},
		"seLinuxOptions": {"type": "nfd_worker_t", "level": "s0"}
	}}}}`

	var nfd NodeFeatureDiscovery
	if err := json.Unmarshal([]byte(doc), &nfd); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	got, err := json.Marshal(nfd.Spec.Operand)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	const want = `{"servicePort":0,"workerSecurityContext":{` +
		`"appArmorProfile":{"type":"Localhost","localhostProfile":"nfd-worker"},` +
		`"seLinuxOptions":{"type":"nfd_worker_t","level":"s0"}}}`
	if string(got) != want {
		t.Errorf("operand round-trip:\n got: %s\nwant: %s", got, want)
	}
}
