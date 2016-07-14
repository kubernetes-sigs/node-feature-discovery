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
	"log"
	"os"
	"os/exec"

	"github.com/klauspost/cpuid"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	version = "0.1.0"
)

func main() {
	// Setting-up a logger file
	fd, err := os.OpenFile("dbi-iafeature-discovery.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Can't Open File for Logging: %v", err)
	}
	defer fd.Close()
	log.SetOutput(fd)

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

	// Get the cpu features as strings
	features := cpuid.CPU.Features.Strings()
	log.Printf("CPU Features Detected from cpuid: %s\n", features)
	// Add each of the cpu feature as the node label
	for _, feature := range features {
		node.Labels["node.alpha.intel.com/v"+version+"-cpu-"+feature] = "true"
	}

	// If supported, add CMT, MBM and CAT features as a node label
	cmd := "/go/src/github.com/intelsdi-x/dbi-iafeature-discovery/rdt-discovery/mon-discovery"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT Monitoring: %v", err)
	}

	outString := string(out[:])
	if outString == "DETECTED" {
		node.Labels["node.alpha.intel.com/v"+version+"-cpu-RDTMON"] = "true"
	}
	log.Printf("RDT Monitoring Detected\n")

	cmd = "/go/src/github.com/intelsdi-x/dbi-iafeature-discovery/rdt-discovery/l3-alloc-discovery"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT L3 Allocation: %v", err)
	}

	outString = string(out[:])
	if outString == "DETECTED" {
		node.Labels["node.alpha.intel.com/v"+version+"-cpu-RDTL3CA"] = "true"
	}
	log.Printf("RDT L3 Cache Allocation Detected\n")

	cmd = "/go/src/github.com/intelsdi-x/dbi-iafeature-discovery/rdt-discovery/l2-alloc-discovery"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT L2 Allocation: %v", err)
	}

	outString = string(out[:])
	if outString == "DETECTED" {
		node.Labels["node.alpha.intel.com/v"+version+"-cpu-RDTL2CA"] = "true"
	}
	log.Printf("RDT L2 Cache Allocation Detected\n")

	// Update the node with the node labels
	_, err = cli.Nodes().Update(node)
	if err != nil {
		log.Fatalf("Can't Update Node: %v", err)
	}
}
