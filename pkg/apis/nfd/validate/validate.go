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

package validate

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8sQuantity "k8s.io/apimachinery/pkg/api/resource"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/template"
)

var (
	// Default error message for invalid label/annotation keys
	ErrNSNotAllowed = fmt.Errorf("namespace is not allowed")
	// Default error message for invalid label/annotation keys
	ErrUnprefixedKeysNotAllowed = fmt.Errorf("unprefixed keys are not allowed")
	// Default error for invalid taint effect
	ErrInvalidTaintEffect = fmt.Errorf("invalid taint effect")
	// Default error for empty taint effect
	ErrEmptyTaintEffect = fmt.Errorf("empty taint effect")
)

// MatchAny validates a slice of MatchAnyElem and returns a slice of errors if
// any of the MatchAnyElem are invalid.
func MatchAny(matchAny []nfdv1alpha1.MatchAnyElem) []error {
	var validationErr []error

	for _, matcher := range matchAny {
		validationErr = append(validationErr, MatchFeatures(matcher.MatchFeatures)...)
	}

	return validationErr
}

// MatchFeatures validates a slice of FeatureMatcher and returns a slice of
// errors if any of the FeatureMatcher are invalid.
func MatchFeatures(matchFeature nfdv1alpha1.FeatureMatcher) []error {
	var validationErr []error

	for _, match := range matchFeature {
		nameSplit := strings.Split(match.Feature, ".")
		if len(nameSplit) != 2 {
			validationErr = append(validationErr, fmt.Errorf("invalid feature name %v (not <domain>.<feature>), cannot be used for templating", match.Feature))
		}
	}

	return validationErr
}

// Template validates a template string and returns a slice of errors if the
// template is invalid.
func Template(labelsTemplate string) []error {
	var validationErr []error
	// Only validate template
	_, err := template.NewHelper(labelsTemplate)
	if err != nil {
		validationErr = append(validationErr, err)
	}
	return validationErr
}

// Labels validates a map of labels and returns a slice of errors if any of the
// labels are invalid.
func Labels(labels map[string]string) []error {
	var errs []error
	for key, value := range labels {
		if err := Label(key, value); err != nil {
			errs = append(errs, fmt.Errorf("invalid label %q:%q %w", key, value, err))
		}
	}
	return errs
}

// Label validates a label key and value and returns an error if the key or
// value is invalid.
func Label(key, value string) error {
	//Validate label key and value
	if err := k8svalidation.IsQualifiedName(key); len(err) > 0 {
		return fmt.Errorf("invalid label key %q: %s", key, strings.Join(err, "; "))
	}
	// Check label namespace, filter out if ns is not whitelisted
	ns, _ := splitNs(key)
	// And is not empty
	if ns == "" {
		return ErrUnprefixedKeysNotAllowed
	}
	// And is not a denied namespace
	if ns == "kubernetes.io" || strings.HasSuffix(ns, ".kubernetes.io") {
		// And is not a default namespace
		if ns != nfdv1alpha1.FeatureLabelNs && ns != nfdv1alpha1.ProfileLabelNs &&
			!strings.HasSuffix(ns, nfdv1alpha1.FeatureLabelSubNsSuffix) && !strings.HasSuffix(ns, nfdv1alpha1.ProfileLabelSubNsSuffix) {
			return ErrNSNotAllowed
		}
	}

	// Validate label value
	if err := k8svalidation.IsValidLabelValue(value); len(err) > 0 {
		return fmt.Errorf("invalid value %q: %s", value, strings.Join(err, "; "))
	}

	return nil
}

// Annotations validates a map of annotations and returns a slice of errors if
// any of the annotations are invalid.
func Annotations(annotations map[string]string) []error {
	var errs []error
	for key, value := range annotations {
		if err := Annotation(key, value); err != nil {
			errs = append(errs, fmt.Errorf("invalid annotation %q:%q %w", key, value, err))
		}
	}
	return errs
}

