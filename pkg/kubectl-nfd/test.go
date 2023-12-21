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
	"fmt"
	"os"
	"time"

	k8sLabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/clientcmd"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	nfdinformers "sigs.k8s.io/node-feature-discovery/pkg/generated/informers/externalversions"

	"sigs.k8s.io/yaml"
)

func Test(nodefeaturerulepath, nodeName, kubeconfig string) []error {
	var errs []error
	var err error

	nfr := nfdv1alpha1.NodeFeatureRule{}

	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return []error{fmt.Errorf("error building kubeconfig: %w", err)}
	}

	nfdClient := nfdclientset.NewForConfigOrDie(config)
	informerFactory := nfdinformers.NewSharedInformerFactory(nfdClient, 1*time.Second)
	featureLister := informerFactory.Nfd().V1alpha1().NodeFeatures().Lister()

	sel := k8sLabels.SelectorFromSet(k8sLabels.Set{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodeName})
	objs, err := featureLister.List(sel)
	if err != nil {
		return []error{fmt.Errorf("failed to get NodeFeature resources for node %q: %w", nodeName, err)}
	}
	features := nfdv1alpha1.NewNodeFeatureSpec()
	if len(objs) > 0 {
		features = objs[0].Spec.DeepCopy()
		for _, o := range objs[1:] {
			s := o.Spec.DeepCopy()
			s.MergeInto(features)
		}
	}

	nfrFile, err := os.ReadFile(nodefeaturerulepath)
	if err != nil {
		return []error{fmt.Errorf("error reading NodeFeatureRule file: %w", err)}
	}

	err = yaml.Unmarshal(nfrFile, &nfr)
	if err != nil {
		return []error{fmt.Errorf("error parsing NodeFeatureRule: %w", err)}
	}

	errs = append(errs, processNodeFeatureRule(nfr, *features)...)

	return errs
}
