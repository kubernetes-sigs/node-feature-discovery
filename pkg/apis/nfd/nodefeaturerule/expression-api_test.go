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

package nodefeaturerule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
	api "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/nodefeaturerule"
)

type BoolAssertionFunc func(assert.TestingT, bool, ...interface{}) bool

type ValueAssertionFunc func(assert.TestingT, interface{}, ...interface{}) bool

func TestMatchKeys(t *testing.T) {
	type I = map[string]nfdv1alpha1.Nil
	type O = []api.MatchedElement
	type TC struct {
		name   string
		mes    string
		input  I
		output O
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{
			name:   "empty expression and nil input",
			output: O{},
			result: assert.True,
			err:    assert.Nil,
		},
		{
			name:   "empty expression and empty input",
			input:  I{},
			output: O{},
			result: assert.True,
			err:    assert.Nil,
		},
		{
			name:   "empty expression with non-empty input",
			input:  I{"foo": {}},
			output: O{},
			result: assert.True,
			err:    assert.Nil,
		},
		{
			name: "expressions match",
			mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}, "buzz": {}},
			output: O{{"Name": "bar"}, {"Name": "foo"}},
			result: assert.True,
			err:    assert.Nil,
		},
		{
			name: "expression does not match",
			mes: `
foo: { op: DoesNotExist }
bar: { op: Exists }
`,
			input:  I{"foo": {}, "bar": {}, "baz": {}},
			output: nil,
			result: assert.False,
			err:    assert.Nil,
		},
		{
			name: "op that never matches",
			mes: `
foo: { op: In, value: ["bar"] }
bar: { op: Exists }
`,
			input:  I{"bar": {}, "baz": {}},
			output: nil,
			result: assert.False,
			err:    assert.Nil,
		},
		{
			name: "error in expression",
			mes: `
foo: { op: Exists, value: ["bar"] }
bar: { op: Exists }
`,
			input:  I{"bar": {}},
			output: nil,
			result: assert.False,
			err:    assert.NotNil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mes := &nfdv1alpha1.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			res, out, err := api.MatchGetKeys(mes, tc.input)
			tc.result(t, res)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err = api.MatchKeys(mes, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMatchValues(t *testing.T) {
	type I = map[string]string
	type O = []api.MatchedElement
	type TC struct {
		name   string
		mes    string
		input  I
		output O
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", output: O{}, result: assert.True, err: assert.Nil},

		{name: "2", input: I{}, output: O{}, result: assert.True, err: assert.Nil},

		{name: "3", input: I{"foo": "bar"}, output: O{}, result: assert.True, err: assert.Nil},

		{name: "4",
			mes: `
foo: { op: Exists }
bar: { op: In, value: ["val", "wal"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"bar": "val"},
			result: assert.False, err: assert.Nil},

		{name: "5",
			mes: `
foo: { op: Exists }
bar: { op: In, value: ["val", "wal"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"foo": "1", "bar": "val", "baz": "123", "buzz": "light"},
			output: O{{"Name": "bar", "Value": "val"}, {"Name": "baz", "Value": "123"}, {"Name": "foo", "Value": "1"}},
			result: assert.True, err: assert.Nil},

		{name: "5",
			mes: `
foo: { op: Exists }
bar: { op: In, value: ["val"] }
baz: { op: Gt, value: ["10"] }
`,
			input:  I{"foo": "1", "bar": "val", "baz": "123.0"},
			result: assert.False, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mes := &nfdv1alpha1.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			res, out, err := api.MatchGetValues(mes, tc.input)
			tc.result(t, res)
			assert.Equal(t, tc.output, out)
			tc.err(t, err)

			res, err = api.MatchValues(mes, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMatchInstances(t *testing.T) {
	type I = nfdv1alpha1.InstanceFeature
	type O = []api.MatchedElement
	type A = map[string]string
	type TC struct {
		name   string
		mes    string
		input  []I
		output O
		result BoolAssertionFunc
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{name: "1", output: O{}, result: assert.False, err: assert.Nil}, // nil instances -> false

		{name: "2", input: []I{}, output: O{}, result: assert.False, err: assert.Nil}, // zero instances -> false

		{name: "3", input: []I{I{Attributes: A{}}}, output: O{A{}}, result: assert.True, err: assert.Nil}, // one "empty" instance

		{name: "4",
			mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "1"}}},
			output: O{},
			result: assert.False, err: assert.Nil},

		{name: "5",
			mes: `
foo: { op: Exists }
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"foo": "2", "bar": "1"}}},
			output: O{A{"foo": "2", "bar": "1"}},
			result: assert.True, err: assert.Nil},

		{name: "6",
			mes: `
bar: { op: Lt, value: ["10"] }
`,
			input:  []I{I{Attributes: A{"foo": "1"}}, I{Attributes: A{"bar": "0x1"}}},
			result: assert.False, err: assert.NotNil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mes := &nfdv1alpha1.MatchExpressionSet{}
			if err := yaml.Unmarshal([]byte(tc.mes), mes); err != nil {
				t.Fatal("failed to parse data of test case")
			}

			res, out, err := api.MatchGetInstances(mes, tc.input)
			assert.Equal(t, tc.output, out)
			tc.result(t, res)
			tc.err(t, err)

			res, err = api.MatchInstances(mes, tc.input)
			tc.result(t, res)
			tc.err(t, err)
		})
	}
}

func TestMatchKeyNames(t *testing.T) {
	type O = []api.MatchedElement
	type I = map[string]nfdv1alpha1.Nil

	type TC struct {
		name   string
		me     *nfdv1alpha1.MatchExpression
		input  I
		result bool
		output O
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{
			name:   "empty input",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			input:  I{},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name:   "MatchAny",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			input:  I{"key1": {}, "key2": {}},
			result: true,
			output: O{{"Name": "key1"}, {"Name": "key2"}},
			err:    assert.Nil,
		},
		{
			name:   "MatchExists",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			input:  I{"key1": {}, "key2": {}},
			result: true,
			output: O{{"Name": "key1"}, {"Name": "key2"}},
			err:    assert.Nil,
		},
		{
			name:   "MatchDoesNotExist",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchDoesNotExist},
			input:  I{"key1": {}, "key2": {}},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name:   "MatchIn matches",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"key1"}},
			input:  I{"key1": {}, "key2": {}},
			result: true,
			output: O{{"Name": "key1"}},
			err:    assert.Nil,
		},
		{
			name:   "MatchIn no match",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"key3"}},
			input:  I{"key1": {}, "key2": {}},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name:   "MatchNotIn",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchNotIn, Value: nfdv1alpha1.MatchValue{"key1"}},
			input:  I{"key1": {}, "key2": {}},
			result: true,
			output: O{{"Name": "key2"}},
			err:    assert.Nil,
		},
		{
			name:   "error",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists, Value: nfdv1alpha1.MatchValue{"key1"}},
			input:  I{"key1": {}, "key2": {}},
			result: false,
			output: nil,
			err:    assert.NotNil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res, ret, err := api.MatchKeyNames(tc.me, tc.input)
			assert.Equal(t, tc.result, res)
			assert.Equal(t, tc.output, ret)
			tc.err(t, err)
		})
	}
}

func TestMatchValueNames(t *testing.T) {
	type O = []api.MatchedElement
	type I = map[string]string

	type TC struct {
		name   string
		me     *nfdv1alpha1.MatchExpression
		input  I
		result bool
		output O
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{
			name:   "empty input",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			input:  I{},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name:   "MatchExists",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			input:  I{"key1": "val1", "key2": "val2"},
			result: true,
			output: O{{"Name": "key1", "Value": "val1"}, {"Name": "key2", "Value": "val2"}},
			err:    assert.Nil,
		},
		{
			name:   "MatchDoesNotExist",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchDoesNotExist},
			input:  I{"key1": "val1", "key2": "val2"},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name:   "MatchIn matches",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"key1"}},
			input:  I{"key1": "val1", "key2": "val2"},
			result: true,
			output: O{{"Name": "key1", "Value": "val1"}},
			err:    assert.Nil,
		},
		{
			name:   "MatchIn no match",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"key3"}},
			input:  I{"key1": "val1", "key2": "val2"},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name:   "MatchNotIn",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchNotIn, Value: nfdv1alpha1.MatchValue{"key1"}},
			input:  I{"key1": "val1", "key2": "val2"},
			result: true,
			output: O{{"Name": "key2", "Value": "val2"}},
			err:    assert.Nil,
		},
		{
			name:   "error",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchNotIn},
			input:  I{"key1": "val1", "key2": "val2"},
			result: false,
			output: nil,
			err:    assert.NotNil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res, ret, err := api.MatchValueNames(tc.me, tc.input)
			assert.Equal(t, tc.result, res)
			assert.Equal(t, tc.output, ret)
			tc.err(t, err)
		})
	}
}

func TestMatchInstanceAttributeNames(t *testing.T) {
	type O = []api.MatchedElement
	type I = []nfdv1alpha1.InstanceFeature
	type A = map[string]string

	type TC struct {
		name   string
		me     *nfdv1alpha1.MatchExpression
		input  I
		result bool
		output O
		err    ValueAssertionFunc
	}

	tcs := []TC{
		{
			name:   "empty input",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			input:  I{},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name: "no match",
			me: &nfdv1alpha1.MatchExpression{
				Op:    nfdv1alpha1.MatchIn,
				Value: nfdv1alpha1.MatchValue{"foo"},
			},
			input: I{
				{Attributes: A{"bar": "1"}},
				{Attributes: A{"baz": "2"}},
			},
			result: false,
			output: O{},
			err:    assert.Nil,
		},
		{
			name: "match",
			me: &nfdv1alpha1.MatchExpression{
				Op:    nfdv1alpha1.MatchIn,
				Value: nfdv1alpha1.MatchValue{"foo"},
			},
			input: I{
				{Attributes: A{"foo": "1"}},
				{Attributes: A{"bar": "2"}},
				{Attributes: A{"foo": "3", "baz": "4"}},
			},
			result: true,
			output: O{
				{"foo": "1"},
				{"foo": "3", "baz": "4"},
			},
			err: assert.Nil,
		},
		{
			name: "error",
			me: &nfdv1alpha1.MatchExpression{
				Op: nfdv1alpha1.MatchIn,
			},
			input: I{
				{Attributes: A{"foo": "1"}},
			},
			output: nil,
			err:    assert.NotNil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			match, matched, err := api.MatchInstanceAttributeNames(tc.me, tc.input)
			assert.Equal(t, tc.result, match)
			assert.Equal(t, tc.output, matched)
			tc.err(t, err)
		})
	}
}

func TestMatchMulti(t *testing.T) {
	type O = []api.MatchedElement
	type IK = map[string]nfdv1alpha1.Nil
	type IV = map[string]string
	type II = []nfdv1alpha1.InstanceFeature
	type A = map[string]string

	type TC struct {
		name           string
		mes            *nfdv1alpha1.MatchExpressionSet
		inputKeys      IK
		inputValues    IV
		inputInstances II
		output         O
		result         BoolAssertionFunc
		expectErr      bool
	}

	tcs := []TC{
		{
			name:   "empty expression and nil input",
			mes:    &nfdv1alpha1.MatchExpressionSet{},
			output: O{},
			result: assert.False,
		},
		{
			name:           "empty expression and empty input keys",
			mes:            &nfdv1alpha1.MatchExpressionSet{},
			inputKeys:      IK{},
			inputValues:    IV{},
			inputInstances: II{},
			output:         O{},
			result:         assert.True,
		},
		{
			name:        "empty expression and empty input values",
			mes:         &nfdv1alpha1.MatchExpressionSet{},
			inputValues: IV{},
			output:      O{},
			result:      assert.True,
		},
		{
			name:           "empty expression and empty input instances",
			mes:            &nfdv1alpha1.MatchExpressionSet{},
			inputInstances: II{},
			output:         O{},
			result:         assert.False,
		},
		{
			name:           "empty expression and one input instance with empty attributes",
			mes:            &nfdv1alpha1.MatchExpressionSet{},
			inputInstances: II{{Attributes: A{}}},
			output:         O{A{}},
			result:         assert.True,
		},
		{
			name:        "empty expression",
			mes:         &nfdv1alpha1.MatchExpressionSet{},
			inputValues: IV{"foo": "bar"},
			output:      O{},
			result:      assert.True,
		},
		{
			name: "keys match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchDoesNotExist},
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			},
			inputKeys: IK{"bar": {}, "baz": {}, "buzz": {}},
			output:    O{{"Name": "bar"}, {"Name": "foo"}},
			result:    assert.True,
		},
		{
			name: "keys do not match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchDoesNotExist},
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			},
			inputKeys: IK{"foo": {}, "bar": {}, "baz": {}},
			output:    O{},
			result:    assert.False,
		},
		{
			name: "keys error",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists, Value: nfdv1alpha1.MatchValue{"val"}},
			},
			inputKeys: IK{"bar": {}, "baz": {}, "buzz": {}},
			output:    nil,
			result:    assert.False,
			expectErr: true,
		},
		{
			name: "values match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"val", "wal"}},
				"baz": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchGt, Value: nfdv1alpha1.MatchValue{"10"}},
			},
			inputValues: IV{"foo": "1", "bar": "val", "baz": "123", "buzz": "light"},
			output:      O{{"Name": "bar", "Value": "val"}, {"Name": "baz", "Value": "123"}, {"Name": "foo", "Value": "1"}},
			result:      assert.True,
		},
		{
			name: "values do not match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"val", "wal"}},
				"baz": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchGt, Value: nfdv1alpha1.MatchValue{"10"}},
			},
			inputValues: IV{"bar": "val"},
			output:      O{},
			result:      assert.False,
		},
		{
			name: "values error",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn},
			},
			inputValues: IV{"foo": "1", "bar": "val", "baz": "123", "buzz": "light"},
			output:      nil,
			result:      assert.False,
			expectErr:   true,
		},
		{
			name: "instances match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchLt, Value: nfdv1alpha1.MatchValue{"10"}},
			},
			inputInstances: II{{Attributes: A{"foo": "1"}}, {Attributes: A{"foo": "2", "bar": "1"}}},
			output:         O{A{"foo": "2", "bar": "1"}},
			result:         assert.True,
		},
		{
			name: "instances do not match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
				"baz": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchLt, Value: nfdv1alpha1.MatchValue{"10"}},
			},
			inputInstances: II{{Attributes: A{"foo": "1"}}, {Attributes: A{"bar": "1"}}},
			output:         O{},
			result:         assert.False,
		},
		{
			name: "instances error",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn},
			},
			inputInstances: II{{Attributes: A{"foo": "1"}}, {Attributes: A{"foo": "2", "bar": "1"}}},
			output:         nil,
			result:         assert.False,
			expectErr:      true,
		},
		{
			name: "multi: keys and values either matches",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"1"}},
				"baz": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			},
			inputKeys:   IK{"bar": {}, "baz": {}, "qux": {}},
			inputValues: IV{"foo": "1", "bar": "2", "quux": "3"},
			output:      O{{"Name": "baz"}, {"Name": "foo", "Value": "1"}},
			result:      assert.True,
		},
		{
			name: "multi: keys and values duplicate match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"1"}},
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			},
			inputKeys:   IK{"bar": {}, "baz": {}, "qux": {}},
			inputValues: IV{"foo": "1", "bar": "2", "quux": "3"},
			output:      O{{"Name": "bar"}, {"Name": "bar", "Value": "2"}, {"Name": "foo", "Value": "1"}},
			result:      assert.True,
		},
		{
			name: "multi: keys and values NotIn match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchNotIn, Value: nfdv1alpha1.MatchValue{"1", "3"}},
			},
			inputKeys:   IK{"bar": {}, "baz": {}, "qux": {}},
			inputValues: IV{"foo": "1", "bar": "2", "quux": "3"},
			output:      O{{"Name": "bar", "Value": "2"}},
			result:      assert.True,
		},
		{
			name: "multi: keys and values NotIn does not match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchNotIn, Value: nfdv1alpha1.MatchValue{"2"}},
			},
			inputKeys:   IK{"bar": {}, "baz": {}, "qux": {}},
			inputValues: IV{"foo": "1", "bar": "2", "quux": "3"},
			output:      O{},
			result:      assert.False,
		},
		{
			name: "multi: keys and values DoesNotExist match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"xyzzy": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchDoesNotExist},
			},
			inputKeys:   IK{"bar": {}, "baz": {}, "qux": {}},
			inputValues: IV{"foo": "1", "bar": "2", "quux": "3"},
			output:      O{{"Name": "xyzzy"}, {"Name": "xyzzy", "Value": ""}},
			result:      assert.True,
		},
		{
			name: "multi: keys and values DoesNotExist does not match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"quux": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchDoesNotExist},
			},
			inputKeys:   IK{"bar": {}, "baz": {}, "qux": {}},
			inputValues: IV{"foo": "1", "bar": "2", "quux": "3"},
			output:      O{},
			result:      assert.False,
		},
		{
			name: "multi: keys, values and instances all match",
			mes: &nfdv1alpha1.MatchExpressionSet{
				"foo": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"1"}},
				"bar": &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			},
			inputKeys:   IK{"bar": {}, "baz": {}, "qux": {}},
			inputValues: IV{"foo": "1", "bar": "2", "quux": "3"},
			inputInstances: II{
				{Attributes: A{"foo": "1", "bar": "2"}},
				{Attributes: A{"foo": "10", "bar": "20"}},
			},
			output: O{{"Name": "bar"}, {"Name": "bar", "Value": "2"}, {"Name": "foo", "Value": "1"}, {"bar": "2", "foo": "1"}},
			result: assert.True,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res, out, err := api.MatchMulti(tc.mes, tc.inputKeys, tc.inputValues, tc.inputInstances)
			if tc.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			tc.result(t, res)
			assert.Equal(t, tc.output, out)
		})
	}
}

func TestMatchNamesMulti(t *testing.T) {
	type O = []api.MatchedElement
	type IK = map[string]nfdv1alpha1.Nil
	type IV = map[string]string
	type II = []nfdv1alpha1.InstanceFeature
	type A = map[string]string

	type TC struct {
		name           string
		me             *nfdv1alpha1.MatchExpression
		inputKeys      IK
		inputValues    IV
		inputInstances II
		output         O
		result         bool
		expectErr      bool
	}

	tcs := []TC{
		{
			name:   "nil input",
			me:     &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			result: false,
			output: O{},
		},
		{
			name:      "empty input keys",
			me:        &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			inputKeys: IK{},
			result:    false,
			output:    O{},
		},
		{
			name:        "empty input values",
			me:          &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			inputValues: IV{},
			result:      false,
			output:      O{},
		},
		{
			name:           "empty input instances",
			me:             &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchAny},
			inputInstances: II{},
			result:         false,
			output:         O{},
		},
		{
			name:      "input keys match",
			me:        &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchExists},
			inputKeys: IK{"key1": {}, "key2": {}},
			result:    true,
			output:    O{{"Name": "key1"}, {"Name": "key2"}},
		},
		{
			name:      "input keys do not match",
			me:        &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchDoesNotExist},
			inputKeys: IK{"key1": {}, "key2": {}},
			result:    false,
			output:    O{},
		},
		{
			name:      "input keys error",
			me:        &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn},
			inputKeys: IK{"key1": {}, "key2": {}},
			result:    false,
			output:    nil,
			expectErr: true,
		},
		{
			name:        "input values match",
			me:          &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"key1"}},
			inputValues: IV{"key1": "val1", "key2": "val2"},
			result:      true,
			output:      O{{"Name": "key1", "Value": "val1"}},
		},
		{
			name:        "input values do not match",
			me:          &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"key3"}},
			inputValues: IV{"key1": "val1", "key2": "val2"},
			result:      false,
			output:      O{},
		},
		{
			name:        "input values error",
			me:          &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn},
			inputValues: IV{"key1": "val1", "key2": "val2"},
			result:      false,
			output:      nil,
			expectErr:   true,
		},
		{
			name: "input instances match",
			me: &nfdv1alpha1.MatchExpression{
				Op:    nfdv1alpha1.MatchIn,
				Value: nfdv1alpha1.MatchValue{"foo"},
			},
			inputInstances: II{
				{Attributes: A{"foo": "1"}},
				{Attributes: A{"bar": "2"}},
				{Attributes: A{"foo": "3", "baz": "4"}},
			},
			result: true,
			output: O{
				{"foo": "1"},
				{"foo": "3", "baz": "4"},
			},
		},
		{
			name: "input instances do not match",
			me: &nfdv1alpha1.MatchExpression{
				Op:    nfdv1alpha1.MatchIn,
				Value: nfdv1alpha1.MatchValue{"foo"},
			},
			inputInstances: II{
				{Attributes: A{"bar": "1"}},
				{Attributes: A{"baz": "2"}},
			},
			result: false,
			output: O{},
		},
		{
			name: "input instances error",
			me:   &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn},
			inputInstances: II{
				{Attributes: A{"bar": "1"}},
			},
			result:    false,
			output:    nil,
			expectErr: true,
		},
		{
			name:        "input keys, values and instances match",
			me:          &nfdv1alpha1.MatchExpression{Op: nfdv1alpha1.MatchIn, Value: nfdv1alpha1.MatchValue{"key2"}},
			inputKeys:   IK{"key1": {}, "key2": {}},
			inputValues: IV{"key1": "val1", "key2": "val2"},
			inputInstances: II{
				{Attributes: A{"key1": "1"}},
				{Attributes: A{"key1": "2"}},
				{Attributes: A{"key1": "3", "key2": "4"}},
			},
			result: true,
			output: O{{"Name": "key2"}, {"Name": "key2", "Value": "val2"}, {"key1": "3", "key2": "4"}},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res, ret, err := api.MatchNamesMulti(tc.me, tc.inputKeys, tc.inputValues, tc.inputInstances)
			if tc.expectErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, tc.result, res)
			assert.Equal(t, tc.output, ret)
		})
	}
}
