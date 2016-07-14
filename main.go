/*
Copyright 2016 Intel Corporation
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

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/klauspost/cpuid"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

const namespace = "node.alpha.intel.com"

var version = "" // Must not be const, set using ldflags at build time
var prefix = fmt.Sprintf("%s/%s", namespace, version)

func main() {
	// Assert that the version is known
	if version == "" {
		log.Fatalf("`main.version` not set! Set -ldflags '-X main.version `git describe --tags --dirty --always`' during build or run.")
	}
	log.Printf("Version: [%s]", version)
	log.Printf("Label prefix: [%s]", prefix)

	// Setting-up K8S client
	cli, err := client.NewInCluster()
	if err != nil {
		log.Fatalf("Can't Get K8s Client: %v", err)
	}

	// Get the pod name and namespace from the env variables
	podName := os.Getenv("POD_NAME")
	podns := os.Getenv("POD_NAMESPACE")
	log.Printf("Pod Name ENV Variable: %s\n", podName)
	log.Printf("Pod Namespace ENV Variable: %s\n", podns)

	// Get the pod object using the pod name and namespace
	pod, err := cli.Pods(podns).Get(podName)
	if err != nil {
		log.Fatalf("Can't Get Pod: %v", err)
	}

	// Get the node object using the pod name and namespace
	node, err := cli.Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		log.Fatalf("Can't Get Node: %v", err)
	}

	// Add the version of this discovery code as a node label
	node.Labels[fmt.Sprintf("%s/dbi-ia-feature-discovery.version", prefix)] = version

	// Get the cpu features as strings
	features := cpuid.CPU.Features.Strings()
	log.Printf("CPU Features Detected from cpuid: %s\n", features)
	// Add each of the cpu feature as the node label
	for _, feature := range features {
		node.Labels[fmt.Sprintf("%s-cpu-%s", prefix, feature)] = "true"
	}

	// If supported, add CMT, MBM and CAT features as a node label
	cmd := "/go/src/github.com/intelsdi-x/dbi-iafeature-discovery/rdt-discovery/mon-discovery"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT Monitoring: %v", err)
	}

	outString := string(out[:])
	if outString == "DETECTED" {
		node.Labels[fmt.Sprintf("%s-cpu-RDTMON", prefix)] = "true"
		log.Printf("RDT Monitoring Detected\n")
	}

	cmd = "/go/src/github.com/intelsdi-x/dbi-iafeature-discovery/rdt-discovery/l3-alloc-discovery"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT L3 Allocation: %v", err)
	}

	outString = string(out[:])
	if outString == "DETECTED" {
		node.Labels[fmt.Sprintf("%s-cpu-RDTL3CA", prefix)] = "true"
		log.Printf("RDT L3 Cache Allocation Detected\n")
	}

	cmd = "/go/src/github.com/intelsdi-x/dbi-iafeature-discovery/rdt-discovery/l2-alloc-discovery"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT L2 Allocation: %v", err)
	}

	outString = string(out[:])
	if outString == "DETECTED" {
		node.Labels[fmt.Sprintf("%s-cpu-RDTL2CA", prefix)] = "true"
		log.Printf("RDT L2 Cache Allocation Detected\n")
	}

	// If turbo boost is enabled, add it as a node label
	cmd = "cat /sys/devices/system/cpu/intel_pstate/no_turbo"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect if Turbo Boost is Enabled: %v", err)
	}

	outString = string(out[:])
	if outString == "0\n" {
		node.Labels[fmt.Sprintf("%s-cpu-turbo", prefix)] = "true"
		log.Printf("Turbo Boost is Enabled\n")
	}

	// Update the node with the node labels
	_, err = cli.Nodes().Update(node)
	if err != nil {
		log.Fatalf("Can't Update Node: %v", err)
	}
}
