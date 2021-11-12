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

package expression_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/node-feature-discovery/pkg/api/feature"
	e "sigs.k8s.io/node-feature-discovery/source/custom/expression"
)

type BoolAssertionFuncf func(assert.TestingT, bool, string, ...interface{}) bool

type ValueAssertionFuncf func(assert.TestingT, interface{}, string, ...interface{}) bool

func TestCreateMatchExpression(t *testing.T) {
	type V = e.MatchValue
	type TC struct {
		op     e.MatchOp
		values V
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: e.MatchAny, err: assert.Nilf}, // #0
		{op: e.MatchAny, values: V{"1"}, err: assert.NotNilf},

		{op: e.MatchIn, err: assert.NotNilf},
		{op: e.MatchIn, values: V{"1"}, err: assert.Nilf},
		{op: e.MatchIn, values: V{"1", "2", "3", "4"}, err: assert.Nilf},

		{op: e.MatchNotIn, err: assert.NotNilf},
		{op: e.MatchNotIn, values: V{"1"}, err: assert.Nilf},
		{op: e.MatchNotIn, values: V{"1", "2"}, err: assert.Nilf},

		{op: e.MatchInRegexp, err: assert.NotNilf},
		{op: e.MatchInRegexp, values: V{"1"}, err: assert.Nilf},
		{op: e.MatchInRegexp, values: V{"()", "2", "3"}, err: assert.Nilf},
		{op: e.MatchInRegexp, values: V{"("}, err: assert.NotNilf},

		{op: e.MatchExists, err: assert.Nilf},
		{op: e.MatchExists, values: V{"1"}, err: assert.NotNilf},

		{op: e.MatchDoesNotExist, err: assert.Nilf},
		{op: e.MatchDoesNotExist, values: V{"1"}, err: assert.NotNilf},

		{op: e.MatchGt, err: assert.NotNilf},
		{op: e.MatchGt, values: V{"1"}, err: assert.Nilf},
		{op: e.MatchGt, values: V{"-10"}, err: assert.Nilf},
		{op: e.MatchGt, values: V{"1", "2"}, err: assert.NotNilf},
		{op: e.MatchGt, values: V{""}, err: assert.NotNilf},

		{op: e.MatchLt, err: assert.NotNilf},
		{op: e.MatchLt, values: V{"1"}, err: assert.Nilf},
		{op: e.MatchLt, values: V{"-1"}, err: assert.Nilf},
		{op: e.MatchLt, values: V{"1", "2", "3"}, err: assert.NotNilf},
		{op: e.MatchLt, values: V{"a"}, err: assert.NotNilf},

		{op: e.MatchGtLt, err: assert.NotNilf},
		{op: e.MatchGtLt, values: V{"1"}, err: assert.NotNilf},
		{op: e.MatchGtLt, values: V{"1", "2"}, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"2", "1"}, err: assert.NotNilf},
		{op: e.MatchGtLt, values: V{"1", "2", "3"}, err: assert.NotNilf},
		{op: e.MatchGtLt, values: V{"a", "2"}, err: assert.NotNilf},

		{op: e.MatchIsTrue, err: assert.Nilf},
		{op: e.MatchIsTrue, values: V{"1"}, err: assert.NotNilf},

		{op: e.MatchIsFalse, err: assert.Nilf},
		{op: e.MatchIsFalse, values: V{"1", "2"}, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		_, err := e.CreateMatchExpression(tc.op, tc.values...)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMatch(t *testing.T) {
	type V = e.MatchValue
	type TC struct {
		op     e.MatchOp
		values V
		input  interface{}
		valid  bool
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: e.MatchAny, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchAny, input: "2", valid: false, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchIn, values: V{"1"}, input: "2", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIn, values: V{"1"}, input: "2", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIn, values: V{"1", "2", "3"}, input: "2", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIn, values: V{"1", "2", "3"}, input: "2", valid: true, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchNotIn, values: V{"2"}, input: 2, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchNotIn, values: V{"1"}, input: 2, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchNotIn, values: V{"1", "2", "3"}, input: "2", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchNotIn, values: V{"1", "2", "3"}, input: "2", valid: true, result: assert.Falsef, err: assert.Nilf},

		{op: e.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-1", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-1", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchInRegexp, values: V{"val-[0-9]$"}, input: "val-12", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchInRegexp, values: V{"val-[0-9]$", "al-[1-9]"}, input: "val-12", valid: true, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchExists, input: nil, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchExists, input: nil, valid: true, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchDoesNotExist, input: false, valid: false, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchDoesNotExist, input: false, valid: true, result: assert.Falsef, err: assert.Nilf},

		{op: e.MatchGt, values: V{"2"}, input: 3, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGt, values: V{"2"}, input: 2, valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGt, values: V{"2"}, input: 3, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchGt, values: V{"-10"}, input: -3, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchGt, values: V{"2"}, input: "3a", valid: true, result: assert.Falsef, err: assert.NotNilf},

		{op: e.MatchLt, values: V{"2"}, input: "1", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchLt, values: V{"2"}, input: "2", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchLt, values: V{"-10"}, input: -3, valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchLt, values: V{"2"}, input: "1", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchLt, values: V{"2"}, input: "1.0", valid: true, result: assert.Falsef, err: assert.NotNilf},

		{op: e.MatchGtLt, values: V{"1", "10"}, input: "1", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"1", "10"}, input: "1", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"1", "10"}, input: "10", valid: true, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"1", "10"}, input: "2", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"1", "10"}, input: "1.0", valid: true, result: assert.Falsef, err: assert.NotNilf},

		{op: e.MatchIsTrue, input: true, valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIsTrue, input: true, valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchIsTrue, input: false, valid: true, result: assert.Falsef, err: assert.Nilf},

		{op: e.MatchIsFalse, input: "false", valid: false, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIsFalse, input: "false", valid: true, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchIsFalse, input: "true", valid: true, result: assert.Falsef, err: assert.Nilf},
	}

	for i, tc := range tcs {
		me := e.MustCreateMatchExpression(tc.op, tc.values...)
		res, err := me.Match(tc.valid, tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}

	// Check some special error cases separately because MustCreateMatch panics
	tcs = []TC{

		{op: e.MatchGt, values: V{"3.0"}, input: 1, valid: true},
		{op: e.MatchLt, values: V{"0x2"}, input: 1, valid: true},
		{op: e.MatchGtLt, values: V{"1", "str"}, input: 1, valid: true},
		{op: "non-existent-op", values: V{"1"}, input: 1, valid: true},
	}

	for i, tc := range tcs {
		me := e.MatchExpression{Op: tc.op, Value: tc.values}
		res, err := me.Match(tc.valid, tc.input)
		assert.Falsef(t, res, "err test case #%d (%v) failed", i, tc)
		assert.NotNilf(t, err, "err test case #%d (%v) failed", i, tc)
	}
}

