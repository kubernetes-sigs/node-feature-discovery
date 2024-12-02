/*
Copyright 2024 The Kubernetes Authors.

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
package nfdmaster

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	k8sclient "k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

// NamespaceLister lists kubernetes namespaces.
type NamespaceLister struct {
	namespaceLister v1lister.NamespaceLister
	labelsSelector  labels.Selector
	stopChan        chan struct{}
}

func newNamespaceLister(k8sClient k8sclient.Interface, labelsSelector labels.Selector) (*NamespaceLister, error) {
	factory := informers.NewSharedInformerFactory(k8sClient, time.Hour)
	namespaceLister := factory.Core().V1().Namespaces().Lister()

	stopChan := make(chan struct{})
	factory.Start(stopChan) // runs in background

	start := time.Now()
	ret := factory.WaitForCacheSync(stopChan)
	for res, ok := range ret {
		if !ok {
			return &NamespaceLister{}, fmt.Errorf("namespace informer cache failed to sync (%s)", res)
		}
	}
	klog.InfoS("namespace informer cache synced", "duration", time.Since(start))

	return &NamespaceLister{
		namespaceLister: namespaceLister,
		labelsSelector:  labelsSelector,
		stopChan:        stopChan,
	}, nil
}

// list returns all kubernetes namespaces.
func (lister *NamespaceLister) list() ([]*corev1.Namespace, error) {
	return lister.namespaceLister.List(lister.labelsSelector)
}

// stop closes the channel used by the lister
func (lister *NamespaceLister) stop() {
	close(lister.stopChan)
}
