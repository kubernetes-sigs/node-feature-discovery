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

package e2e

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	taintutils "k8s.io/kubernetes/pkg/util/taints"

	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	e2elog "k8s.io/kubernetes/test/e2e/framework"
)

type k8sLabels map[string]string
type k8sAnnotations map[string]string

// eventuallyNonControlPlaneNodes is a helper for asserting node properties
func eventuallyNonControlPlaneNodes(ctx context.Context, cli clientset.Interface) gomega.AsyncAssertion {
	return gomega.Eventually(func(g gomega.Gomega, ctx context.Context) ([]corev1.Node, error) {
		return getNonControlPlaneNodes(ctx, cli)
	}).WithPolling(1 * time.Second).WithTimeout(10 * time.Second).WithContext(ctx)
}

// MatchLabels returns a specialized Gomega matcher for checking if a list of
// nodes are labeled as expected.
func MatchLabels(expectedNew map[string]k8sLabels, oldNodes []corev1.Node) gomegatypes.GomegaMatcher {
	matcher := &nodeIterablePropertyMatcher[k8sLabels]{
		propertyName: "labels",
		matchFunc: func(newNode, oldNode corev1.Node, expected k8sLabels) ([]string, []string, []string) {
			expectedAll := maps.Clone(oldNode.Labels)
			maps.Copy(expectedAll, expected)
			return matchMap(newNode.Labels, expectedAll)
		},
	}

	return &nodeListPropertyMatcher[k8sLabels]{
		expected: expectedNew,
		oldNodes: oldNodes,
		matcher:  matcher,
	}
}

// MatchAnnotations returns a specialized Gomega matcher for checking if a list of
// nodes are annotated as expected.
func MatchAnnotations(expectedNew map[string]k8sAnnotations, oldNodes []corev1.Node) gomegatypes.GomegaMatcher {
	matcher := &nodeIterablePropertyMatcher[k8sAnnotations]{
		propertyName: "annotations",
		matchFunc: func(newNode, oldNode corev1.Node, expected k8sAnnotations) ([]string, []string, []string) {
			expectedAll := maps.Clone(oldNode.Annotations)
			maps.Copy(expectedAll, expected)
			return matchMap(newNode.Annotations, expectedAll)
		},
	}

	return &nodeListPropertyMatcher[k8sAnnotations]{
		expected: expectedNew,
		oldNodes: oldNodes,
		matcher:  matcher,
	}
}

// MatchCapacity returns a specialized Gomega matcher for checking if a list of
// nodes have resource capacity as expected.
func MatchCapacity(expectedNew map[string]corev1.ResourceList, oldNodes []corev1.Node) gomegatypes.GomegaMatcher {
	matcher := &nodeIterablePropertyMatcher[corev1.ResourceList]{
		propertyName: "resource capacity",
		matchFunc: func(newNode, oldNode corev1.Node, expected corev1.ResourceList) ([]string, []string, []string) {
			expectedAll := oldNode.Status.DeepCopy().Capacity
			maps.Copy(expectedAll, expected)
			return matchMap(newNode.Status.Capacity, expectedAll)
		},
	}

	return &nodeListPropertyMatcher[corev1.ResourceList]{
		expected: expectedNew,
		oldNodes: oldNodes,
		matcher:  matcher,
	}
}

// MatchTaints returns a specialized Gomega matcher for checking if a list of
// nodes are tainted as expected.
func MatchTaints(expectedNew map[string][]corev1.Taint, oldNodes []corev1.Node) gomegatypes.GomegaMatcher {
	matcher := &nodeIterablePropertyMatcher[[]corev1.Taint]{
		propertyName: "taints",
		matchFunc: func(newNode, oldNode corev1.Node, expected []corev1.Taint) (missing, invalid, unexpected []string) {
			expectedAll := oldNode.Spec.DeepCopy().Taints
			expectedAll = append(expectedAll, expected...)
			taints := newNode.Spec.Taints

			for _, expectedTaint := range expectedAll {
				if !taintutils.TaintExists(taints, &expectedTaint) {
					missing = append(missing, expectedTaint.ToString())
				} else if ok, matched := taintWithValueExists(taints, &expectedTaint); !ok {
					invalid = append(invalid, fmt.Sprintf("%s, expected value %s", matched.ToString(), expectedTaint.Value))
				}
			}

			for _, taint := range taints {
				if !taintutils.TaintExists(expectedAll, &taint) {
					unexpected = append(unexpected, taint.ToString())
				}
			}
			return missing, invalid, unexpected
		},
	}

	return &nodeListPropertyMatcher[[]corev1.Taint]{
		expected: expectedNew,
		oldNodes: oldNodes,
		matcher:  matcher,
	}
}

