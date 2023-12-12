package validate

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestAnnotation(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  error
		fail  bool
	}{
		{
			name:  "Valid annotation",
			key:   "feature.node.kubernetes.io/feature",
			value: "true",
			want:  nil,
		},
		{
			name:  "Invalid annotation key",
			key:   "invalid-key",
			value: "true",
			want:  ErrUnprefixedKeysNotAllowed,
		},
		{
			name:  "Invalid annotation value",
			key:   "feature.node.kubernetes.io/feature",
			value: "invalid value",
			want:  fmt.Errorf("invalid value \"invalid value\": value must be a valid label value"),
			fail:  true,
		},
		{
			name:  "Denied annotation key",
			key:   "kubernetes.io/denied",
			value: "true",
			want:  ErrNSNotAllowed,
			fail:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Annotation(tt.key, tt.value)
			if got != tt.want {
				if tt.fail {
					return
				}
				t.Errorf("Annotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaint(t *testing.T) {
	tests := []struct {
		name  string
		taint *corev1.Taint
		want  error
	}{
		{
			name: "Valid taint",
			taint: &corev1.Taint{
				Key:    "feature.node.kubernetes.io/taint",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: nil,
		},
		{
			name: "UNPREFIXED taint key",
			taint: &corev1.Taint{
				Key:    "invalid-key",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: ErrUnprefixedKeysNotAllowed,
		},
		{
			name: "Invalid taint key",
			taint: &corev1.Taint{
				Key:    "invalid.kubernetes.io/invalid-key",
				Value:  "true",
				Effect: corev1.TaintEffectNoSchedule,
			},
			want: ErrNSNotAllowed,
		},
		{
			name: "Empty taint effect",
			taint: &corev1.Taint{
				Key:    "feature.node.kubernetes.io/taint",
				Value:  "true",
				Effect: "",
			},
			want: ErrEmptyTaintEffect,
		},
		{
			name: "Invalid taint effect",
			taint: &corev1.Taint{
				Key:    "feature.node.kubernetes.io/taint",
				Value:  "true",
				Effect: "invalid-effect",
			},
			want: ErrInvalidTaintEffect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Taint(tt.taint)
			if got != tt.want {
				t.Errorf("Taint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  error
		fail  bool
	}{
		{
			name:  "Valid label",
			key:   "feature.node.kubernetes.io/label",
			value: "true",
			want:  nil,
			fail:  false,
		},
		{
			name:  "Valid vendor label",
			key:   "vendor.io/label",
			value: "true",
			want:  nil,
			fail:  false,
		},
		{
			name:  "Denied label with prefix",
			key:   "kubernetes.io/label",
			value: "true",
			want:  ErrNSNotAllowed,
			fail:  true,
		},
		{
			name:  "Invalid label key",
			key:   "invalid-key",
			value: "true",
			want:  ErrNSNotAllowed,
			fail:  true,
		},
		{
			name:  "Invalid label value",
			key:   "feature.node.kubernetes.io/label",
			value: "invalid value",
			want:  fmt.Errorf("invalid value \"invalid value\": value must be a valid label value"),
			fail:  true,
		},
		{
			name:  "Valid value label",
			key:   "feature.node.kubernetes.io/label",
			value: "true",
			want:  nil,
			fail:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Label(tt.key, tt.value)
			if err != tt.want {
				if tt.fail {
					return
				}
				t.Errorf("Label() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestExtendedResource(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  error
		fail  bool
	}{
		{
			name:  "Valid extended resource",
			key:   "feature.node.kubernetes.io/extended-resource",
			value: "123",
			want:  nil,
			fail:  false,
		},
		{
			name:  "Invalid extended resource key",
			key:   "invalid-key",
			value: "123",
			want:  ErrNSNotAllowed,
			fail:  true,
		},
		{
			name:  "Invalid extended resource value",
			key:   "feature.node.kubernetes.io/extended-resource",
			value: "invalid value",
			want:  fmt.Errorf("invalid value \"invalid value\": value must be a valid label value"),
			fail:  true,
		},
		{
			name:  "Denied extended resource key",
			key:   "kubernetes.io/extended-resource",
			value: "123",
			want:  ErrNSNotAllowed,
			fail:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtendedResource(tt.key, tt.value)
			if err != tt.want {
				if tt.fail {
					return
				}
				t.Errorf("ExtendedResource() = %v, want %v", err, tt.want)
			}
		})
	}
}
