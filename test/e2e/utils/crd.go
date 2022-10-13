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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	nfdclientset "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned"
	nfdscheme "sigs.k8s.io/node-feature-discovery/pkg/generated/clientset/versioned/scheme"
)

var packagePath string

// CreateNodeFeatureRulesCRD creates the NodeFeatureRule CRD in the API server.
func CreateNodeFeatureRulesCRD(cli extclient.Interface) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd, err := crdFromFile(filepath.Join(packagePath, "..", "..", "..", "deployment", "base", "nfd-crds", "nodefeaturerule-crd.yaml"))
	if err != nil {
		return nil, err
	}

	// Delete existing CRD (if any) with this we also get rid of stale objects
	err = cli.ApiextensionsV1().CustomResourceDefinitions().Delete(context.TODO(), crd.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to delete NodeFeatureRule CRD: %w", err)
	}

	return cli.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), crd, metav1.CreateOptions{})
}

// CreateNodeFeatureRuleFromFile creates a NodeFeatureRule object from a given file located under test data directory.
func CreateNodeFeatureRuleFromFile(cli nfdclientset.Interface, filename string) error {
	obj, err := nodeFeatureRuleFromFile(filepath.Join(packagePath, "..", "data", filename))
	if err != nil {
		return err
	}
	_, err = cli.NfdV1alpha1().NodeFeatureRules().Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func apiObjFromFile(path string, decoder apiruntime.Decoder) (apiruntime.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	obj, _, err := decoder.Decode(data, nil, nil)
	return obj, err
}

// crdFromFile creates a CustomResourceDefinition API object from a file.
func crdFromFile(path string) (*apiextensionsv1.CustomResourceDefinition, error) {
	obj, err := apiObjFromFile(path, scheme.Codecs.UniversalDeserializer())
	if err != nil {
		return nil, err
	}

	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return nil, fmt.Errorf("unexpected type %t when reading %q", obj, path)
	}

	return crd, nil
}

func nodeFeatureRuleFromFile(path string) (*nfdv1alpha1.NodeFeatureRule, error) {
	obj, err := apiObjFromFile(path, nfdscheme.Codecs.UniversalDeserializer())
	if err != nil {
		return nil, err
	}

	crd, ok := obj.(*nfdv1alpha1.NodeFeatureRule)
	if !ok {
		return nil, fmt.Errorf("unexpected type %t when reading %q", obj, path)
	}

	return crd, nil
}

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	packagePath = filepath.Dir(thisFile)

	// Register k8s scheme to be able to create CRDs
	_ = apiextensionsv1.AddToScheme(scheme.Scheme)
}
