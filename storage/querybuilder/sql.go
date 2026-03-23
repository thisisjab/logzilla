package querybuilder

import (
	"fmt"
	"strings"

	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/querier/ast"
)

type SQLQueryBuilder struct {
}

func NewSQLQueryBuilder() *SQLQueryBuilder {
	return &SQLQueryBuilder{}
}

func (s *SQLQueryBuilder) BuildQuery(query ast.Query) (string, []any, error) {
	// To build query we need several steps:
	// 1) Build where clause considering cursor
	where, whereArgs, err := s.buildWhereClause(query)
	if err != nil {
		return "", nil, err
	}

	// 2) Provide sort
	sort, sortArgs, err := s.buildSortClause(query)
	if err != nil {
		return "", nil, err
	}

	// 3) Provide limit
	limit, limitArgs, err := s.buildLimitClause(query)
	if err != nil {
		return "", nil, err
	}

	q := fmt.Sprintf(`SELECT id, source, level, message, timestamp, metadata FROM processed_logs %s %s %s`, where, sort, limit)
	args := make([]any, 0)
	args = append(args, whereArgs...)
	args = append(args, sortArgs...)
	args = append(args, limitArgs...)

	return q, args, nil
}

func (s *SQLQueryBuilder) buildWhereClause(q ast.Query) (string, []any, error) {
	queryParts := make([]string, 2)
	queryArgs := make([]any, 0)

	rootClause, rootArgs, err := s.parseRootTerm(q.Root)
	if err != nil {
		return "", nil, fmt.Errorf("cannot build where clause do to error with root node: %w", err)
	}

	// root
	queryParts[0] = rootClause
	queryArgs = append(queryArgs, rootArgs...)

	// timestamp
	if q.End.IsZero() {
		queryParts[1] = `(timestamp >= ?)`
		queryArgs = append(queryArgs, q.Start)
	} else {
		queryParts[1] = `(timestamp BETWEEN ? AND ?)`

		if q.End.After(q.Start) {
			queryArgs = append(queryArgs, q.Start, q.End)
		} else {
			queryArgs = append(queryArgs, q.End, q.Start)
		}
	}

	// TODO: remove this comment or replace with my clear intention + a understandable language
	// earlier time -----e-------------------c--------s----- newer time
	// if start > end: descending
	// and if cursor is present as well, id < c
	// earlier time -----s------c---------------------e----- newer time
	// else: ascending
	// and if cursor is present: id > c

	// cursor
	if q.Cursor != "" {
		if q.Start.After(q.End) {
			queryParts = append(queryParts, `(id < ?)`)
		} else {
			queryParts = append(queryParts, `(id > ?)`)
		}

		queryArgs = append(queryArgs, q.Cursor)
	}

	return fmt.Sprintf("WHERE %s", strings.Join(queryParts, " AND ")), queryArgs, nil
}

