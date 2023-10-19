// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import "testing"

func TestValidateAssertion(t *testing.T) {
	tests := []struct {
		actual    uint64
		assertion Assertion
		expected  bool
	}{
		{5, Assertion{Operator: string(GreaterThan), Operand: 3}, true},
		{5, Assertion{Operator: string(LessThan), Operand: 10}, true},
		{5, Assertion{Operator: string(EqualTo), Operand: 5}, true},
		{5, Assertion{Operator: string(NotEqualTo), Operand: 3}, true},
		{5, Assertion{Operator: string(GreaterThan), Operand: 10}, false},
		{5, Assertion{Operator: string(LessThan), Operand: 2}, false},
	}

	for _, tt := range tests {
		result := validateAssertion(tt.actual, &tt.assertion)
		if result != tt.expected {
			t.Errorf("validateAssertion(%d, %+v) = %v; want %v", tt.actual, tt.assertion, result, tt.expected)
		}
	}
}
