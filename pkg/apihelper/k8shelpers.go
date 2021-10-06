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
	"context"
	"encoding/json"

	api "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	kubeletconfigscheme "k8s.io/kubernetes/pkg/kubelet/apis/config/scheme"

	topologyclientset "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/generated/clientset/versioned"
	"github.com/pkg/errors"
)

// Implements APIHelpers
type K8sHelpers struct {
	Kubeconfig string
}

func (h K8sHelpers) GetClient() (*k8sclient.Clientset, error) {
	// Set up an in-cluster K8S client.
	var config *restclient.Config
	var err error

	if h.Kubeconfig == "" {
		config, err = restclient.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", h.Kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	clientset, err := k8sclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func (h K8sHelpers) GetTopologyClient() (*topologyclientset.Clientset, error) {
	// Set up an in-cluster K8S client.
	var config *restclient.Config
	var err error

	if h.Kubeconfig == "" {
		config, err = restclient.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", h.Kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	topologyClient, err := topologyclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return topologyClient, nil
}

func (h K8sHelpers) GetNode(cli *k8sclient.Clientset, nodeName string) (*api.Node, error) {
	// Get the node object using node name
	node, err := cli.CoreV1().Nodes().Get(context.TODO(), nodeName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (h K8sHelpers) GetNodes(cli *k8sclient.Clientset) (*api.NodeList, error) {
	return cli.CoreV1().Nodes().List(context.TODO(), meta_v1.ListOptions{})
}

func (h K8sHelpers) UpdateNode(c *k8sclient.Clientset, n *api.Node) error {
	// Send the updated node to the apiserver.
	_, err := c.CoreV1().Nodes().Update(context.TODO(), n, meta_v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (h K8sHelpers) PatchNode(c *k8sclient.Clientset, nodeName string, patches []JsonPatch) error {
	if len(patches) > 0 {
		data, err := json.Marshal(patches)
		if err == nil {
			_, err = c.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, data, meta_v1.PatchOptions{})
		}
		return err
	}
	return nil
}

func (h K8sHelpers) PatchNodeStatus(c *k8sclient.Clientset, nodeName string, patches []JsonPatch) error {
	if len(patches) > 0 {
		data, err := json.Marshal(patches)
		if err == nil {
			_, err = c.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, data, meta_v1.PatchOptions{}, "status")
		}
		return err
	}
	return nil

}

func (h K8sHelpers) GetPod(cli *k8sclient.Clientset, namespace string, podName string) (*api.Pod, error) {
	// Get the node object using pod name
	pod, err := cli.CoreV1().Pods(namespace).Get(context.TODO(), podName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func (h K8sHelpers) GetKubeletConfig(c *k8sclient.Clientset, nodeName string) (*kubeletconfig.KubeletConfiguration, error) {
	//request here is the same as: curl https://$APISERVER:6443/api/v1/nodes/$NODE_NAME/proxy/configz
	res := c.CoreV1().RESTClient().Get().Resource("nodes").Name(nodeName).SubResource("proxy").Suffix("configz").Do(context.TODO())
	return decodeConfigz(&res)
}

// Decodes the rest result from /configz and returns a kubeletconfig.KubeletConfiguration (internal type).
func decodeConfigz(res *restclient.Result) (*kubeletconfig.KubeletConfiguration, error) {
	// This hack because /configz reports the following structure:
	// {"kubeletconfig": {the JSON representation of kubeletconfigv1beta1.KubeletConfiguration}}
	type configzWrapper struct {
		ComponentConfig kubeletconfigv1beta1.KubeletConfiguration `json:"kubeletconfig"`
	}

	configz := configzWrapper{}
	kubeCfg := kubeletconfig.KubeletConfiguration{}

	contentsBytes, err := res.Raw()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(contentsBytes, &configz)
	if err != nil {
		return nil, errors.Wrap(err, string(contentsBytes))
	}

	scheme, _, err := kubeletconfigscheme.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	err = scheme.Convert(&configz.ComponentConfig, &kubeCfg, nil)
	if err != nil {
		return nil, err
	}

	return &kubeCfg, nil
}
