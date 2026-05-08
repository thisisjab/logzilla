package ast

import "testing"

func TestTermString(t *testing.T) {
	tests := []struct {
		term     Term
		expected string
	}{
		{
			term:     ComparisonTerm{FieldName: "x", Operator: OperatorEq, Values: []any{"a", "b", "c"}},
			expected: "(x = \"a\", \"b\", \"c\")",
		},
		{
			term:     ComparisonTerm{FieldName: "x", Operator: OperatorNe, Values: []any{1, 2, 3}},
			expected: "(x != 1, 2, 3)",
		},
		{
			term:     ComparisonTerm{FieldName: "x", Operator: OperatorILike, Values: []any{"test-test"}},
			expected: "(x ~ \"test-test\")",
		},
		{
			term:     AndTerm{Left: ComparisonTerm{FieldName: "x", Operator: OperatorEq, Values: []any{"a", "b", "c"}}, Right: ComparisonTerm{FieldName: "x", Operator: OperatorILike, Values: []any{"test-test"}}},
			expected: "((x = \"a\", \"b\", \"c\") & (x ~ \"test-test\"))",
		},
		{
			term:     OrTerm{Left: ComparisonTerm{FieldName: "x", Operator: OperatorEq, Values: []any{"a", "b", "c"}}, Right: ComparisonTerm{FieldName: "x", Operator: OperatorILike, Values: []any{"test-test"}}},
			expected: "((x = \"a\", \"b\", \"c\") | (x ~ \"test-test\"))",
		},
	}

	for i, tc := range tests {
		res := tc.term.String()
		if res != tc.expected {
			t.Fatalf("[%d] expected `%s`, but got `%s`", i, tc.expected, res)
		}
	}
}
