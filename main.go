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

func main() {
	//Setting-up K8S client
	cli, err := client.NewInCluster()
	if err != nil {
		log.Fatalf("Can't Get K8s Client:%v", err)
	}

	//Get the cpu features as strings
	features := cpuid.CPU.Features.Strings()

	//Get the pod name and namespace from the env variables
	podName := os.Getenv("POD_NAME")
	podns := os.Getenv("POD_NAMESPACE")

	//Get the pod object using the pod name and namespace
	pod, err := cli.Pods(podns).Get(podName)
	if err != nil {
		log.Fatalf("Can't Get Pod:%v", err)
	}

	//Get the node object using the pod name and namespace
	node, err := cli.Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		log.Fatalf("Can't Get Node:%v", err)
	}

	//Add each of the cpu feature as the node label
	for _, feature := range features {
		node.Labels["scheduler.alpha.intel.com/"+feature] = "true"
	}

	//If supported, add CMT, MBM and CAT features as a node label
	cmd := "/go/bin/mon-discovery"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT Monitoring:%v", err)
	}

	outString := string(out[:])
	if outString == "DETECTED" {
		node.Labels["scheduler.alpha.intel.com/RDTMON"] = "true"
	}

	cmd = "/go/bin/l3-alloc-discovery"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT L3 Allocation:%v", err)
	}

	outString = string(out[:])
	if outString == "DETECTED" {
		node.Labels["scheduler.alpha.intel.com/RDTL3CA"] = "true"
	}

	cmd = "/go/bin/l2-alloc-discovery"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Can't Dectect Support for RDT L2 Allocation:%v", err)
	}

	outString = string(out[:])
	if outString == "DETECTED" {
		node.Labels["scheduler.alpha.intel.com/RDTL2CA"] = "true"
	}

	//Update the node with the node labels
	_, err = cli.Nodes().Update(node)
	if err != nil {
		log.Fatalf("Can't Update Node:%v", err)
	}
}
