/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	api "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
)

type BoolAssertionFuncf func(assert.TestingT, bool, string, ...interface{}) bool

type ValueAssertionFuncf func(assert.TestingT, interface{}, string, ...interface{}) bool

func TestCreateMatchExpression(t *testing.T) {
	type V = api.MatchValue
	type TC struct {
		op     api.MatchOp
		values V
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: api.MatchAny, err: assert.Nilf}, // #0
		{op: api.MatchAny, values: V{"1"}, err: assert.NotNilf},

		{op: api.MatchIn, err: assert.NotNilf},
		{op: api.MatchIn, values: V{"1"}, err: assert.Nilf},
		{op: api.MatchIn, values: V{"1", "2", "3", "4"}, err: assert.Nilf},

		{op: api.MatchNotIn, err: assert.NotNilf},
		{op: api.MatchNotIn, values: V{"1"}, err: assert.Nilf},
		{op: api.MatchNotIn, values: V{"1", "2"}, err: assert.Nilf},

		{op: api.MatchInRegexp, err: assert.NotNilf},
		{op: api.MatchInRegexp, values: V{"1"}, err: assert.Nilf},
		{op: api.MatchInRegexp, values: V{"()", "2", "3"}, err: assert.Nilf},
		{op: api.MatchInRegexp, values: V{"("}, err: assert.NotNilf},

		{op: api.MatchExists, err: assert.Nilf},
		{op: api.MatchExists, values: V{"1"}, err: assert.NotNilf},

		{op: api.MatchDoesNotExist, err: assert.Nilf},
		{op: api.MatchDoesNotExist, values: V{"1"}, err: assert.NotNilf},

		{op: api.MatchGt, err: assert.NotNilf},
		{op: api.MatchGt, values: V{"1"}, err: assert.Nilf},
		{op: api.MatchGt, values: V{"-10"}, err: assert.Nilf},
		{op: api.MatchGt, values: V{"1", "2"}, err: assert.NotNilf},
		{op: api.MatchGt, values: V{""}, err: assert.NotNilf},

		{op: api.MatchLt, err: assert.NotNilf},
		{op: api.MatchLt, values: V{"1"}, err: assert.Nilf},
		{op: api.MatchLt, values: V{"-1"}, err: assert.Nilf},
		{op: api.MatchLt, values: V{"1", "2", "3"}, err: assert.NotNilf},
		{op: api.MatchLt, values: V{"a"}, err: assert.NotNilf},

		{op: api.MatchGtLt, err: assert.NotNilf},
		{op: api.MatchGtLt, values: V{"1"}, err: assert.NotNilf},
		{op: api.MatchGtLt, values: V{"1", "2"}, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"2", "1"}, err: assert.NotNilf},
		{op: api.MatchGtLt, values: V{"1", "2", "3"}, err: assert.NotNilf},
		{op: api.MatchGtLt, values: V{"a", "2"}, err: assert.NotNilf},

		{op: api.MatchIsTrue, err: assert.Nilf},
		{op: api.MatchIsTrue, values: V{"1"}, err: assert.NotNilf},

		{op: api.MatchIsFalse, err: assert.Nilf},
		{op: api.MatchIsFalse, values: V{"1", "2"}, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		_, err := api.CreateMatchExpression(tc.op, tc.values...)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMatch(t *testing.T) {
	type V = api.MatchValue
	type TC struct {
		op     api.MatchOp
		values V
		input  interface{}
		valid  bool
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: api.MatchAny, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchAny, input: "2", valid: false, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchIn, values: V{"1"}, input: "2", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIn, values: V{"1"}, input: "2", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIn, values: V{"1", "2", "3"}, input: "2", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIn, values: V{"1", "2", "3"}, input: "2", valid: true, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchNotIn, values: V{"2"}, input: 2, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchNotIn, values: V{"1"}, input: 2, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchNotIn, values: V{"1", "2", "3"}, input: "2", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchNotIn, values: V{"1", "2", "3"}, input: "2", valid: true, result: assert.Falsef, err: assert.Nilf},

		{op: api.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-1", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-1", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-12", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchInRegexp, values: V{"val-[0-9]$", "al-[1-9]"}, input: "val-12", valid: true, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchExists, input: nil, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchExists, input: nil, valid: true, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchDoesNotExist, input: false, valid: false, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchDoesNotExist, input: false, valid: true, result: assert.Falsef, err: assert.Nilf},

		{op: api.MatchGt, values: V{"2"}, input: 3, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGt, values: V{"2"}, input: 2, valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGt, values: V{"2"}, input: 3, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchGt, values: V{"-10"}, input: -3, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchGt, values: V{"2"}, input: "3a", valid: true, result: assert.Falsef, err: assert.NotNilf},

		{op: api.MatchLt, values: V{"2"}, input: "1", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchLt, values: V{"2"}, input: "2", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchLt, values: V{"-10"}, input: -3, valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchLt, values: V{"2"}, input: "1", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchLt, values: V{"2"}, input: "1.0", valid: true, result: assert.Falsef, err: assert.NotNilf},

		{op: api.MatchGtLt, values: V{"1", "10"}, input: "1", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"1", "10"}, input: "1", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"1", "10"}, input: "10", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"1", "10"}, input: "2", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"1", "10"}, input: "1.0", valid: true, result: assert.Falsef, err: assert.NotNilf},

		{op: api.MatchIsTrue, input: true, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIsTrue, input: true, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchIsTrue, input: false, valid: true, result: assert.Falsef, err: assert.Nilf},

		{op: api.MatchIsFalse, input: "false", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIsFalse, input: "false", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchIsFalse, input: "true", valid: true, result: assert.Falsef, err: assert.Nilf},
	}

	for i, tc := range tcs {
		me := api.MustCreateMatchExpression(tc.op, tc.values...)
		res, err := me.Match(tc.valid, tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}

	// Check some special error cases separately because MustCreateMatch panics
	tcs = []TC{

		{op: api.MatchGt, values: V{"3.0"}, input: 1, valid: true},
		{op: api.MatchLt, values: V{"0x2"}, input: 1, valid: true},
		{op: api.MatchGtLt, values: V{"1", "str"}, input: 1, valid: true},
		{op: "non-existent-op", values: V{"1"}, input: 1, valid: true},
	}

	for i, tc := range tcs {
		me := api.MatchExpression{Op: tc.op, Value: tc.values}
		res, err := me.Match(tc.valid, tc.input)
		assert.Falsef(t, res, "err test case #%d (%v) failed", i, tc)
		assert.NotNilf(t, err, "err test case #%d (%v) failed", i, tc)
	}
}

