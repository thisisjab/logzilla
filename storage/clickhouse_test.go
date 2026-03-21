package storage

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thisisjab/logzilla/querier/ast"
)

func TestClickhouseBuildWhereClause(t *testing.T) {
	timeA := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	timeB := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		input          ast.Query
		expectedClause string
		expectedArgs   []any
	}{
		{
			input: ast.Query{
				Root:  ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1}},
				Start: timeA,
			},
			expectedClause: "WHERE (level = ?) AND (timestamp >= ?)",
			expectedArgs:   []any{1, timeA},
		},
		{
			input: ast.Query{
				Root:  ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1}},
				Start: timeA,
				End:   timeB,
			},
			expectedClause: "WHERE (level = ?) AND (timestamp BETWEEN ? AND ?)",
			expectedArgs:   []any{1, timeA, timeB},
		},
		{
			input: ast.Query{
				Root:  ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1}},
				Start: timeB,
				End:   timeA,
			},
			expectedClause: "WHERE (level = ?) AND (timestamp BETWEEN ? AND ?)",
			expectedArgs:   []any{1, timeA, timeB},
		},
		{
			input: ast.Query{
				Root:   ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1}},
				Start:  timeA,
				End:    timeB,
				Cursor: "x",
			},
			expectedClause: "WHERE (level = ?) AND (timestamp BETWEEN ? AND ?) AND (id > ?)",
			expectedArgs:   []any{1, timeA, timeB, "x"},
		},
		{
			input: ast.Query{
				Root:   ast.ComparisonTerm{FieldName: "level", Operator: ast.OperatorEq, Values: []any{1}},
				Start:  timeB,
				End:    timeA,
				Cursor: "x",
			},
			expectedClause: "WHERE (level = ?) AND (timestamp BETWEEN ? AND ?) AND (id < ?)",
			expectedArgs:   []any{1, timeA, timeB, "x"},
		},
	}

	clh, err := NewClickHouseStorage(ClickHouseStorageConfig{})
	if err != nil {
		t.Fatalf("cannot run test due to clickhouse storage creation error: %s", err)
	}

	for i, tc := range tests {
		queryString, queryArgs, err := clh.buildWhereClause(tc.input)

		if err != nil {
			t.Fatalf("[%d] parseTerm failed with an error: %s", i, err)
		}

		if queryString != tc.expectedClause {
			t.Fatalf("[%d] expected `%s`, but got `%s`", i, tc.expectedClause, queryString)
		}

		if len(queryArgs) != len(tc.expectedArgs) {
			t.Fatalf("[%d] expected `%d` args, but got `%d`", i, len(tc.expectedArgs), len(queryArgs))
		}

		if !reflect.DeepEqual(queryArgs, tc.expectedArgs) {
			t.Fatalf("[%d] expected `%v` as args, but got `%v`", i, tc.expectedArgs, queryArgs)
		}
	}
}

func TestClickhouseParseRootTerm(t *testing.T) {
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
		queryString, queryArgs, err := clh.parseRootTerm(tc.input.Root)

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

func TestClickhouseBuildSortClause(t *testing.T) {
	timeA := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	timeB := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		input          ast.Query
		expectedClause string
		expectedArgs   []any
	}{
		{
			input: ast.Query{
				Start: timeA,
			},
			expectedClause: "ORDER BY timestamp ASC, id ASC",
			expectedArgs:   []any{},
		},
		{
			input: ast.Query{
				Start: timeA,
				End:   timeB,
			},
			expectedClause: "ORDER BY timestamp ASC, id ASC",
			expectedArgs:   []any{},
		},
		{
			input: ast.Query{
				Start: timeB,
				End:   timeA,
			},
			expectedClause: "ORDER BY timestamp DESC, id DESC",
			expectedArgs:   []any{},
		},
		{
			input: ast.Query{
				Sort:  []ast.SortField{{Name: "metadata.elapsed_time"}, {Name: "foo", IsDescending: true}},
				Start: timeB,
				End:   timeA,
			},
			expectedClause: "ORDER BY metadata.elapsed_time ASC, foo DESC, timestamp DESC, id DESC",
			expectedArgs:   []any{},
		},
	}

	clh, err := NewClickHouseStorage(ClickHouseStorageConfig{})
	if err != nil {
		t.Fatalf("cannot run test due to clickhouse storage creation error: %s", err)
	}

	for i, tc := range tests {
		queryString, queryArgs, err := clh.buildSortClause(tc.input)

		if err != nil {
			t.Fatalf("[%d] parseTerm failed with an error: %s", i, err)
		}

		if queryString != tc.expectedClause {
			t.Fatalf("[%d] expected `%s`, but got `%s`", i, tc.expectedClause, queryString)
		}

		if len(queryArgs) != len(tc.expectedArgs) {
			t.Fatalf("[%d] expected `%d` args, but got `%d`", i, len(tc.expectedArgs), len(queryArgs))
		}

		if !reflect.DeepEqual(queryArgs, tc.expectedArgs) {
			t.Fatalf("[%d] expected `%v` as args, but got `%v`", i, tc.expectedArgs, queryArgs)
		}
	}
}

