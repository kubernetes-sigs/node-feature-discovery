/*
Copyright 2021 The Kubernetes Authors.

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

package resourcemonitor

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	topologyv1alpha2 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha2"
)

// Args stores commandline arguments used for resource monitoring
type Args struct {
	PodResourceSocketPath string
	SleepInterval         time.Duration
	Namespace             string
	KubeletConfigURI      string
	APIAuthTokenFile      string
}

// ResourceInfo stores information of resources and their corresponding IDs obtained from PodResource API
type ResourceInfo struct {
	Name        corev1.ResourceName
	Data        []string
	NumaNodeIds []int
}

// ContainerResources contains information about the node resources assigned to a container
type ContainerResources struct {
	Name      string
	Resources []ResourceInfo
}

// PodResources contains information about the node resources assigned to a pod
type PodResources struct {
	Name       string
	Namespace  string
	Containers []ContainerResources
}

// ResourcesScanner gathers all the PodResources from the system, using the podresources API client
type ResourcesScanner interface {
	Scan() ([]PodResources, error)
}

// ResourcesAggregator aggregates resource information based on the received data from underlying hardware and podresource API
type ResourcesAggregator interface {
	Aggregate(podResData []PodResources) topologyv1alpha2.ZoneList
}
