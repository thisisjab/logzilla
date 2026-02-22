package storage

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/querier"
)

var allowedFieldsRegex = regexp.MustCompile(`^(id|level|timestamp|message|source|metadata(\.("[^"]+"|[a-zA-Z0-9_]+))?)$`)

type ClickHouseStorageConfig struct {
	Addr     []string `yaml:"addr"`
	Database string   `yaml:"database"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

// TODO: add support for printing generated/executed queries (both for insert and select)
type ClickHouseStorage struct {
	conn clickhouse.Conn
	cfg  ClickHouseStorageConfig
}

func NewClickHouseStorage(cfg ClickHouseStorageConfig) (*ClickHouseStorage, error) {
	return &ClickHouseStorage{cfg: cfg}, nil
}

func setupClickHouseTables(ctx context.Context, conn driver.Conn) error {
	// Table 1: Raw Logs
	// Use String for raw_data to hold bytes; ClickHouse handles bytes as String.
	err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS raw_logs (
			id UUID,
			source String,
			timestamp DateTime64(3),
			level Enum8('UNKNOWN' = 0, 'DEBUG' = 1, 'INFO' = 2, 'WARN' = 3, 'ERROR' = 4, 'FATAL' = 5),
			raw_data String -- Binary-safe field
		)
		ENGINE = MergeTree
		ORDER BY (source, timestamp, id)
		PARTITION BY toYYYYMM(timestamp)
	`)
	if err != nil {
		return err
	}

	// Table 2: Processed Logs
	// We use the JSON type for the flexible metadata
	err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS processed_logs (
			id UUID,
			source String,
			timestamp DateTime64(3),
			level Enum8('UNKNOWN' = 0, 'DEBUG' = 1, 'INFO' = 2, 'WARN' = 3, 'ERROR' = 4, 'FATAL' = 5),
			message String,
			metadata JSON
		)
		ENGINE = MergeTree
		ORDER BY (source, timestamp, level)
		PARTITION BY toYYYYMM(timestamp)
	`)
	return err
}

func (s *ClickHouseStorage) Connect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: s.cfg.Addr,
		Auth: clickhouse.Auth{
			Database: s.cfg.Database,
			Username: s.cfg.Username,
			Password: s.cfg.Password,
		},
		Settings: clickhouse.Settings{
			"allow_experimental_json_type": 1, // This is for supporting JSON columns
		},
		DialTimeout: 5 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping the database: %w", err)
	}

	s.conn = conn

	// Since we only have two tables, for now we don't need to introduce go-migrate
	if err := setupClickHouseTables(ctx, conn); err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	return nil
}

func (s *ClickHouseStorage) Close(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return s.conn.Close()
}

func (s *ClickHouseStorage) StoreRawLogs(ctx context.Context, logs ...entity.LogRecord) error {
	if len(logs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO raw_logs (id, source, timestamp, level, raw_data)")
	if err != nil {
		return fmt.Errorf("couldn't prepare batch: %w", err)
	}

	for _, log := range logs {
		err = batch.Append(uuid.New(), log.Source, log.Timestamp, log.Level, log.RawData)

		if err != nil {
			return fmt.Errorf("couldn't append log to batch: %w", err)
		}
	}

	err = batch.Send()
	if err != nil {
		return fmt.Errorf("couldn't send batch: %w", err)
	}

	return nil
}

func (s *ClickHouseStorage) StoreProcessedLogs(ctx context.Context, logs ...entity.LogRecord) error {
	if len(logs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO processed_logs (id, source, timestamp, level, message, metadata)")
	if err != nil {
		return fmt.Errorf("couldn't prepare batch: %w", err)
	}

	for _, log := range logs {
		err = batch.Append(log.ID, log.Source, log.Timestamp, log.Level, log.Message, log.Metadata)

		if err != nil {
			return fmt.Errorf("couldn't append log to batch: %w", err)
		}
	}

	err = batch.Send()
	if err != nil {
		return fmt.Errorf("couldn't send batch: %w", err)
	}

	return nil
}

func (s *ClickHouseStorage) Query(ctx context.Context, req querier.QueryRequest) (querier.QueryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Build WHERE clause, ORDER BY, and LIMIT clauses from expression tree
	queryClause, args, err := s.buildQuery(req.Query)
	if err != nil {
		return querier.QueryResponse{}, fmt.Errorf("failed to build where clause: %w", err)
	}

	// Execute the query
	rows, err := s.conn.Query(ctx, queryClause, args...)
	if err != nil {
		return querier.QueryResponse{}, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Scan results
	records, err := scanLogRecords(rows)
	if err != nil {
		return querier.QueryResponse{}, fmt.Errorf("failed to scan results: %w", err)
	}

	return querier.QueryResponse{
		Records: records,
		Cursor:  "", // TODO: Implement cursor-based pagination
	}, nil
}

func (s *ClickHouseStorage) buildQuery(q querier.Query) (string, []any, error) {
	// build WHERE clause from expression tree
	whereClause, args, err := s.buildWhereClause(q.Node, q.Start, q.End, uuid.UUID{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to build where clause: %w", err)
	}

	// Build ORDER BY clause
	orderByClause, err := s.buildOrderByClause(q.Start, q.End, q.Sort)
	if err != nil {
		return "", nil, fmt.Errorf("cannot build query (order clause): %w", err)
	}

	// Build LIMIT clause
	limitClause := fmt.Sprintf("LIMIT %d", q.Limit)

	// Construct the full SQL query
	sqlQuery := fmt.Sprintf(`
			SELECT id, source, timestamp, level, message, metadata
			FROM processed_logs
			WHERE %s
			%s
			%s
		`, whereClause, orderByClause, limitClause)

	return sqlQuery, args, nil
}

// buildOrderByClause determines the sort order based on custom fields
// and the relationship between Start and End timestamps.
func (s *ClickHouseStorage) buildOrderByClause(start, end time.Time, sortFields []querier.SortField) (string, error) {
	// Determine the chronological direction based on the comment:
	// "If End is before Start, the query is executed in backward chronological order."
	timeDirection := "ASC"
	if !end.IsZero() && end.Before(start) {
		timeDirection = "DESC"
	}

	// Define allowed fields for security/validation
	allowedFields := []string{"source", "level", "timestamp"}

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
	hasTimestamp := slices.ContainsFunc(sortFields, func(f querier.SortField) bool {
		return f.Name == "timestamp"
	})

	if !hasTimestamp {
		parts = append(parts, fmt.Sprintf("timestamp %s", timeDirection))
	}

	return fmt.Sprintf("ORDER BY %s", strings.Join(parts, ", ")), nil
}

func (s *ClickHouseStorage) buildWhereClause(root querier.QueryNode, start, end time.Time, skipID uuid.UUID) (string, []any, error) {
	q, args, err := s.parseQueryNode(root)
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
	if q != "" {
		parts = append(parts, q)
		finalArgs = append(finalArgs, args...)
	}

	return strings.Join(parts, " AND "), finalArgs, nil
}

func (s *ClickHouseStorage) parseQueryNode(node querier.QueryNode) (string, []any, error) {
	if node == nil {
		return "", nil, nil
	}

	// args is used to collect the arguments for the query parameters
	var args []any

	switch n := node.(type) {
	case querier.AndNode:
		// Join all children with AND. If there are no children,
		// we return an empty string or a truthy expression like (1=1).
		return s.joinNodes(n.Children, "AND", args)

	case querier.OrNode:
		// Join all children with OR.
		return s.joinNodes(n.Children, "OR", args)

	case querier.NotNode:
		// Recurse into the single child and wrap with NOT.
		childQuery, args, err := s.parseQueryNode(n.Child)

		if err != nil {
			return "", nil, err
		}

		if childQuery == "" {
			return "", nil, nil
		}

		return fmt.Sprintf("NOT (%s)", childQuery), args, nil

	case querier.ComparisonNode:
		// This is a leaf node. We stop recursing here and
		// convert the specific comparison into ClickHouse SQL.
		return s.formatComparison(n)

	default:
		return "", nil, fmt.Errorf("unknown query node type: %T", node)
	}
}

// joinNodes is a helper to handle the recursion for logical groups.
func (s *ClickHouseStorage) joinNodes(children []querier.QueryNode, operator string, args []any) (string, []any, error) {
	if len(children) == 0 {
		return "", nil, nil
	}

	var parts []string
	for _, child := range children {
		query, qArgs, err := s.parseQueryNode(child) // Recursive call
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
	// when ClickHouse evaluates the full string.
	return fmt.Sprintf("(%s)", strings.Join(parts, fmt.Sprintf(" %s ", operator))), args, nil
}

// formatComparison is a helper to handle the recursion for logical groups.
func (s *ClickHouseStorage) formatComparison(n querier.ComparisonNode) (string, []any, error) {
	if n.FieldName == "" || n.Value == nil {
		return "", nil, fmt.Errorf("invalid comparison node: missing field name or value")
	}

	// Prevent SQL injection
	if !allowedFieldsRegex.MatchString(n.FieldName) {
		return "", nil, fmt.Errorf("invalid field name: %s", n.FieldName)
	}

	args := make([]any, 1)
	args[0] = n.Value

	op := ""
	switch n.Operator {
	case querier.OperatorEq:
		op = "="
	case querier.OperatorNe:
		op = "!="
	case querier.OperatorGt:
		op = ">"
	case querier.OperatorLt:
		op = "<"
	case querier.OperatorGte:
		op = ">="
	case querier.OperatorLte:
		op = "<="
	case querier.OperatorLike:
		op = "LIKE"
	case querier.OperatorILike:
		op = "ILIKE"
	case querier.OperatorIn:
		op = "IN"
	default:
		return "", nil, fmt.Errorf("unsupported operator: %v", n.Operator)
	}

	return fmt.Sprintf("%s %s ?", n.FieldName, op), args, nil
}

func scanLogRecords(rows driver.Rows) ([]entity.LogRecord, error) {
	var records []entity.LogRecord

	for rows.Next() {
		var record entity.LogRecord
		var levelStr string

		err := rows.Scan(
			&record.ID,
			&record.Source,
			&record.Timestamp,
			&levelStr,
			&record.Message,
			&record.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		record.Level = parseLogLevel(levelStr)
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return records, nil
}

func parseLogLevel(level string) entity.LogLevel {
	switch level {
	case "DEBUG":
		return entity.LogLevelDebug
	case "INFO":
		return entity.LogLevelInfo
	case "WARN":
		return entity.LogLevelWarn
	case "ERROR":
		return entity.LogLevelError
	case "FATAL":
		return entity.LogLevelFatal
	default:
		return entity.LogLevelUnknown
	}
}
