/*
Copyright 2022 The Kubernetes Authors.

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

package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	nfdscheme "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned/scheme"
)

var packagePath string

// CreateNfdCRDs creates the NodeFeatureRule CRD in the API server.
func CreateNfdCRDs(ctx context.Context, cli extclient.Interface) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	crds, err := crdsFromFile(filepath.Join(packagePath, "..", "..", "..", "deployment", "base", "nfd-crds", "nfd-api-crds.yaml"))
	if err != nil {
		return nil, err
	}

	newCRDs := make([]*apiextensionsv1.CustomResourceDefinition, len(crds))
	for i, crd := range crds {
		// Delete existing CRD (if any) with this we also get rid of stale objects
		err = cli.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to delete %q CRD: %w", crd.Name, err)
		} else if err == nil {
			// Wait for CRD deletion to complete before trying to re-create it
			err = wait.PollUntilContextTimeout(ctx, 1*time.Second, 1*time.Minute, false, func(ctx context.Context) (bool, error) {
				_, err = cli.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
				if err == nil {
					return false, nil
				} else if errors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			})
			if err != nil {
				return nil, fmt.Errorf("timed out waiting for %q CRD to be deleted: %w", crd.Name, err)
			}
		}

		newCRDs[i], err = cli.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	}
	return newCRDs, nil
}

// CreateOrUpdateNodeFeaturesFromFile creates/updates a NodeFeature object from a given file located under test data directory.
func CreateOrUpdateNodeFeaturesFromFile(ctx context.Context, cli nfdclientset.Interface, filename, namespace, nodename string) ([]string, error) {
	objs, err := nodeFeaturesFromFile(filepath.Join(packagePath, "..", "data", filename))
	if err != nil {
		return nil, err
	}

	names := make([]string, len(objs))
	for i, obj := range objs {
		obj.Namespace = namespace
		if obj.Labels == nil {
			obj.Labels = map[string]string{}
		}
		obj.Labels[nfdv1alpha1.NodeFeatureObjNodeNameLabel] = nodename

		if oldObj, err := cli.NfdV1alpha1().NodeFeatures(namespace).Get(ctx, obj.Name, metav1.GetOptions{}); errors.IsNotFound(err) {
			if _, err := cli.NfdV1alpha1().NodeFeatures(namespace).Create(ctx, obj, metav1.CreateOptions{}); err != nil {
				return names, fmt.Errorf("failed to create NodeFeature %w", err)
			}
		} else if err == nil {
			obj.SetResourceVersion(oldObj.GetResourceVersion())
			if _, err = cli.NfdV1alpha1().NodeFeatures(namespace).Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
				return names, fmt.Errorf("failed to update NodeFeature object: %w", err)
			}
		} else {
			return names, fmt.Errorf("failed to get NodeFeature %w", err)
		}
		names[i] = obj.Name
	}
	return names, nil
}

// CreateNodeFeatureRuleFromFile creates a NodeFeatureRule object from a given file located under test data directory.
func CreateNodeFeatureRulesFromFile(ctx context.Context, cli nfdclientset.Interface, filename string) error {
	objs, err := nodeFeatureRulesFromFile(filepath.Join(packagePath, "..", "data", filename))
	if err != nil {
		return err
	}

	for _, obj := range objs {
		if _, err = cli.NfdV1alpha1().NodeFeatureRules().Create(ctx, obj, metav1.CreateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// UpdateNodeFeatureRulesFromFile updates existing NodeFeatureRule object from a given file located under test data directory.
func UpdateNodeFeatureRulesFromFile(ctx context.Context, cli nfdclientset.Interface, filename string) error {
	objs, err := nodeFeatureRulesFromFile(filepath.Join(packagePath, "..", "data", filename))
	if err != nil {
		return err
	}

	for _, obj := range objs {
		var nfr *nfdv1alpha1.NodeFeatureRule
		if nfr, err = cli.NfdV1alpha1().NodeFeatureRules().Get(ctx, obj.Name, metav1.GetOptions{}); err != nil {
			return fmt.Errorf("failed to get NodeFeatureRule %w", err)
		}

		obj.SetResourceVersion(nfr.GetResourceVersion())
		if _, err = cli.NfdV1alpha1().NodeFeatureRules().Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update NodeFeatureRule %w", err)
		}
	}
	return nil
}

// CreateNodeFeature creates a dummy NodeFeature object for a node
func CreateNodeFeature(ctx context.Context, cli nfdclientset.Interface, namespace, name, nodeName string) error {
	nr := &nfdv1alpha1.NodeFeature{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{nfdv1alpha1.NodeFeatureObjNodeNameLabel: nodeName},
		},
	}
	_, err := cli.NfdV1alpha1().NodeFeatures(namespace).Create(ctx, nr, metav1.CreateOptions{})
	return err
}

func apiObjsFromFile(path string, decoder apiruntime.Decoder) ([]apiruntime.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// TODO: find out a nicer way to decode multiple api objects from a single
	// file (K8s must have that somewhere)
	split := bytes.Split(data, []byte("---"))
	objs := []apiruntime.Object{}

	for _, slice := range split {
		if len(slice) == 0 {
			continue
		}
		obj, _, err := decoder.Decode(slice, nil, nil)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
	}
	return objs, err
}

// crdsFromFile creates a CustomResourceDefinition API object from a file.
func crdsFromFile(path string) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	objs, err := apiObjsFromFile(path, scheme.Codecs.UniversalDeserializer())
	if err != nil {
		return nil, err
	}

	crds := make([]*apiextensionsv1.CustomResourceDefinition, len(objs))

	for i, obj := range objs {
		var ok bool
		crds[i], ok = obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T when reading %q", obj, path)
		}
	}

	return crds, nil
}

func nodeFeaturesFromFile(path string) ([]*nfdv1alpha1.NodeFeature, error) {
	objs, err := apiObjsFromFile(path, nfdscheme.Codecs.UniversalDeserializer())
	if err != nil {
		return nil, err
	}

	crs := make([]*nfdv1alpha1.NodeFeature, len(objs))

	for i, obj := range objs {
		var ok bool
		crs[i], ok = obj.(*nfdv1alpha1.NodeFeature)
		if !ok {
			return nil, fmt.Errorf("unexpected type %t when reading %q", obj, path)
		}
	}

	return crs, nil
}

func nodeFeatureRulesFromFile(path string) ([]*nfdv1alpha1.NodeFeatureRule, error) {
	objs, err := apiObjsFromFile(path, nfdscheme.Codecs.UniversalDeserializer())
	if err != nil {
		return nil, err
	}

	crs := make([]*nfdv1alpha1.NodeFeatureRule, len(objs))

	for i, obj := range objs {
		var ok bool
		crs[i], ok = obj.(*nfdv1alpha1.NodeFeatureRule)
		if !ok {
			return nil, fmt.Errorf("unexpected type %t when reading %q", obj, path)
		}
	}

	return crs, nil
}

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	packagePath = filepath.Dir(thisFile)

	// Register k8s scheme to be able to create CRDs
	_ = apiextensionsv1.AddToScheme(scheme.Scheme)
}