func TestClickhouseBuildLimitClause(t *testing.T) {
	tests := []struct {
		input          ast.Query
		expectedClause string
		expectedArgs   []any
	}{
		{
			input: ast.Query{
				Limit: 10,
			},
			expectedClause: "LIMIT ?",
			expectedArgs:   []any{10},
		},
	}

	clh, err := NewClickHouseStorage(ClickHouseStorageConfig{})
	if err != nil {
		t.Fatalf("cannot run test due to clickhouse storage creation error: %s", err)
	}

	for i, tc := range tests {
		queryString, queryArgs, err := clh.buildLimitClause(tc.input)

		if err != nil {
			t.Fatalf("[%d] parseTerm failed with an error: %s", i, err)
		}

		if queryString != tc.expectedClause {
			t.Fatalf("[%d] expected `%s`, but got `%s`", i, tc.expectedClause, queryString)
		}

		if len(queryArgs) != len(tc.expectedArgs) {
			t.Fatalf("[%d] expected `%d` args, but got `%d`", i, len(tc.expectedArgs), len(queryArgs))
		}

		if !reflect.DeepEqual(queryArgs, tc.expectedArgs) {
			t.Fatalf("[%d] expected `%v` as args, but got `%v`", i, tc.expectedArgs, queryArgs)
		}
	}
}

func TestClickhouseConstructQuery(t *testing.T) {
	// prefix will be trimmed from all generated result, since it's the same among all queries
	const prefix = `SELECT id, source, level, message, timestamp, metadata FROM processed_logs `
	timeA := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	timeB := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		input          ast.Query
		expectedClause string
		expectedArgs   []any
	}{
		{
			input: ast.Query{
				Start: timeA,
				Limit: 10,
			},
			expectedClause: "WHERE (1 = 1) AND (timestamp >= ?) ORDER BY timestamp ASC, id ASC LIMIT ?",
			expectedArgs:   []any{timeA, 10},
		},
		{
			input: ast.Query{
				Start: timeA,
				End:   timeB,
				Limit: 10,
			},
			expectedClause: "WHERE (1 = 1) AND (timestamp BETWEEN ? AND ?) ORDER BY timestamp ASC, id ASC LIMIT ?",
			expectedArgs:   []any{timeA, timeB, 10},
		},
	}

	clh, err := NewClickHouseStorage(ClickHouseStorageConfig{})
	if err != nil {
		t.Fatalf("cannot run test due to clickhouse storage creation error: %s", err)
	}

	for i, tc := range tests {
		queryString, queryArgs, err := clh.constructQuery(tc.input)

		if err != nil {
			t.Fatalf("[%d] parseTerm failed with an error: %s", i, err)
		}

		if trimmed := strings.TrimPrefix(queryString, prefix); trimmed != tc.expectedClause {
			t.Fatalf("[%d] expected `%s`, but got `%s`", i, tc.expectedClause, trimmed)
		}

		if len(queryArgs) != len(tc.expectedArgs) {
			t.Fatalf("[%d] expected `%d` args, but got `%d`", i, len(tc.expectedArgs), len(queryArgs))
		}

		if !reflect.DeepEqual(queryArgs, tc.expectedArgs) {
			t.Fatalf("[%d] expected `%v` as args, but got `%v`", i, tc.expectedArgs, queryArgs)
		}
	}
}