func (s *SQLQueryBuilder) parseRootTerm(term ast.Term) (string, []any, error) {
	if term == nil {
		return "(1 = 1)", []any{}, nil
	}

	switch term := term.(type) {
	case ast.AndTerm:
		left, leftArgs, err := s.parseRootTerm(term.Left)
		if err != nil {
			return "", nil, fmt.Errorf("cannot parse `and` term due to errors with left: %w", err)
		}

		right, rightArgs, err := s.parseRootTerm(term.Right)
		if err != nil {
			return "", nil, fmt.Errorf("cannot parse `and` term due to errors with right: %w", err)
		}

		allArgs := leftArgs
		allArgs = append(allArgs, rightArgs...)

		return fmt.Sprintf("(%s AND %s)", left, right), allArgs, nil
	case ast.OrTerm:
		left, leftArgs, err := s.parseRootTerm(term.Left)
		if err != nil {
			return "", nil, fmt.Errorf("cannot parse `or` term due to errors with left: %w", err)
		}

		right, rightArgs, err := s.parseRootTerm(term.Right)
		if err != nil {
			return "", nil, fmt.Errorf("cannot parse `and` term due to errors with right: %w", err)
		}

		allArgs := leftArgs
		allArgs = append(allArgs, rightArgs...)

		return fmt.Sprintf("(%s OR %s)", left, right), allArgs, nil
	case ast.NotNode:
		innerTerm, innerArgs, err := s.parseRootTerm(term.Term)
		if err != nil {
			return "", nil, fmt.Errorf("cannot parse `not` term: %w", err)
		}

		return fmt.Sprintf("NOT %s", innerTerm), innerArgs, nil
	case ast.ComparisonTerm:
		var op string

		switch term.Operator {
		case ast.OperatorEq:
			if len(term.Values) > 1 {
				op = "IN"
			} else if len(term.Values) == 1 && term.Values[0] == nil {
				op = "IS"
			} else {
				op = "="
			}

		case ast.OperatorNe:
			if len(term.Values) > 1 {
				op = "NOT IN"
			} else if len(term.Values) == 1 && term.Values[0] == nil {
				op = "IS NOT"
			} else {
				op = "!="
			}

		case ast.OperatorILike:
			op = "ILIKE"

		case ast.OperatorLike:
			op = "LIKE"

		case ast.OperatorLt:
			op = "<"

		case ast.OperatorLte:
			op = "<="

		case ast.OperatorGt:
			op = ">"

		case ast.OperatorGte:
			op = ">="

		default:
			return "", nil, fmt.Errorf("comparison operator `%d` is unknown", term.Operator)
		}

		// For clickhouse, log levels must be expressed using numbers (8 bit integers)
		// Let's convert them here, but we should remove this as soon as we are using this in another SQL database.
		if term.FieldName == "level" {
			normalizeLevel(term.Values)
		}

		if len(term.Values) > 1 {
			return fmt.Sprintf("(%s %s ?)", term.FieldName, op), []any{term.Values}, nil
		}

		return fmt.Sprintf("(%s %s ?)", term.FieldName, op), term.Values, nil

	default:
		return "", nil, fmt.Errorf("unknown term")
	}
}

func (s *SQLQueryBuilder) buildSortClause(q ast.Query) (string, []any, error) {
	sortFields := make([]string, len(q.Sort))
	sortArgs := make([]any, 0)

	for i := range q.Sort {
		dir := "DESC"
		if !q.Sort[i].IsDescending {
			dir = "ASC"
		}

		sortFields[i] = fmt.Sprintf("%s %s", q.Sort[i].Name, dir)
	}

	if !q.End.IsZero() && q.Start.After(q.End) {
		sortFields = append(sortFields, "timestamp DESC, id DESC")
	} else {
		sortFields = append(sortFields, "timestamp ASC, id ASC")
	}

	return fmt.Sprintf("ORDER BY %s", strings.Join(sortFields, ", ")), sortArgs, nil
}

func (s *SQLQueryBuilder) buildLimitClause(q ast.Query) (string, []any, error) {
	if q.Limit == 0 {
		q.Limit = 100
	}

	if !(q.Limit >= 1 && q.Limit <= 1000) {
		return "", nil, fmt.Errorf("limit value is not in range [1, 1000]")
	}

	return "LIMIT ?", []any{q.Limit}, nil
}

func normalizeLevel(values []any) {
	for i, v := range values {
		s := strings.ToLower(fmt.Sprint(v))

		switch s {
		case "1", "debug":
			values[i] = int(entity.LogLevelDebug)
		case "2", "info":
			values[i] = int(entity.LogLevelInfo)
		case "3", "warn", "warning":
			values[i] = int(entity.LogLevelWarn)
		case "4", "error":
			values[i] = int(entity.LogLevelError)
		case "5", "fatal":
			values[i] = int(entity.LogLevelFatal)
		default:
			values[i] = int(entity.LogLevelUnknown)
		}
	}
}
