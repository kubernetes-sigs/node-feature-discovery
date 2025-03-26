package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type ValueAssertionFunc func(assert.TestingT, interface{}, ...interface{}) bool

func TestExtractVersion(t *testing.T) {
	tcs := []struct {
		name   string
		input  string
		result string
		err    ValueAssertionFunc
	}{
		{name: "1", input: "1", result: "1", err: assert.Nil},
		{name: "2", input: "1.2", result: "1.2", err: assert.Nil},
		{name: "3", input: "1.2.3", result: "1.2.3", err: assert.Nil},
		{name: "4", input: "1.2.3-flavor", result: "1.2.3", err: assert.Nil},
		{name: "5", input: "1.2.3-flavor.2.3.4", result: "1.2.3", err: assert.Nil},
		{name: "6", input: "flavor-1.2.3", result: "", err: assert.NotNil},
		{name: "7", input: "A.2.3", result: "", err: assert.NotNil},
		{name: "8", input: "str", result: "", err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ExtractVersion(tc.input)
			assert.Equal(t, result, tc.result)
			tc.err(t, err)
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tcs := []struct {
		name   string
		v1     string
		v2     string
		result VersionComparisonResult
	}{
		{name: "1", v1: "1", v2: "2", result: CmpLt},
		{name: "2", v1: "2", v2: "2", result: CmpEq},
		{name: "3", v1: "3", v2: "2", result: CmpGt},
		{name: "4", v1: "1.9", v2: "2.0", result: CmpLt},
		{name: "5", v1: "2.0", v2: "2.0", result: CmpEq},
		{name: "6", v1: "2.1", v2: "2.0", result: CmpGt},
		{name: "7", v1: "1.9.9", v2: "2.0.0", result: CmpLt},
		{name: "8", v1: "2.0.0", v2: "2.0.0", result: CmpEq},
		{name: "9", v1: "2.0.1", v2: "2.0.0", result: CmpGt},
		{name: "10", v1: "1", v2: "2.0", result: CmpLt},
		{name: "11", v1: "2", v2: "2.0", result: CmpEq},
		{name: "12", v1: "3", v2: "2.0", result: CmpGt},
		{name: "13", v1: "1", v2: "2.0.0", result: CmpLt},
		{name: "14", v1: "2", v2: "2.0.0", result: CmpEq},
		{name: "15", v1: "3", v2: "2.0.0", result: CmpGt},
		{name: "16", v1: "1.9.9", v2: "2", result: CmpLt},
		{name: "17", v1: "2.0.0", v2: "2", result: CmpEq},
		{name: "18", v1: "2.0.1", v2: "2", result: CmpGt},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := CompareVersions(tc.v1, tc.v2)
			assert.Equal(t, result, tc.result)
		})
	}
}
