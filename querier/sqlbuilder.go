package querier

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/querier/ast"
)

// SQLOptions holds configuration for the SQL query builder.
type SQLOptions struct {
	// AllowedSortFields is a whitelist of field names permitted in ORDER BY clauses.
	// This prevents SQL injection through malicious sort parameters.
	// If empty, defaults to ["source", "level", "timestamp"].
	AllowedSortFields []string

	// AllowedFilterFieldsRegex is a regex pattern to validate field names in WHERE clauses.
	// This provides fine-grained control over which fields can be filtered,
	// including support for nested JSON paths (e.g., metadata.user_id).
	// If nil, no regex validation is performed on filter fields.
	AllowedFilterFieldsRegex *regexp.Regexp

	// TableName is the name of the table to query from.
	TableName string

	// SelectColumns is the list of columns to SELECT.
	// If empty, defaults to SELECT *.
	SelectColumns []string
}

// SQLQueryBuilder is a generic SQL query builder that constructs
// SELECT queries with WHERE, ORDER BY, and LIMIT clauses.
type SQLQueryBuilder struct {
	opts SQLOptions
}

// NewSQLQueryBuilder creates a new SQL query builder with the given options.
func NewSQLQueryBuilder(opts SQLOptions) *SQLQueryBuilder {
	return &SQLQueryBuilder{opts: opts}
}

// BuildResult holds the generated SQL query and its arguments.
type BuildResult struct {
	Query string
	Args  []any
}

// Build builds a complete SELECT query from the given Query parameters.
func (b *SQLQueryBuilder) Build(q ast.Query) (BuildResult, error) {
	whereClause, args, err := b.buildWhereClause(q.Node, q.Start, q.End, uuid.UUID{})
	if err != nil {
		return BuildResult{}, fmt.Errorf("failed to build where clause: %w", err)
	}

	orderByClause, err := b.buildOrderByClause(q.Start, q.End, q.Sort)
	if err != nil {
		return BuildResult{}, fmt.Errorf("failed to build order by clause: %w", err)
	}

	limitClause := fmt.Sprintf("LIMIT %d", q.Limit)

	selectCols := strings.Join(b.opts.SelectColumns, ", ")
	if len(b.opts.SelectColumns) == 0 {
		selectCols = "*"
	}

	sqlQuery := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s %s %s",
		selectCols,
		b.opts.TableName,
		whereClause,
		orderByClause,
		limitClause,
	)

	return BuildResult{Query: sqlQuery, Args: args}, nil
}

// buildWhereClause constructs the WHERE clause with timestamp bounds and query conditions.
func (b *SQLQueryBuilder) buildWhereClause(root ast.QueryNode, start, end time.Time, skipID uuid.UUID) (string, []any, error) {
	queryClause, args, err := b.parseQueryNode(root)
	if err != nil {
		return "", nil, err
	}

	var sTime, eTime time.Time

	if start.Compare(end) < 0 {
		sTime = start
		eTime = end
	} else {
		sTime = end
		eTime = start
	}

	// Always add timestamp bounds
	parts := []string{"timestamp >= ?"}
	finalArgs := []any{sTime}

	if !eTime.IsZero() {
		parts = append(parts, "timestamp <= ?")
		finalArgs = append(finalArgs, eTime)
	}

	// Add query conditions if they exist
	if queryClause != "" {
		parts = append(parts, queryClause)
		finalArgs = append(finalArgs, args...)
	}

	return strings.Join(parts, " AND "), finalArgs, nil
}