// Annotation validates an annotation key and value and returns an error if the
// key or value is invalid.
func Annotation(key, value string) error {
	// Validate the annotation key
	if err := k8svalidation.IsQualifiedName(key); len(err) > 0 {
		return fmt.Errorf("invalid annotation key %q: %s", key, strings.Join(err, "; "))
	}

	ns, _ := splitNs(key)
	// And is not empty
	if ns == "" {
		return ErrUnprefixedKeysNotAllowed
	}
	// And is not a denied namespace
	if ns == "kubernetes.io" || strings.HasSuffix(ns, ".kubernetes.io") {
		// And is not a default namespace
		if ns != nfdv1alpha1.FeatureAnnotationNs && !strings.HasSuffix(ns, nfdv1alpha1.FeatureAnnotationSubNsSuffix) {
			return ErrNSNotAllowed
		}
	}

	// Validate annotation value
	if len(value) > nfdv1alpha1.FeatureAnnotationValueSizeLimit {
		return fmt.Errorf("invalid value: too long: feature annotations must not be longer than %d characters", nfdv1alpha1.FeatureAnnotationValueSizeLimit)
	}

	return nil
}

// Taints validates a slice of taints and returns a slice of errors if any of
// the taints are invalid.
func Taints(taints []corev1.Taint) []error {
	var errs []error
	for _, taint := range taints {
		if err := Taint(&taint); err != nil {
			errs = append(errs, fmt.Errorf("invalid taint %s=%s:%s: %w", taint.Key, taint.Value, taint.Effect, err))
		}
	}
	return errs
}

// Taint validates a taint key and value and returns an error if the key or
// value is invalid.
func Taint(taint *corev1.Taint) error {
	ns, _ := splitNs(taint.Key)
	// And is not empty
	if ns == "" {
		return ErrUnprefixedKeysNotAllowed
	}
	// And is not a denied namespace
	if ns == "kubernetes.io" || strings.HasSuffix(ns, ".kubernetes.io") {
		// And is not a default namespace
		if ns != nfdv1alpha1.TaintNs && !strings.HasSuffix(ns, nfdv1alpha1.TaintSubNsSuffix) {
			return ErrNSNotAllowed
		}
	}

	// Validate taint effect is not empty
	if taint.Effect == "" {
		return ErrEmptyTaintEffect
	}
	// Validate effect to be only one of NoSchedule, PreferNoSchedule or NoExecute
	if taint.Effect != corev1.TaintEffectNoSchedule &&
		taint.Effect != corev1.TaintEffectPreferNoSchedule &&
		taint.Effect != corev1.TaintEffectNoExecute {
		return ErrInvalidTaintEffect
	}

	return nil
}

// ExtendedResources validates a map of extended resources and returns a slice
// of errors if any of the extended resources are invalid.
func ExtendedResources(extendedResources map[string]string) []error {
	var errs []error
	for key, value := range extendedResources {
		if err := ExtendedResource(key, value); err != nil {
			errs = append(errs, fmt.Errorf("invalid extended resource %q:%q %w", key, value, err))
		}
	}
	return errs
}

// ExtendedResource validates an extended resource key and value and returns an
// error if the key or value is invalid.
func ExtendedResource(key, value string) error {
	//Validate extendedResource name
	if errs := k8svalidation.IsQualifiedName(key); len(errs) > 0 {
		return fmt.Errorf("invalid name %q: %s", key, strings.Join(errs, "; "))
	}
	ns, _ := splitNs(key)
	// And is not empty
	if ns == "" {
		return ErrUnprefixedKeysNotAllowed
	}
	// And is not a denied namespace
	if ns == "kubernetes.io" || strings.HasSuffix(ns, ".kubernetes.io") {
		// And is not a default namespace
		if ns != nfdv1alpha1.ExtendedResourceNs && !strings.HasSuffix(ns, nfdv1alpha1.ExtendedResourceSubNsSuffix) {
			return ErrNSNotAllowed
		}
	}

	// Validate extended resource value
	_, err := k8sQuantity.ParseQuantity(value)
	if err != nil {
		return fmt.Errorf("invalid value %q: %w", value, err)
	}

	return nil
}

// splitNs splits a name into its namespace and name parts
func splitNs(fullname string) (string, string) {
	split := strings.SplitN(fullname, "/", 2)
	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", fullname
}