func TestMatchKeys(t *testing.T) {
	type V = e.MatchValue
	type I = map[string]feature.Nil
	type TC struct {
		op     e.MatchOp
		values V
		name   string
		input  I
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: e.MatchAny, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchExists, name: "foo", input: nil, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchExists, name: "foo", input: I{"bar": {}}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchExists, name: "foo", input: I{"bar": {}, "foo": {}}, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchDoesNotExist, name: "foo", input: nil, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchDoesNotExist, name: "foo", input: I{}, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchDoesNotExist, name: "foo", input: I{"bar": {}}, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchDoesNotExist, name: "foo", input: I{"bar": {}, "foo": {}}, result: assert.Falsef, err: assert.Nilf},

		// All other ops should return an error
		{op: e.MatchIn, values: V{"foo"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: e.MatchNotIn, values: V{"foo"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: e.MatchInRegexp, values: V{"foo"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: e.MatchGt, values: V{"1"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: e.MatchLt, values: V{"1"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: e.MatchGtLt, values: V{"1", "10"}, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: e.MatchIsTrue, name: "foo", result: assert.Falsef, err: assert.NotNilf},
		{op: e.MatchIsFalse, name: "foo", result: assert.Falsef, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		me := e.MustCreateMatchExpression(tc.op, tc.values...)
		res, err := me.MatchKeys(tc.name, tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMatchValues(t *testing.T) {
	type V = []string
	type I = map[string]string

	type TC struct {
		op     e.MatchOp
		values V
		name   string
		input  I
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{op: e.MatchAny, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchIn, values: V{"1", "2"}, name: "foo", input: I{"bar": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "3"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "2"}, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchNotIn, values: V{"1", "2"}, name: "foo", input: I{"bar": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchNotIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "3"}, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchNotIn, values: V{"1", "2"}, name: "foo", input: I{"foo": "2"}, result: assert.Falsef, err: assert.Nilf},

		{op: e.MatchInRegexp, values: V{"1", "2"}, name: "foo", input: I{"bar": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchInRegexp, values: V{"1", "[0-8]"}, name: "foo", input: I{"foo": "9"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchInRegexp, values: V{"1", "[0-8]"}, name: "foo", input: I{"foo": "2"}, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchExists, name: "foo", input: I{"bar": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchExists, name: "foo", input: I{"foo": "1"}, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchDoesNotExist, name: "foo", input: nil, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchDoesNotExist, name: "foo", input: I{"foo": "1"}, result: assert.Falsef, err: assert.Nilf},

		{op: e.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "3"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "3", "foo": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "3", "foo": "3"}, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchGt, values: V{"2"}, name: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.Falsef, err: assert.NotNilf},

		{op: e.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "1", "foo": "2"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "1", "foo": "1"}, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchLt, values: V{"2"}, name: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.Falsef, err: assert.NotNilf},

		{op: e.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1", "foo": "11"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1", "foo": "-11"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "1", "foo": "1"}, result: assert.Truef, err: assert.Nilf},
		{op: e.MatchGtLt, values: V{"-10", "10"}, name: "foo", input: I{"bar": "str", "foo": "str"}, result: assert.Falsef, err: assert.NotNilf},

		{op: e.MatchIsTrue, name: "foo", result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIsTrue, name: "foo", input: I{"foo": "1"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIsTrue, name: "foo", input: I{"foo": "true"}, result: assert.Truef, err: assert.Nilf},

		{op: e.MatchIsFalse, name: "foo", input: I{"foo": "true"}, result: assert.Falsef, err: assert.Nilf},
		{op: e.MatchIsFalse, name: "foo", input: I{"foo": "false"}, result: assert.Truef, err: assert.Nilf},
	}

	for i, tc := range tcs {
		me := e.MustCreateMatchExpression(tc.op, tc.values...)
		res, err := me.MatchValues(tc.name, tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMESMatchKeys(t *testing.T) {
	type I = map[string]feature.Nil
	type TC struct {
		mes    string
		input  I
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{result: assert.Truef, err: assert.Nilf},

		{input: I{"foo": {}}, result: assert.Truef, err: assert.Nilf},

		{mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}},
			result: assert.Truef, err: assert.Nilf},

		{mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"foo": {}, "bar": {}, "baz": {}},
			result: assert.Falsef, err: assert.Nilf},

		{mes: `
foo: { op: In, value: ["bar"] }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}},
			result: assert.Falsef, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		mes := &e.MatchExpressionSet{}
		if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
			t.Fatalf("failed to parse data of test case #%d (%v): %v", i, tc, err)
		}

		res, err := mes.MatchKeys(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMESMatchValues(t *testing.T) {
	type I = map[string]string
	type TC struct {
		mes    string
		input  I
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{result: assert.Truef, err: assert.Nilf},

		{input: I{"foo": "bar"}, result: assert.Truef, err: assert.Nilf},

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
			input:  I{"foo": "1", "bar": "val", "baz": "123"},
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
		mes := &e.MatchExpressionSet{}
		if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
			t.Fatalf("failed to parse data of test case #%d (%v): %v", i, tc, err)
		}

		res, err := mes.MatchValues(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}

func TestMESMatchInstances(t *testing.T) {
	type I = feature.InstanceFeature
	type A = map[string]string
	type TC struct {
		mes    string
		input  []I
		result BoolAssertionFuncf
		err    ValueAssertionFuncf
	}

	tcs := []TC{
		{result: assert.Falsef, err: assert.Nilf}, // nil instances -> false

		{input: []I{}, result: assert.Falsef, err: assert.Nilf}, // zero instances -> false

		{input: []I{I{Attributes: A{}}}, result: assert.Truef, err: assert.Nilf}, // one "empty" instance

		{mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "1"}}},
			result: assert.Falsef, err: assert.Nilf},

		{mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"foo": "2", "bar": "1"}}},
			result: assert.Truef, err: assert.Nilf},

		{mes: `
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "0x1"}}},
			result: assert.Falsef, err: assert.NotNilf},
	}

	for i, tc := range tcs {
		mes := &e.MatchExpressionSet{}
		if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
			t.Fatalf("failed to parse data of test case #%d (%v): %v", i, tc, err)
		}

		res, err := mes.MatchInstances(tc.input)
		tc.result(t, res, "test case #%d (%v) failed", i, tc)
		tc.err(t, err, "test case #%d (%v) failed", i, tc)
	}
}
