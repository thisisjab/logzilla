package storage

import (
	"reflect"
	"testing"

	"github.com/thisisjab/logzilla/querier/ast"
)

func TestClickhouseBuildWhereClause(t *testing.T) {
	tests := []struct {
		input         ast.Query
		expectedQuery string
		expectedArgs  []any
	}{
		{
			input: ast.Query{
				Root: ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1}},
			},
			expectedQuery: "(level = ?)",
			expectedArgs:  []any{1},
		},
		{
			input: ast.Query{
				Root: ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1, 2}},
			},
			expectedQuery: "(level IN ?)",
			expectedArgs:  []any{[]any{1, 2}},
		},
		{
			input: ast.Query{
				Root: ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorNe, Values: []any{1}},
			},
			expectedQuery: "(level != ?)",
			expectedArgs:  []any{1},
		},
		{
			input: ast.Query{
				Root: ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorNe, Values: []any{1, 2}},
			},
			expectedQuery: "(level NOT IN ?)",
			expectedArgs:  []any{[]any{1, 2}},
		},
		{
			input: ast.Query{
				Root: ast.AndTerm{
					Left:  ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1}},
					Right: ast.ComparisonTerm{FieldName: "message", Operator: ast.OperatorEq, Values: []any{3}},
				},
			},
			expectedQuery: "((level = ?) AND (message = ?))",
			expectedArgs:  []any{1, 3},
		},
		{
			input: ast.Query{
				Root: ast.OrTerm{
					Left:  ast.ComparisonTerm{FieldName: "kiwi", Operator: ast.OperatorEq, Values: []any{1}},
					Right: ast.ComparisonTerm{FieldName: "message", Operator: ast.OperatorEq, Values: []any{2}},
				},
			},
			expectedQuery: "((kiwi = ?) OR (message = ?))",
			expectedArgs:  []any{1, 2},
		},
		{
			input: ast.Query{
				Root: ast.NotNode{
					Term: ast.ComparisonTerm{FieldName: "message", Operator: ast.OperatorLike, Values: []any{"foo"}},
				},
			},
			expectedQuery: "NOT (message LIKE ?)",
			expectedArgs:  []any{"foo"},
		},
		{
			input: ast.Query{
				Root: ast.NotNode{
					Term: ast.OrTerm{
						Left:  ast.NotNode{Term: ast.ComparisonTerm{FieldName: "message", Operator: ast.OperatorLike, Values: []any{"foo"}}},
						Right: ast.ComparisonTerm{FieldName: "other-field", Operator: ast.OperatorNe, Values: []any{"bar"}},
					},
				},
			},
			expectedQuery: "NOT (NOT (message LIKE ?) OR (other-field != ?))",
			expectedArgs:  []any{"foo", "bar"},
		},
	}

	clh, err := NewClickHouseStorage(ClickHouseStorageConfig{})
	if err != nil {
		t.Fatalf("cannot run test due to clickhouse storage creation error: %s", err)
	}

	for i, tc := range tests {
		queryString, queryArgs, err := clh.parseTerm(tc.input.Root)

		if err != nil {
			t.Fatalf("[%d] parseTerm failed with an error: %s", i, err)
		}

		if queryString != tc.expectedQuery {
			t.Fatalf("[%d] expected `%s`, but got `%s`", i, tc.expectedQuery, queryString)
		}

		if len(queryArgs) != len(tc.expectedArgs) {
			t.Fatalf("[%d] expected `%d` args, but got `%d`", i, len(tc.expectedArgs), len(queryArgs))
		}

		if !reflect.DeepEqual(queryArgs, tc.expectedArgs) {
			t.Fatalf("[%d] expected `%v` as args, but got `%v`", i, tc.expectedArgs, queryArgs)
		}
	}
}
