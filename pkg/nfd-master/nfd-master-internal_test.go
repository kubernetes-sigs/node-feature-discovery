/*
Copyright 2019-2021 The Kubernetes Authors.

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
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	fakecorev1client "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	nfdclientset "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned"
	fakenfdclient "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/fake"
	nfdscheme "sigs.k8s.io/node-feature-discovery/api/generated/clientset/versioned/scheme"
	nfdinformers "sigs.k8s.io/node-feature-discovery/api/generated/informers/externalversions"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/features"
	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

const (
	testNodeName = "mock-node"
)

func newTestNode() *corev1.Node {
	n := corev1.Node{}
	n.Name = testNodeName
	n.Labels = map[string]string{}
	n.Annotations = map[string]string{}
	n.Status.Capacity = corev1.ResourceList{"cpu": resource.MustParse("2")}
	return &n
}

func newFakeNfdAPIController(client *fakenfdclient.Clientset) *nfdController {
	c := &nfdController{
		stopChan:           make(chan struct{}),
		updateAllNodesChan: make(chan struct{}, 1),
		updateOneNodeChan:  make(chan string),
	}

	informerFactory := nfdinformers.NewSharedInformerFactory(client, 1*time.Hour)

	// Add informer for NodeFeature objects
	featureInformer := informerFactory.Nfd().V1alpha1().NodeFeatures()
	if _, err := featureInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) {},
		UpdateFunc: func(oldObj, newObj interface{}) {},
		DeleteFunc: func(obj interface{}) {},
	}); err != nil {
		return nil
	}
	c.featureLister = featureInformer.Lister()

	// Add informer for NodeFeatureRule objects
	ruleInformer := informerFactory.Nfd().V1alpha1().NodeFeatureRules()
	if _, err := ruleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(object interface{}) {},
		UpdateFunc: func(oldObject, newObject interface{}) {},
		DeleteFunc: func(object interface{}) {},
	}); err != nil {
		return nil
	}
	c.ruleLister = ruleInformer.Lister()

	// Start informers
	informerFactory.Start(c.stopChan)

	utilruntime.Must(nfdv1alpha1.AddToScheme(nfdscheme.Scheme))

	return c
}

func withNodeName(nodeName string) NfdMasterOption {
	return &nfdMasterOpt{f: func(n *nfdMaster) { n.nodeName = nodeName }}
}

func withConfig(config *NFDConfig) NfdMasterOption {
	return &nfdMasterOpt{f: func(n *nfdMaster) { n.config = config }}
}

// withNFDClient forces to use the given client for the NFD API, without
// initializing one from kubeconfig.
func withNFDClient(cli nfdclientset.Interface) NfdMasterOption {
	return &nfdMasterOpt{f: func(n *nfdMaster) { n.nfdClient = cli }}
}

func newFakeMaster(opts ...NfdMasterOption) *nfdMaster {
	nfdCli := fakenfdclient.NewSimpleClientset()
	//nolint:staticcheck // See issue #2400 for migration to NewClientset
	k8sCli := fakeclient.NewSimpleClientset()
	defaultOpts := []NfdMasterOption{
		withNodeName(testNodeName),
		withConfig(&NFDConfig{Restrictions: Restrictions{AllowOverwrite: true}}),
		WithKubernetesClient(k8sCli),
		withNFDClient(nfdCli),
	}
	m, err := NewNfdMaster(append(defaultOpts, opts...)...)
	if err != nil {
		panic(err)
	}
	// Add FeatureGates
	if err := features.NFDMutableFeatureGate.Add(features.DefaultNFDFeatureGates); err != nil {
		panic(err)
	}
	return m.(*nfdMaster)
}

func newFakeMasterWithFeatureGate(opts ...NfdMasterOption) *nfdMaster {
	nfdCli := fakenfdclient.NewSimpleClientset()
	//nolint:staticcheck // See issue #2400 for migration to NewClientset
	k8sCli := fakeclient.NewSimpleClientset()
	defaultOpts := []NfdMasterOption{
		withNodeName(testNodeName),
		withConfig(&NFDConfig{Restrictions: Restrictions{AllowOverwrite: true}}),
		WithKubernetesClient(k8sCli),
		withNFDClient(nfdCli),
	}
	m, err := NewNfdMaster(append(defaultOpts, opts...)...)
	if err != nil {
		panic(err)
	}
	// Add FeatureGates
	if err := features.NFDMutableFeatureGate.Add(features.DefaultNFDFeatureGates); err != nil {
		panic(err)
	}
	if err := features.NFDMutableFeatureGate.Set("DisableAutoPrefix=true"); err != nil {
		panic(err)
	}
	// Enable DisableAutoPrefix feature gate
	if !features.NFDFeatureGate.Enabled(features.DisableAutoPrefix) {
		err = errors.New("DisableAutoPrefix feature gate is not enabled")
		panic(err)
	}
	return m.(*nfdMaster)
}

func TestUpdateNodeObject(t *testing.T) {
	Convey("When I update the node using fake client", t, func() {
		featureLabels := map[string]string{
			nfdv1alpha1.FeatureLabelNs + "/source-feature.1": "1",
			nfdv1alpha1.FeatureLabelNs + "/source-feature.2": "2",
			nfdv1alpha1.FeatureLabelNs + "/source-feature.3": "val3",
			nfdv1alpha1.ProfileLabelNs + "/profile-a":        "val4",
		}
		featureAnnotations := map[string]string{
			"feature.node.kubernetesl.io/my-annotation": "my-val",
		}
		featureExtResources := map[string]string{
			nfdv1alpha1.FeatureLabelNs + "/source-feature.1": "1",
			nfdv1alpha1.FeatureLabelNs + "/source-feature.2": "2",
		}

		featureLabelNames := make([]string, 0, len(featureLabels))
		for k := range featureLabels {
			featureLabelNames = append(featureLabelNames, strings.TrimPrefix(k, nfdv1alpha1.FeatureLabelNs+"/"))
		}
		sort.Strings(featureLabelNames)

		featureAnnotationNames := make([]string, 0, len(featureLabels))
		for k := range featureAnnotations {
			featureAnnotationNames = append(featureAnnotationNames, strings.TrimPrefix(k, nfdv1alpha1.FeatureAnnotationNs+"/"))
		}
		sort.Strings(featureAnnotationNames)

		featureExtResourceNames := make([]string, 0, len(featureExtResources))
		for k := range featureExtResources {
			featureExtResourceNames = append(featureExtResourceNames, strings.TrimPrefix(k, nfdv1alpha1.FeatureLabelNs+"/"))
		}
		sort.Strings(featureExtResourceNames)

		// Create a node with some existing features
		testNode := newTestNode()
		testNode.Labels[nfdv1alpha1.FeatureLabelNs+"/old-feature"] = "old-value"
		testNode.Annotations[nfdv1alpha1.AnnotationNs+"/feature-labels"] = "old-feature"

		// Create fake api client and initialize NfdMaster instance
		//nolint:staticcheck // See issue #2400 for migration to NewClientset
		fakeCli := fakeclient.NewSimpleClientset(testNode)
		fakeMaster := newFakeMaster(WithKubernetesClient(fakeCli))

		Convey("When I successfully update the node with feature labels", func() {
			err := fakeMaster.updateNodeObject(fakeCli, testNode, featureLabels, featureAnnotations, featureExtResources, nil, false)
			Convey("Error is nil", func() {
				So(err, ShouldBeNil)
			})

			Convey("Node object is updated", func() {
				expectedAnnotations := map[string]string{
					nfdv1alpha1.FeatureLabelsAnnotation:              strings.Join(featureLabelNames, ","),
					nfdv1alpha1.FeatureAnnotationsTrackingAnnotation: strings.Join(featureAnnotationNames, ","),
					nfdv1alpha1.ExtendedResourceAnnotation:           strings.Join(featureExtResourceNames, ","),
				}
				maps.Copy(expectedAnnotations, featureAnnotations)

				expectedCapacity := testNode.Status.Capacity.DeepCopy()
				for k, v := range featureExtResources {
					expectedCapacity[corev1.ResourceName(k)] = resource.MustParse(v)
				}

				// Get the node
				updatedNode, err := fakeCli.CoreV1().Nodes().Get(context.TODO(), testNodeName, metav1.GetOptions{})

				So(err, ShouldBeNil)
				So(updatedNode.Labels, ShouldEqual, featureLabels)
				So(updatedNode.Annotations, ShouldEqual, expectedAnnotations)
				So(updatedNode.Status.Capacity, ShouldEqual, expectedCapacity)
			})
		})

		Convey("When I fail to patch a node", func() {
			fakeCli.CoreV1().(*fakecorev1client.FakeCoreV1).PrependReactor("patch", "nodes", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &corev1.Node{}, errors.New("Fake error when patching node")
			})
			err := fakeMaster.updateNodeObject(fakeCli, testNode, nil, featureAnnotations, ExtendedResources{"": ""}, nil, false)

			Convey("Error is produced", func() {
				So(err, ShouldBeError)
			})
		})

	})
}

func TestUpdateMasterNode(t *testing.T) {
	Convey("When updating the nfd-master node", t, func() {
		testNode := newTestNode()
		testNode.Annotations["nfd.node.kubernetes.io/master.version"] = "foo"

		Convey("When update operation succeeds", func() {
			//nolint:staticcheck // See issue #2400 for migration to NewClientset
			fakeCli := fakeclient.NewSimpleClientset(testNode)
			fakeMaster := newFakeMaster(WithKubernetesClient(fakeCli))
			err := fakeMaster.updateMasterNode()
			Convey("No error should be returned", func() {
				So(err, ShouldBeNil)
			})

			Convey("Master version annotation was removed", func() {
				updatedNode, err := fakeCli.CoreV1().Nodes().Get(context.TODO(), testNodeName, metav1.GetOptions{})
				So(err, ShouldBeNil)
				So(updatedNode.Annotations, ShouldBeEmpty)
			})
		})

		Convey("When getting API node object fails", func() {
			//nolint:staticcheck // See issue #2400 for migration to NewClientset
			fakeCli := fakeclient.NewSimpleClientset(testNode)
			fakeMaster := newFakeMaster(WithKubernetesClient(fakeCli))
			fakeMaster.nodeName = "does-not-exist"

			err := fakeMaster.updateMasterNode()
			Convey("An error should be returned", func() {
				So(err, ShouldBeError)
			})
		})

		Convey("When updating node object fails", func() {
			fakeErr := errors.New("Fake error when patching node")
			//nolint:staticcheck // See issue #2400 for migration to NewClientset
			fakeCli := fakeclient.NewSimpleClientset(testNode)
			fakeCli.CoreV1().(*fakecorev1client.FakeCoreV1).PrependReactor("patch", "nodes", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &corev1.Node{}, fakeErr
			})
			fakeMaster := newFakeMaster(WithKubernetesClient(fakeCli))

			err := fakeMaster.updateMasterNode()
			Convey("An error should be returned", func() {
				So(err, ShouldWrap, fakeErr)
			})
		})
	})
}

func TestAddingExtResources(t *testing.T) {
	Convey("When adding extended resources", t, func() {
		fakeMaster := newFakeMaster()
		Convey("When there are no matching labels", func() {
			testNode := newTestNode()
			extendedResources := ExtendedResources{}
			patches := fakeMaster.createExtendedResourcePatches(testNode, extendedResources)
			So(len(patches), ShouldEqual, 0)
		})

		Convey("When there are matching labels", func() {
			testNode := newTestNode()
			extendedResources := ExtendedResources{"feature-1": "1", "feature-2": "2"}
			expectedPatches := []utils.JsonPatch{
				utils.NewJsonPatch("add", "/status/capacity", "feature-1", "1"),
				utils.NewJsonPatch("add", "/status/capacity", "feature-2", "2"),
			}
			patches := fakeMaster.createExtendedResourcePatches(testNode, extendedResources)
			So(sortJsonPatches(patches), ShouldResemble, sortJsonPatches(expectedPatches))
		})

		Convey("When the resource already exists", func() {
			testNode := newTestNode()
			testNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			extendedResources := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-1": "1"}
			patches := fakeMaster.createExtendedResourcePatches(testNode, extendedResources)
			So(len(patches), ShouldEqual, 0)
		})

		Convey("When the resource already exists but its capacity has changed", func() {
			testNode := newTestNode()
			testNode.Status.Capacity[corev1.ResourceName("feature-1")] = *resource.NewQuantity(2, resource.BinarySI)
			extendedResources := ExtendedResources{"feature-1": "1"}
			expectedPatches := []utils.JsonPatch{
				utils.NewJsonPatch("replace", "/status/capacity", "feature-1", "1"),
				utils.NewJsonPatch("replace", "/status/allocatable", "feature-1", "1"),
			}
			patches := fakeMaster.createExtendedResourcePatches(testNode, extendedResources)
			So(sortJsonPatches(patches), ShouldResemble, sortJsonPatches(expectedPatches))
		})
	})
}

func TestRemovingExtResources(t *testing.T) {
	Convey("When removing extended resources", t, func() {
		fakeMaster := newFakeMaster()
		Convey("When none are removed", func() {
			testNode := newTestNode()
			extendedResources := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-1": "1", nfdv1alpha1.FeatureLabelNs + "/feature-2": "2"}
			testNode.Annotations[nfdv1alpha1.AnnotationNs+"/extended-resources"] = "feature-1,feature-2"
			testNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			testNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			patches := fakeMaster.createExtendedResourcePatches(testNode, extendedResources)
			So(len(patches), ShouldEqual, 0)
		})
		Convey("When the related label is gone", func() {
			testNode := newTestNode()
			extendedResources := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-4": "", nfdv1alpha1.FeatureLabelNs + "/feature-2": "2"}
			testNode.Annotations[nfdv1alpha1.AnnotationNs+"/extended-resources"] = "feature-4,feature-2"
			testNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-4")] = *resource.NewQuantity(4, resource.BinarySI)
			testNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			patches := fakeMaster.createExtendedResourcePatches(testNode, extendedResources)
			So(len(patches), ShouldBeGreaterThan, 0)
		})
		Convey("When the extended resource is no longer wanted", func() {
			testNode := newTestNode()
			testNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-1")] = *resource.NewQuantity(1, resource.BinarySI)
			testNode.Status.Capacity[corev1.ResourceName(nfdv1alpha1.FeatureLabelNs+"/feature-2")] = *resource.NewQuantity(2, resource.BinarySI)
			extendedResources := ExtendedResources{nfdv1alpha1.FeatureLabelNs + "/feature-2": "2"}
			testNode.Annotations[nfdv1alpha1.AnnotationNs+"/extended-resources"] = "feature-1,feature-2"
			patches := fakeMaster.createExtendedResourcePatches(testNode, extendedResources)
			So(len(patches), ShouldBeGreaterThan, 0)
		})
	})
}

func TestFilterLabels(t *testing.T) {
	fakeMaster := newFakeMaster()
	fakeMaster.config.ExtraLabelNs = map[string]struct{}{"example.io": {}}
	fakeMaster.deniedNs = deniedNs{
		normal:   map[string]struct{}{"": {}, "kubernetes.io": {}, "denied.ns": {}},
		wildcard: map[string]struct{}{".kubernetes.io": {}, ".denied.subns": {}},
	}

	type TC struct {
		description          string
		labelName            string
		labelValue           string
		features             nfdv1alpha1.Features
		expectErr            bool
		expectedValue        string
		expectedExtResources ExtendedResources
		expectedAnnotations  map[string]string
	}

	tcs := []TC{
		{
			description:   "Static value",
			labelName:     "example.io/test",
			labelValue:    "test-val",
			expectedValue: "test-val",
		},
		{
			description: "Dynamic value",
			labelName:   "example.io/testLabel",
			labelValue:  "@test.feature.LSM",
			features: nfdv1alpha1.Features{
				Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{
					"test.feature": {
						Elements: map[string]string{
							"LSM": "123",
						},
					},
				},
			},
			expectedValue: "123",
		},
		{
			description: "Unprefixed should be denied",
			labelName:   "test-label",
			labelValue:  "test-value",
			expectErr:   true,
		},
		{
			description: "kubernetes.io ns should be denied",
			labelName:   "kubernetes.io/test-label",
			labelValue:  "test-value",
			expectErr:   true,
		},
		{
			description: "*.kubernetes.io ns should be denied",
			labelName:   "sub.ns.kubernetes.io/test-label",
			labelValue:  "test-value",
			expectErr:   true,
		},
		{
			description: "denied.ns ns should be denied",
			labelName:   "denied.ns/test-label",
			labelValue:  "test-value",
			expectErr:   true,
		},
		{
			description: "*.denied.subns ns should be denied",
			labelName:   "my.denied.subns/test-label",
			labelValue:  "test-value",
			expectErr:   true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			labelValue, err := fakeMaster.filterFeatureLabel(tc.labelName, tc.labelValue, &tc.features)

			if tc.expectErr {
				Convey("Label should be filtered out", t, func() {
					So(err, ShouldBeError)
				})
			} else {
				Convey("Label should not be filtered out", t, func() {
					So(err, ShouldBeNil)
				})
				Convey("Label value should be correct", t, func() {
					So(labelValue, ShouldEqual, tc.expectedValue)
				})
			}
		})
	}

	tcs = []TC{
		{
			description:          "Unprefixed extended resources & annotations should not be allowed",
			expectedExtResources: ExtendedResources{},
			expectedAnnotations:  map[string]string{},
		},
	}

	extendedResources := ExtendedResources{"micromicrowaves": "10", "tooster": "5"}
	prefixlessAnnotation := map[string]string{"test-annotation": "bar"}

	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			outExtendedResources := fakeMaster.filterExtendedResources(&tc.features, extendedResources)
			Convey("Unprefixed extended resources should not be allowed", t, func() {
				So(outExtendedResources, ShouldEqual, tc.expectedExtResources)
			})
		})
	}

	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			filteredAnnotations := fakeMaster.filterFeatureAnnotations(prefixlessAnnotation)
			Convey("Unprefixed annotation should not be allowed", t, func() {
				So(filteredAnnotations, ShouldEqual, tc.expectedAnnotations)
			})
		})
	}

	// Create a new fake master with the feature gate enabled
	fakeMaster = newFakeMasterWithFeatureGate()
	tcs = []TC{
		{
			description:   "Unprefixed label & annotation should be allowed",
			labelName:     "test-label",
			labelValue:    "test-value",
			expectedValue: "test-value",
			expectedAnnotations: map[string]string{
				"test-annotation": "bar",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			testPrefixlessAnnotation := map[string]string{
				"test-annotation": "bar",
			}

			labelValue, err := fakeMaster.filterFeatureLabel(tc.labelName, tc.labelValue, &tc.features)
			filteredAnnotations := fakeMaster.filterFeatureAnnotations(testPrefixlessAnnotation)

			Convey("Label should not be filtered out", t, func() {
				So(err, ShouldBeNil)
			})
			Convey("Label value should be correct", t, func() {
				So(labelValue, ShouldEqual, tc.expectedValue)
			})
			Convey("Unprefixed annotation should be allowed", t, func() {
				So(filteredAnnotations, ShouldEqual, tc.expectedAnnotations)
			})
		})
	}

	tcs = []TC{
		{
			description: "Unprefixed extended resources should be allowed",
			expectedExtResources: ExtendedResources{
				"micromicrowaves": "10",
				"tooster":         "5",
			},
		},
	}

	extendedResources = ExtendedResources{"micromicrowaves": "10", "tooster": "5"}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			outExtendedResources := fakeMaster.filterExtendedResources(&tc.features, extendedResources)
			Convey("Unprefixed extended resources should be allowed", t, func() {
				So(outExtendedResources, ShouldEqual, tc.expectedExtResources)
			})
		})
	}
}

func TestCreatePatches(t *testing.T) {
	Convey("When creating JSON patches", t, func() {
		existingItems := map[string]string{"key-1": "val-1", "key-2": "val-2", "key-3": "val-3"}
		overwriteKeys := true
		jsonPath := "/root"

		Convey("When there are neither itmes to remoe nor to add or update", func() {
			p := createPatches(sets.New([]string{"foo", "bar"}...), existingItems, map[string]string{}, jsonPath, overwriteKeys)
			So(len(p), ShouldEqual, 0)
		})

		Convey("When there are itmes to remoe but none to add or update", func() {
			p := createPatches(sets.New([]string{"key-2", "key-3", "foo"}...), existingItems, map[string]string{}, jsonPath, overwriteKeys)
			expected := []utils.JsonPatch{
				utils.NewJsonPatch("remove", jsonPath, "key-2", ""),
				utils.NewJsonPatch("remove", jsonPath, "key-3", ""),
			}
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches(expected))
		})

		Convey("When there are no itmes to remove but new items to add", func() {
			newItems := map[string]string{"new-key": "new-val", "key-1": "new-1"}
			p := createPatches(sets.New([]string{"key-1"}...), existingItems, newItems, jsonPath, overwriteKeys)
			expected := []utils.JsonPatch{
				utils.NewJsonPatch("add", jsonPath, "new-key", newItems["new-key"]),
				utils.NewJsonPatch("replace", jsonPath, "key-1", newItems["key-1"]),
			}
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches(expected))
		})

		Convey("When there are items to remove add and update", func() {
			newItems := map[string]string{"new-key": "new-val", "key-2": "new-2", "key-4": "val-4"}
			p := createPatches(sets.New([]string{"key-1", "key-2", "key-3", "foo"}...), existingItems, newItems, jsonPath, overwriteKeys)
			expected := []utils.JsonPatch{
				utils.NewJsonPatch("add", jsonPath, "new-key", newItems["new-key"]),
				utils.NewJsonPatch("add", jsonPath, "key-4", newItems["key-4"]),
				utils.NewJsonPatch("replace", jsonPath, "key-2", newItems["key-2"]),
				utils.NewJsonPatch("remove", jsonPath, "key-1", ""),
				utils.NewJsonPatch("remove", jsonPath, "key-3", ""),
			}
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches(expected))
		})

		Convey("When overwrite of keys is denied and there is already an existant key", func() {
			overwriteKeys = false
			newItems := map[string]string{"key-1": "new-2", "key-4": "val-4"}
			p := createPatches(sets.New([]string{}...), existingItems, newItems, jsonPath, overwriteKeys)
			expected := []utils.JsonPatch{
				utils.NewJsonPatch("add", jsonPath, "key-4", newItems["key-4"]),
				utils.NewJsonPatch("replace", jsonPath, "key-1", newItems["key-1"]),
			}
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches(expected))
		})
	})
}

func TestRemoveLabelsWithPrefix(t *testing.T) {
	Convey("When removing labels", t, func() {
		n := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"single-label": "123",
					"multiple_A":   "a",
					"multiple_B":   "b",
				},
			},
		}

		Convey("a unique label should be removed", func() {
			p := removeLabelsWithPrefix(n, "single")
			So(p, ShouldResemble, []utils.JsonPatch{utils.NewJsonPatch("remove", "/metadata/labels", "single-label", "")})
		})

		Convey("a non-unique search string should remove all matching keys", func() {
			p := removeLabelsWithPrefix(n, "multiple")
			So(sortJsonPatches(p), ShouldResemble, sortJsonPatches([]utils.JsonPatch{
				utils.NewJsonPatch("remove", "/metadata/labels", "multiple_A", ""),
				utils.NewJsonPatch("remove", "/metadata/labels", "multiple_B", ""),
			}))
		})

		Convey("a search string with no matches should not alter labels", func() {
			removeLabelsWithPrefix(n, "unique")
			So(n.Labels, ShouldContainKey, "single-label")
			So(n.Labels, ShouldContainKey, "multiple_A")
			So(n.Labels, ShouldContainKey, "multiple_B")
			So(len(n.Labels), ShouldEqual, 3)
		})
	})
}

func TestConfigParse(t *testing.T) {
	Convey("When parsing configuration", t, func() {
		master := newFakeMaster()
		overrides := `{"noPublish": true, "enableTaints": true, "extraLabelNs": ["added.ns.io","added.kubernetes.io"], "denyLabelNs": ["denied.ns.io","denied.kubernetes.io"], "labelWhiteList": "foo"}`

		Convey("and no core cmdline flags have been specified", func() {
			So(master.configure("non-existing-file", overrides), ShouldBeNil)
			Convey("overrides should be in effect", func() {
				So(master.config.NoPublish, ShouldResemble, true)
				So(master.config.EnableTaints, ShouldResemble, true)
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"added.ns.io": struct{}{}, "added.kubernetes.io": struct{}{}})
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"denied.ns.io": struct{}{}, "denied.kubernetes.io": struct{}{}})
				So(master.config.LabelWhiteList.String(), ShouldEqual, "foo")
			})
		})
		Convey("and a non-accessible file, but cmdline flags and some overrides are specified", func() {
			master.args = Args{Overrides: ConfigOverrideArgs{
				ExtraLabelNs: &utils.StringSetVal{"override.added.ns.io": struct{}{}},
				DenyLabelNs:  &utils.StringSetVal{"override.denied.ns.io": struct{}{}}}}
			So(master.configure("non-existing-file", overrides), ShouldBeNil)

			Convey("cmdline flags should be in effect instead overrides", func() {
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"override.added.ns.io": struct{}{}})
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"override.denied.ns.io": struct{}{}})
			})
			Convey("overrides should take effect", func() {
				So(master.config.NoPublish, ShouldBeTrue)
				So(master.config.EnableTaints, ShouldBeTrue)
			})
		})
		// Create a temporary config file
		f, err := os.CreateTemp("", "nfd-test-")
		defer func() {
			if err := os.Remove(f.Name()); err != nil {
				t.Logf("failed to remove temp file %s: %v", f.Name(), err)
			}
		}()
		So(err, ShouldBeNil)
		_, err = f.WriteString(`
noPublish: true
denyLabelNs: ["denied.ns.io","denied.kubernetes.io"]
enableTaints: false
labelWhiteList: "foo"
leaderElection:
  leaseDuration: 20s
  renewDeadline: 4s
  retryPeriod: 30s
`)
		So(err, ShouldBeNil)
		err = f.Close()
		So(err, ShouldBeNil)

		Convey("and a proper config file is specified", func() {
			master.args = Args{Overrides: ConfigOverrideArgs{ExtraLabelNs: &utils.StringSetVal{"override.added.ns.io": struct{}{}}}}
			So(master.configure(f.Name(), ""), ShouldBeNil)
			Convey("specified configuration should take effect", func() {
				// Verify core config
				So(master.config.NoPublish, ShouldBeTrue)
				So(master.config.EnableTaints, ShouldBeFalse)
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"override.added.ns.io": struct{}{}})
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"denied.ns.io": struct{}{}, "denied.kubernetes.io": struct{}{}})
				So(master.config.LabelWhiteList.String(), ShouldEqual, "foo")
				So(master.config.LeaderElection.LeaseDuration.Seconds(), ShouldEqual, float64(20))
				So(master.config.LeaderElection.RenewDeadline.Seconds(), ShouldEqual, float64(4))
				So(master.config.LeaderElection.RetryPeriod.Seconds(), ShouldEqual, float64(30))
			})
		})

		Convey("and a proper config file and overrides are given", func() {
			master.args = Args{Overrides: ConfigOverrideArgs{DenyLabelNs: &utils.StringSetVal{"denied.ns.io": struct{}{}}}}
			overrides := `{"extraLabelNs": ["added.ns.io"], "noPublish": true}`
			So(master.configure(f.Name(), overrides), ShouldBeNil)

			Convey("overrides should take precedence over the config file", func() {
				// Verify core config
				So(master.config.ExtraLabelNs, ShouldResemble, utils.StringSetVal{"added.ns.io": struct{}{}}) // from overrides
				So(master.config.DenyLabelNs, ShouldResemble, utils.StringSetVal{"denied.ns.io": struct{}{}}) // from cmdline
			})
		})
	})
}

func newTestNodeList() *corev1.NodeList {
	l := corev1.NodeList{}

	for i := 0; i < 1000; i++ {
		n := corev1.Node{}
		n.Name = fmt.Sprintf("node %v", i)
		n.Labels = map[string]string{}
		n.Annotations = map[string]string{}
		n.Status.Capacity = corev1.ResourceList{}

		l.Items = append(l.Items, n)
	}
	return &l
}

func BenchmarkNfdAPIUpdateAllNodes(b *testing.B) {
	//nolint:staticcheck // See issue #2400 for migration to NewClientset
	fakeCli := fakeclient.NewSimpleClientset(newTestNodeList())
	fakeMaster := newFakeMaster(WithKubernetesClient(fakeCli))
	fakeMaster.nfdController = newFakeNfdAPIController(fakenfdclient.NewSimpleClientset())

	updaterPool := newUpdaterPool(fakeMaster)
	fakeMaster.updaterPool = updaterPool

	updaterPool.start(10)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = fakeMaster.nfdAPIUpdateAllNodes()
	}
	fmt.Println(b.Elapsed())
}

// withTimeout is a custom assertion for polling a value asynchronously
// actual is a function for getting the actual value
// expected[0] is a time.Duration value specifying the timeout
// expected[1] is  the "real" assertion function to be called
// expected[2:] are the arguments for the "real" assertion function
func withTimeout(actual interface{}, expected ...interface{}) string {
	getter, ok := actual.(func() interface{})
	if !ok {
		return "not getterFunc"
	}
	t, ok := expected[0].(time.Duration)
	if !ok {
		return "not time.Duration"
	}
	f, ok := expected[1].(func(interface{}, ...interface{}) string)
	if !ok {
		return "not an assert func"
	}
	timeout := time.After(t)
	for {
		result := f(getter(), expected[2:]...)
		if result == "" {
			return ""
		}
		select {
		case <-timeout:
			return result
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func sortJsonPatches(p []utils.JsonPatch) []utils.JsonPatch {
	sort.Slice(p, func(i, j int) bool { return p[i].Path < p[j].Path })
	return p
}

// Remove any labels having the given prefix
func removeLabelsWithPrefix(n *corev1.Node, search string) []utils.JsonPatch {
	var p []utils.JsonPatch

	for k := range n.Labels {
		if strings.HasPrefix(k, search) {
			p = append(p, utils.NewJsonPatch("remove", "/metadata/labels", k, ""))
		}
	}

	return p
}

func TestGetDynamicValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		features *nfdv1alpha1.Features
		want     string
		fail     bool
	}{
		{
			name:  "Valid dynamic value",
			value: "@test.feature.LSM",
			features: &nfdv1alpha1.Features{
				Attributes: map[string]nfdv1alpha1.AttributeFeatureSet{
					"test.feature": {
						Elements: map[string]string{
							"LSM": "123",
						},
					},
				},
			},
			want: "123",
			fail: false,
		},
		{
			name:     "Invalid feature name",
			value:    "@invalid",
			features: &nfdv1alpha1.Features{},
			want:     "",
			fail:     true,
		},
		{
			name:     "Element not found",
			value:    "@test.feature.LSM",
			features: &nfdv1alpha1.Features{},
			want:     "",
			fail:     true,
		},
		{
			name:     "Invalid dynamic value",
			value:    "@test.feature.LSM",
			features: &nfdv1alpha1.Features{},
			want:     "",
			fail:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDynamicValue(tt.value, tt.features)
			if err != nil && !tt.fail {
				t.Errorf("getDynamicValue() = %v, want %v", err, tt.want)
			}
			if got != tt.want {
				t.Errorf("getDynamicValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
