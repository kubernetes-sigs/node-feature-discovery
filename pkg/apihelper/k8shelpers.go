/*
Copyright 2019 The Kubernetes Authors.

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

package apihelper

import (
	"strings"

	api "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

// Implements APIHelpers
type K8sHelpers struct {
	AnnotationNs string
	LabelNs      string
}

func (h K8sHelpers) GetClient() (*k8sclient.Clientset, error) {
	// Set up an in-cluster K8S client.
	config, err := restclient.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := k8sclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func (h K8sHelpers) GetNode(cli *k8sclient.Clientset, nodeName string) (*api.Node, error) {
	// Get the node object using node name
	node, err := cli.Core().Nodes().Get(nodeName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return node, nil
}

// RemoveLabelsWithPrefix searches through all labels on Node n and removes
// any where the key contain the search string.
func (h K8sHelpers) RemoveLabelsWithPrefix(n *api.Node, search string) {
	for k := range n.Labels {
		if strings.Contains(k, search) {
			delete(n.Labels, k)
		}
	}
}

// RemoveLabels removes given NFD labels
func (h K8sHelpers) RemoveLabels(n *api.Node, labelNames []string) {
	for _, l := range labelNames {
		delete(n.Labels, h.LabelNs+l)
	}
}

func (h K8sHelpers) AddLabels(n *api.Node, labels map[string]string) {
	for k, v := range labels {
		n.Labels[h.LabelNs+k] = v
	}
}

// Add Annotations to the Node object
func (h K8sHelpers) AddAnnotations(n *api.Node, annotations map[string]string) {
	for k, v := range annotations {
		n.Annotations[h.AnnotationNs+k] = v
	}
}

func (h K8sHelpers) UpdateNode(c *k8sclient.Clientset, n *api.Node) error {
	// Send the updated node to the apiserver.
	_, err := c.Core().Nodes().Update(n)
	if err != nil {
		return err
	}

	return nil
}