func TestMatchKeys(t *testing.T) {
	type V = api.MatchValue
	type I = map[string]api.Nil
	type TC struct {
		op     api.MatchOp
		values V
		name   string
		input  I
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: api.MatchAny, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchExists, name: "foo", input: nil, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchExists, name: "foo", input: I{"bar": {}}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchExists, name: "foo", input: I{"bar": {}, "foo": {}}, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchDoesNotExist, name: "foo", input: nil, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchDoesNotExist, name: "foo", input: I{}, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchDoesNotExist, name: "foo", input: I{"bar": {}}, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchDoesNotExist, name: "foo", input: I{"bar": {}, "foo": {}}, result: assert.Falsef, err: assert.Nilf},

		// All other ops should return an error
		{op: api.MatchIn, values: V{"foo"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: api.MatchNotIn, values: V{"foo"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: api.MatchInRegexp, values: V{"foo"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: api.MatchGt, values: V{"1"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: api.MatchLt, values: V{"1"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: api.MatchGtLt, values: V{"1", "10"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: api.MatchIsTrue, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: api.MatchIsFalse, name: "foo", result: assert.Falsef, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		me := api.MustCreateMatchExpression(tc.op, tc.values...)
		res, err := me.MatchKeys(tc.name, tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMatchValues(t *testing.T) {
	type V = []string
	type I = map[string]string

	type TC struct {
		op     api.MatchOp
		values V
		name   string
		input  I
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: api.MatchAny, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchIn, values: V{"1", "2"}, name: "foo", input: I{"bar": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "3"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "2"}, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchNotIn, values: V{"1", "2"}, name: "foo", input: I{"bar": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchNotIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "3"}, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchNotIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "2"}, result: assert.Falsef, err: assert.Nilf},

		{op: api.MatchInRegexp, values: V{"1", "2"}, name: "foo", input: I{"bar": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchInRegexp, values: V{"1", "[0-8]"}, name: "foo", input: I{"foo": "9"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchInRegexp, values: V{"1", "[0-8]"}, name: "foo", input: I{"foo": "2"}, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchExists, name: "foo", input: I{"bar": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchExists, name: "foo", input: I{"foo": "1"}, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchDoesNotExist, name: "foo", input: nil, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchDoesNotExist, name: "foo", input: I{"foo": "1"}, result: assert.Falsef, err: assert.Nilf},

		{op: api.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "3"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "3", "foo": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "3", "foo": "3"}, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.Falsef, err: assert.NotNilf},

		{op: api.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "1", "foo": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "1", "foo": "1"}, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.Falsef, err: assert.NotNilf},

		{op: api.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1", "foo": "11"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1", "foo": "-11"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1", "foo": "1"}, result: assert.Truef, err: assert.Nilf},
		{op: api.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.Falsef, err: assert.NotNilf},

		{op: api.MatchIsTrue, name: "foo", result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIsTrue, name: "foo", input: I{"foo": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIsTrue, name: "foo", input: I{"foo": "true"}, result: assert.Truef, err: assert.Nilf},

		{op: api.MatchIsFalse, name: "foo", input: I{"foo": "true"}, result: assert.Falsef, err: assert.Nilf},
		{op: api.MatchIsFalse, name: "foo", input: I{"foo": "false"}, result: assert.Truef, err: assert.Nilf},
	}

	for i, tc := range tcs {
		me := api.MustCreateMatchExpression(tc.op, tc.values...)
		res, err := me.MatchValues(tc.name, tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMESMatchKeys(t *testing.T) {
	type I = map[string]api.Nil
	type MK = api.MatchedKey
	type O = []MK
	type TC struct {
		mes    string
		input  I
		output O
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{output: O{}, result: assert.Truef, err: assert.Nilf},

		{input: I{}, output: O{}, result: assert.Truef, err: assert.Nilf},

		{input: I{"foo": {}}, output: O{}, result: assert.Truef, err: assert.Nilf},

		{mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}, "buzz": {}},
			output: O{MK{Name: "bar"}, MK{Name: "foo"}},
			result: assert.Truef, err: assert.Nilf},

		{mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"foo": {}, "bar": {}, "baz": {}},
			output: nil,
			result: assert.Falsef, err: assert.Nilf},

		{mes: `
foo: { op: In, value: ["bar"] }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}},
			output: nil,
			result: assert.Falsef, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		mes := &api.MatchExpressionSet{}
		if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
			t.Fatalf("failed to parse data of test case #%d (%v): %v", i, tc, err)
		}

		res, out, err := mes.MatchGetKeys(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		assert.Equalf(t, tc.output, out, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)

		res, err = mes.MatchKeys(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMESMatchValues(t *testing.T) {
	type I = map[string]string
	type MV = api.MatchedValue
	type O = []MV
	type TC struct {
		mes    string
		input  I
		output O
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{output: O{}, result: assert.Truef, err: assert.Nilf},

		{input: I{}, output: O{}, result: assert.Truef, err: assert.Nilf},

		{input: I{"foo": "bar"}, output: O{}, result: assert.Truef, err: assert.Nilf},

		{mes: `
foo: { op: Exists }
bar: { op: In, value: ["val", "wal"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"bar": "val"},
			result: assert.Falsef, err: assert.Nilf},

		{mes: `
foo: { op: Exists }
bar: { op: In, value: ["val", "wal"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"foo": "1", "bar": "val", "baz": "123", "buzz": "light"},
			output: O{MV{Name: "bar", Value: "val"}, MV{Name: "baz", Value: "123"}, MV{Name: "foo", Value: "1"}},
			result: assert.Truef, err: assert.Nilf},

		{mes: `
foo: { op: Exists }
bar: { op: In, value: ["val"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"foo": "1", "bar": "val", "baz": "123.0"},
			result: assert.Falsef, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		mes := &api.MatchExpressionSet{}
		if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
			t.Fatalf("failed to parse data of test case #%d (%v): %v", i, tc, err)
		}

		res, out, err := mes.MatchGetValues(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		assert.Equalf(t, tc.output, out, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)

		res, err = mes.MatchValues(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMESMatchInstances(t *testing.T) {
	type I = api.InstanceFeature
	type MI = api.MatchedInstance
	type O = []MI
	type A = map[string]string
	type TC struct {
		mes    string
		input  []I
		output O
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{output: O{}, result: assert.Falsef, err: assert.Nilf}, // nil instances -> false

		{input: []I{}, output: O{}, result: assert.Falsef, err: assert.Nilf}, // zero instances -> false

		{input: []I{I{Attributes: A{}}}, output: O{A{}}, result: assert.Truef, err: assert.Nilf}, // one "empty" instance

		{mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "1"}}},
			output: O{},
			result: assert.Falsef, err: assert.Nilf},

		{mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"foo": "2", "bar": "1"}}},
			output: O{A{"foo": "2", "bar": "1"}},
			result: assert.Truef, err: assert.Nilf},

		{mes: `
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "0x1"}}},
			result: assert.Falsef, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		mes := &api.MatchExpressionSet{}
		if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
			t.Fatalf("failed to parse data of test case #%d (%v): %v", i, tc, err)
		}

		out, err := mes.MatchGetInstances(tc.input)
		assert.Equalf(t, tc.output, out, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)

		res, err := mes.MatchInstances(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}
