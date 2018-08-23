/*
Copyright 2018 The Kubernetes Authors.

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

package e2e

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	configPath  = flag.String("config", path.Join(os.Getenv("HOME"), "/.kube/config"), "Kubeconfig to use")
	namespace   = flag.String("namespace", "default", "K8s namespace to use")
	dockerRepo  = flag.String("repo", "quay.io/kubernetes_incubator/node-feature-discovery", "Docker repository to fetch image from")
	dockerTag   = flag.String("tag", "e2e-test", "Docker tag to use")
	labelPrefix = "node.alpha.kubernetes-incubator.io/nfd"
)

func createClient() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", *configPath)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func findNode(cs *kubernetes.Clientset) (*v1.Node, error) {
	nodes, _ := cs.CoreV1().Nodes().List(metav1.ListOptions{})

NodeLoop:
	for _, node := range nodes.Items {
		// Check taints
		for _, taint := range node.Spec.Taints {
			if taint.Effect == v1.TaintEffectNoSchedule || taint.Effect == v1.TaintEffectNoExecute {
				continue NodeLoop
			}
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
				return &node, nil
			}
		}
	}

	return nil, fmt.Errorf("No schedulable and ready node was found")
}

func deleteNodeLabels(cs *kubernetes.Clientset, name string) error {
	// Get node
	node, err := cs.CoreV1().Nodes().Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// Delete nfd labels
	for k, _ := range node.Labels {
		if strings.HasPrefix(k, labelPrefix) {
			delete(node.Labels, k)
		}
	}
	// Update node with the new set of labels
	_, err = cs.Core().Nodes().Update(node)
	if err != nil {
		return err
	}

	return nil
}

func createPod(t *testing.T, cs *kubernetes.Clientset, nodeName string) (*v1.Pod, error) {
	podName := "node-feature-discovery-" + string(uuid.NewUUID())
	image := fmt.Sprintf("%s:%s", *dockerRepo, *dockerTag)
	t.Logf("Trying to create Pod %s on Node %s, Image %s", podName, nodeName, image)
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "node-feature-discovery",
					Image: image,
					Args:  []string{"--oneshot", "--sources=fake"},
					Env: []v1.EnvVar{
						{
							Name: "NODE_NAME",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						},
					},
					ImagePullPolicy: v1.PullAlways,
				},
			},
			RestartPolicy:      v1.RestartPolicyNever,
			ServiceAccountName: "node-feature-discovery",
			NodeName:           nodeName,
			HostNetwork:        true,
		},
	}
	return cs.Core().Pods(*namespace).Create(pod)
}

func waitForPod(cs *kubernetes.Clientset, podName string) error {
	// Use 2 minute timeout, poll every second
	return wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
		pod, err := cs.CoreV1().Pods(*namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if pod.Status.Phase == v1.PodSucceeded {
			return true, nil
		} else if pod.Status.Phase == v1.PodFailed {
			return false, fmt.Errorf("Pod failed")
		}
		return false, nil
	})
}

func TestNFD(t *testing.T) {
	fakeFeatureLabels := map[string]string{
		fmt.Sprintf("%s-%s-fakefeature1", labelPrefix, "fake"): "true",
		fmt.Sprintf("%s-%s-fakefeature2", labelPrefix, "fake"): "true",
		fmt.Sprintf("%s-%s-fakefeature3", labelPrefix, "fake"): "true",
	}

	cs, err := createClient()
	if err != nil {
		t.Fatalf("Failed to initialize client interface: %v", err)
	}

	node, err := findNode(cs)
	if err != nil {
		t.Fatal("Failed to find schedulable node")
	}

	t.Logf("Removing existing nfd labels\n")
	err = deleteNodeLabels(cs, node.Name)
	if err != nil {
		t.Fatalf("Failed to remove node labels: %v", err)
	}

	pod, err := createPod(t, cs, node.Name)
	if err != nil {
		t.Fatalf("Failed to create NFD pod: %v\n", err)
	}

	t.Logf("Waiting for pod to finish\n")
	err = waitForPod(cs, pod.Name)
	if err != nil {
		t.Errorf("%v\n", err)
	}

	t.Logf("Deleting NFD pod\n")
	err = cs.Core().Pods(*namespace).Delete(pod.Name, nil)
	if err != nil {
		t.Fatalf("Failed to delete pod: %v\n", err)
	}

	options := metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set(fakeFeatureLabels)).String()}
	matchedNodes, err := cs.CoreV1().Nodes().List(options)
	if err != nil {
		t.Fatalf("Failed to list nodes: %v\n", err)
	}

	if len(matchedNodes.Items) != 1 {
		t.Fatalf("Found %v nodes matching fake labels, expecting 1", len(matchedNodes.Items))
	}
	if matchedNodes.Items[0].Name != node.Name {
		t.Fatalf("Labels found on wrong node (%v instead of %v)", matchedNodes.Items[0].Name, node.Name)
	}

	t.Logf("Removing nfd node labels\n")
	err = deleteNodeLabels(cs, node.Name)
	if err != nil {
		t.Errorf("Failed to remove node labels: %v", err)
	}
}