func taintWithValueExists(taints []corev1.Taint, taintToFind *corev1.Taint) (found bool, matched corev1.Taint) {
	for _, taint := range taints {
		if taint.Key == taintToFind.Key && taint.Effect == taintToFind.Effect {
			matched = taint
			if taint.Value == taintToFind.Value {
				return true, matched
			}
		}
	}
	return false, matched
}

// nodeListPropertyMatcher is a generic Gomega matcher for asserting one property a group of nodes.
type nodeListPropertyMatcher[T any] struct {
	expected map[string]T
	oldNodes []corev1.Node

	matcher nodePropertyMatcher[T]
}

// nodePropertyMatcher is a generic helper type for matching one node.
type nodePropertyMatcher[T any] interface {
	match(newNode, oldNode corev1.Node, expected T) bool
	message() string
	negatedMessage() string
}

// Match method of the GomegaMatcher interface.
func (m *nodeListPropertyMatcher[T]) Match(actual interface{}) (bool, error) {
	nodes, ok := actual.([]corev1.Node)
	if !ok {
		return false, fmt.Errorf("expected []corev1.Node, got: %T", actual)
	}

	for _, node := range nodes {
		expected, ok := m.expected[node.Name]
		if !ok {
			if defaultExpected, ok := m.expected["*"]; ok {
				expected = defaultExpected
			} else {
				e2elog.Logf("Skipping node %q as no expected was specified", node.Name)
				continue
			}
		}

		oldNode := getNode(m.oldNodes, node.Name)
		if matched := m.matcher.match(node, oldNode, expected); !matched {
			return false, nil
		}
	}
	return true, nil
}

// FailureMessage method of the GomegaMatcher interface.
func (m *nodeListPropertyMatcher[T]) FailureMessage(actual interface{}) string {
	return m.matcher.message()
}

// NegatedFailureMessage method of the GomegaMatcher interface.
func (m *nodeListPropertyMatcher[T]) NegatedFailureMessage(actual interface{}) string {
	return m.matcher.negatedMessage()
}

// nodeIterablePropertyMatcher is a nodePropertyMatcher for matching iterable
// elements such as maps or lists.
type nodeIterablePropertyMatcher[T any] struct {
	propertyName string
	matchFunc    func(newNode, oldNode corev1.Node, expected T) ([]string, []string, []string)

	// TODO remove nolint when golangci-lint is able to cope with generics
	node         *corev1.Node //nolint:unused
	missing      []string     //nolint:unused
	invalidValue []string     //nolint:unused
	unexpected   []string     //nolint:unused

}

// TODO remove nolint when golangci-lint is able to cope with generics
//
//nolint:unused
func (m *nodeIterablePropertyMatcher[T]) match(newNode, oldNode corev1.Node, expected T) bool {
	m.node = &newNode
	m.missing, m.invalidValue, m.unexpected = m.matchFunc(newNode, oldNode, expected)

	return len(m.missing) == 0 && len(m.invalidValue) == 0 && len(m.unexpected) == 0
}

// TODO remove nolint when golangci-lint is able to cope with generics
//
//nolint:unused
func (m *nodeIterablePropertyMatcher[T]) message() string {
	msg := fmt.Sprintf("Node %q %s did not match:", m.node.Name, m.propertyName)
	if len(m.missing) > 0 {
		msg += fmt.Sprintf("\n  missing:\n    %s", strings.Join(m.missing, "\n    "))
	}
	if len(m.invalidValue) > 0 {
		msg += fmt.Sprintf("\n  invalid value:\n    %s", strings.Join(m.invalidValue, "\n    "))
	}
	if len(m.unexpected) > 0 {
		msg += fmt.Sprintf("\n  unexpected:\n    %s", strings.Join(m.unexpected, "\n    "))
	}
	return msg
}

// TODO remove nolint when golangci-lint is able to cope with generics
//
//nolint:unused
func (m *nodeIterablePropertyMatcher[T]) negatedMessage() string {
	return fmt.Sprintf("Node %q matched unexpectedly", m.node.Name)
}

// matchMap is a helper for matching map types
func matchMap[M ~map[K]V, K comparable, V comparable](actual, expected M) (missing, invalid, unexpected []string) {
	for k, ve := range expected {
		va, ok := actual[k]
		if !ok {
			missing = append(missing, fmt.Sprintf("%v", k))
		} else if va != ve {
			invalid = append(invalid, fmt.Sprintf("%v=%v, expected value %v", k, va, ve))
		}
	}
	for k, v := range actual {
		if _, ok := expected[k]; !ok {
			unexpected = append(unexpected, fmt.Sprintf("%v=%v", k, v))
		}
	}
	return missing, invalid, unexpected
}