// buildOrderByClause determines the sort order based on custom fields
// and the relationship between Start and End timestamps.
func (b *SQLQueryBuilder) buildOrderByClause(start, end time.Time, sortFields []ast.SortField) (string, error) {
	// Determine the chronological direction based on the comment:
	// "If End is before Start, the query is executed in backward chronological order."
	timeDirection := "ASC"
	if !end.IsZero() && end.Before(start) {
		timeDirection = "DESC"
	}

	// Define allowed fields for security/validation
	allowedFields := b.opts.AllowedSortFields
	if len(allowedFields) == 0 {
		allowedFields = []string{"source", "level", "timestamp"}
	}

	// Handle the case where no specific sort fields are requested
	if len(sortFields) == 0 {
		return fmt.Sprintf("ORDER BY timestamp %s", timeDirection), nil
	}

	// Validate and build custom sort parts
	var parts []string
	for _, field := range sortFields {
		if !slices.Contains(allowedFields, field.Name) {
			return "", fmt.Errorf("field `%s` is not allowed for sorting", field.Name)
		}

		direction := "ASC"
		if field.IsDescending {
			direction = "DESC"
		}

		parts = append(parts, fmt.Sprintf("%s %s", field.Name, direction))
	}

	// Ensure timestamp is included in the sort to respect the Start/End logic
	// if it wasn't already explicitly provided in sortFields.
	hasTimestamp := slices.ContainsFunc(sortFields, func(f ast.SortField) bool {
		return f.Name == "timestamp"
	})

	if !hasTimestamp {
		parts = append(parts, fmt.Sprintf("timestamp %s", timeDirection))
	}

	return fmt.Sprintf("ORDER BY %s", strings.Join(parts, ", ")), nil
}

// parseQueryNode recursively traverses the query tree and generates SQL.
func (b *SQLQueryBuilder) parseQueryNode(node ast.QueryNode) (string, []any, error) {
	if node == nil {
		return "", nil, nil
	}

	// args is used to collect the arguments for the query parameters
	var args []any

	switch n := node.(type) {
	case ast.AndNode:
		// Join all children with AND. If there are no children,
		// we return an empty string or a truthy expression like (1=1).
		return b.joinNodes(n.Children, "AND", args)

	case ast.OrNode:
		// Join all children with OR.
		return b.joinNodes(n.Children, "OR", args)

	case ast.NotNode:
		// Recurse into the single child and wrap with NOT.
		childQuery, args, err := b.parseQueryNode(n.Child)

		if err != nil {
			return "", nil, err
		}

		if childQuery == "" {
			return "", nil, nil
		}

		return fmt.Sprintf("NOT (%s)", childQuery), args, nil

	case ast.ComparisonNode:
		// This is a leaf node. We stop recursing here and
		// convert the specific comparison into SQL.
		return b.formatComparison(n)

	default:
		return "", nil, fmt.Errorf("unknown query node type: %T", node)
	}
}

// joinNodes is a helper to handle the recursion for logical groups.
func (b *SQLQueryBuilder) joinNodes(children []ast.QueryNode, operator string, args []any) (string, []any, error) {
	if len(children) == 0 {
		return "", nil, nil
	}

	var parts []string
	for _, child := range children {
		query, qArgs, err := b.parseQueryNode(child) // Recursive call
		if err != nil {
			return "", nil, err
		}
		if query != "" {
			parts = append(parts, query)
			args = append(args, qArgs...)
		}
	}

	if len(parts) == 0 {
		return "", nil, nil
	}

	// Wrap in parentheses to ensure correct mathematical precedence
	// when the database evaluates the full string.
	return fmt.Sprintf("(%s)", strings.Join(parts, fmt.Sprintf(" %s ", operator))), args, nil
}

// formatComparison converts a ComparisonNode into SQL.
func (b *SQLQueryBuilder) formatComparison(n ast.ComparisonNode) (string, []any, error) {
	if n.FieldName == "" || n.Value == nil {
		return "", nil, fmt.Errorf("invalid comparison node: missing field name or value")
	}

	// Prevent SQL injection by validating field name against allowed pattern
	if b.opts.AllowedFilterFieldsRegex != nil && !b.opts.AllowedFilterFieldsRegex.MatchString(n.FieldName) {
		return "", nil, fmt.Errorf("invalid field name: %s", n.FieldName)
	}

	args := make([]any, 1)
	args[0] = n.Value

	op := ""
	switch n.Operator {
	case ast.OperatorEq:
		op = "="
	case ast.OperatorNe:
		op = "!="
	case ast.OperatorGt:
		op = ">"
	case ast.OperatorLt:
		op = "<"
	case ast.OperatorGte:
		op = ">="
	case ast.OperatorLte:
		op = "<="
	case ast.OperatorLike:
		op = "LIKE"
	case ast.OperatorILike:
		op = "ILIKE"
	case ast.OperatorIn:
		op = "IN"
	default:
		return "", nil, fmt.Errorf("unsupported operator: %v", n.Operator)
	}

	return fmt.Sprintf("%s %s ?", n.FieldName, op), args, nil
}
