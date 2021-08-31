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

package v1alpha1

import (
	"regexp"
)

// MatchExpressionSet contains a set of MatchExpressions, each of which is
// evaluated against a set of input values.
type MatchExpressionSet map[string]*MatchExpression

// MatchExpression specifies an expression to evaluate against a set of input
// values. It contains an operator that is applied when matching the input and
// an array of values that the operator evaluates the input against.
// NB: CreateMatchExpression or MustCreateMatchExpression() should be used for
//     creating new instances.
// NB: Validate() must be called if Op or Value fields are modified or if a new
//     instance is created from scratch without using the helper functions.
type MatchExpression struct {
	// Op is the operator to be applied.
	Op MatchOp

	// Value is the list of values that the operand evaluates the input
	// against. Value should be empty if the operator is Exists, DoesNotExist,
	// IsTrue or IsFalse. Value should contain exactly one element if the
	// operator is Gt or Lt and exactly two elements if the operator is GtLt.
	// In other cases Value should contain at least one element.
	Value MatchValue `json:",omitempty"`

	// valueRe caches compiled regexps for "InRegexp" operator
	valueRe []*regexp.Regexp
}

// MatchOp is the match operator that is applied on values when evaluating a
// MatchExpression.
type MatchOp string

// MatchValue is the list of values associated with a MatchExpression.
type MatchValue []string

const (
	// MatchAny returns always true.
	MatchAny MatchOp = ""
	// MatchIn returns true if any of the values stored in the expression is
	// equal to the input.
	MatchIn MatchOp = "In"
	// MatchIn returns true if none of the values in the expression are equal
	// to the input.
	MatchNotIn MatchOp = "NotIn"
	// MatchInRegexp treats values of the expression as regular expressions and
	// returns true if any of them matches the input.
	MatchInRegexp MatchOp = "InRegexp"
	// MatchExists returns true if the input is valid. The expression must not
	// have any values.
	MatchExists MatchOp = "Exists"
	// MatchDoesNotExist returns true if the input is not valid. The expression
	// must not have any values.
	MatchDoesNotExist MatchOp = "DoesNotExist"
	// MatchGt returns true if the input is greater than the value of the
	// expression (number of values in the expression must be exactly one).
	// Both the input and value must be integer numbers, otherwise an error is
	// returned.
	MatchGt MatchOp = "Gt"
	// MatchLt returns true if the input is less  than the value of the
	// expression (number of values in the expression must be exactly one).
	// Both the input and value must be integer numbers, otherwise an error is
	// returned.
	MatchLt MatchOp = "Lt"
	// MatchGtLt returns true if the input is between two values, i.e. greater
	// than the first value and less than the second value of the expression
	// (number of values in the expression must be exactly two). Both the input
	// and values must be integer numbers, otherwise an error is returned.
	MatchGtLt MatchOp = "GtLt"
	// MatchIsTrue returns true if the input holds the value "true". The
	// expression must not have any values.
	MatchIsTrue MatchOp = "IsTrue"
	// MatchIsTrue returns true if the input holds the value "false". The
	// expression must not have any values.
	MatchIsFalse MatchOp = "IsFalse"
)
