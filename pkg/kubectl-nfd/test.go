/*
Copyright 2023 The Kubernetes Authors.

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

package kubectlnfd

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sLabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/clientcmd"

	nfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func getNodeFeatures(nodeName, kubeconfig string) (*nfdv1alpha1.NodeFeatureSpec, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %w", err)
	}

	nfdClient := nfdclientset.NewForConfigOrDie(config)

	sel := k8sLabels.SelectorFromSet(k8sLabels.Set{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodeName})
	list, err := nfdClient.NfdV1alpha1().NodeFeatures(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, fmt.Errorf("failed to get NodeFeature resources for node %q: %w", nodeName, err)
	}
	objs := list.Items
	names := make([]string, len(objs))
	for i, o := range objs {
		names[i] = o.Namespace + "/" + o.Name
	}
	fmt.Printf("Found %d NodeFeature objects for node %q: %s\n", len(objs), nodeName, strings.Join(names, ", "))

	features := nfdv1alpha1.NewNodeFeatureSpec()
	if len(objs) > 0 {
		features = objs[0].Spec.DeepCopy()
		for _, o := range objs[1:] {
			s := o.Spec.DeepCopy()
			s.MergeInto(features)
		}
	}
	return features, nil
}

// Test reads a NodeFeatureRule or NodeFeatureGroup file and evaluates it against
// the NodeFeature objects of the given node. The kind is detected automatically.
func Test(resourcepath, nodeName, kubeconfig string) []error {
	features, err := getNodeFeatures(nodeName, kubeconfig)
	if err != nil {
		return []error{err}
	}

	t := parseRuleFile(resourcepath)
	switch o := t.(type) {
	case *nfdv1alpha1.NodeFeatureRule:
		return processNodeFeatureRule(*o, *features)
	case *nfdv1alpha1.NodeFeatureGroup:
		return processNodeFeatureGroup(*o, *features)
	default:
		return []error{fmt.Errorf("unsupported resource %v: must be NodeFeatureRule or NodeFeatureGroup", t)}
	}
}
