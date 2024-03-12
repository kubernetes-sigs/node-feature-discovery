package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestAnnotation(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  interface{}
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
			want:  "invalid value \"invalid value\": ",
		},
		{
			name:  "Denied annotation key",
			key:   "kubernetes.io/denied",
			value: "true",
			want:  ErrNSNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Annotation(tt.key, tt.value)
			if str, ok := tt.want.(string); ok {
				assert.ErrorContains(t, err, str)
			} else {
				assert.Equal(t, tt.want, err)
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
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  interface{}
	}{
		{
			name:  "Valid label",
			key:   "feature.node.kubernetes.io/label",
			value: "true",
			want:  nil,
		},
		{
			name:  "Valid vendor label",
			key:   "vendor.io/label",
			value: "true",
			want:  nil,
		},
		{
			name:  "Denied label with prefix",
			key:   "kubernetes.io/label",
			value: "true",
			want:  ErrNSNotAllowed,
		},
		{
			name:  "Invalid label key",
			key:   "invalid-key",
			value: "true",
			want:  ErrUnprefixedKeysNotAllowed,
		},
		{
			name:  "Invalid label value",
			key:   "feature.node.kubernetes.io/label",
			value: "invalid value",
			want:  "invalid value \"invalid value\": ",
		},
		{
			name:  "Valid value label",
			key:   "feature.node.kubernetes.io/label",
			value: "true",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Label(tt.key, tt.value)
			if str, ok := tt.want.(string); ok {
				assert.ErrorContains(t, err, str)
			} else {
				assert.Equal(t, tt.want, err)
			}
		})
	}
}

func TestExtendedResource(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  interface{}
	}{
		{
			name:  "Valid extended resource",
			key:   "feature.node.kubernetes.io/extended-resource",
			value: "123",
			want:  nil,
		},
		{
			name:  "Invalid extended resource key",
			key:   "invalid-key",
			value: "123",
			want:  ErrUnprefixedKeysNotAllowed,
		},
		{
			name:  "Invalid extended resource value",
			key:   "feature.node.kubernetes.io/extended-resource",
			value: "invalid value",
			want:  "invalid value \"invalid value\": ",
		},
		{
			name:  "Denied extended resource key",
			key:   "kubernetes.io/extended-resource",
			value: "123",
			want:  ErrNSNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtendedResource(tt.key, tt.value)
			if str, ok := tt.want.(string); ok {
				assert.ErrorContains(t, err, str)
			} else {
				assert.Equal(t, tt.want, err)
			}
		})
	}
}
