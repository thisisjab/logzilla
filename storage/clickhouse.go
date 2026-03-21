package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/querier"
	"github.com/thisisjab/logzilla/querier/ast"
)

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
	return &ClickHouseStorage{
		cfg: cfg,
	}, nil
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

func (s *ClickHouseStorage) Open(ctx context.Context) error {
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
		m, err := json.Marshal(log.Metadata)
		if err != nil {
			return fmt.Errorf("cannot marshal log metadata: %w", err)
		}

		err = batch.Append(log.ID, log.Source, log.Timestamp, log.Level, log.Message, m)

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

func (s *ClickHouseStorage) Query(ctx context.Context, req querier.QueryRequest) (*querier.QueryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	q, args, err := s.constructQuery(req.Query)
	if err != nil {
		return nil, fmt.Errorf("cannot construct query: %w", err)
	}

	rows, err := s.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("cannot query database: %w", err)
	}
	defer rows.Close()

	logs, err := s.scanRows(rows)
	if err != nil {
		return nil, fmt.Errorf("cannot scan rows: %w", err)
	}

	cursor := ""
	if len(logs) > 0 {
		cursor = logs[len(logs)-1].ID.String()
	}

	return &querier.QueryResponse{Records: logs, Cursor: cursor}, nil
}

func (s *ClickHouseStorage) constructQuery(query ast.Query) (string, []any, error) {
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

func (s *ClickHouseStorage) buildWhereClause(q ast.Query) (string, []any, error) {
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

func (s *ClickHouseStorage) parseRootTerm(term ast.Term) (string, []any, error) {
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
		// TODO: check if field name is allowed

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

		if len(term.Values) > 1 {
			return fmt.Sprintf("(%s %s ?)", term.FieldName, op), []any{term.Values}, nil
		}

		return fmt.Sprintf("(%s %s ?)", term.FieldName, op), term.Values, nil

	default:
		return "", nil, fmt.Errorf("unknown term")
	}
}

func (s *ClickHouseStorage) buildSortClause(q ast.Query) (string, []any, error) {
	sortFields := make([]string, len(q.Sort))
	sortArgs := make([]any, 0)

	for i := range q.Sort {
		// TODO: validate sort field names to prevent possible SQL injection attacks

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

func (s *ClickHouseStorage) buildLimitClause(q ast.Query) (string, []any, error) {
	if q.Limit == 0 {
		q.Limit = 100
	}

	if !(q.Limit >= 1 && q.Limit <= 1000) {
		return "", nil, fmt.Errorf("limit value is not in range [1, 1000]")
	}

	return "LIMIT ?", []any{q.Limit}, nil
}

func (s *ClickHouseStorage) scanRows(rows driver.Rows) ([]entity.LogRecord, error) {
	logs := make([]entity.LogRecord, 0)

	for rows.Next() {
		var r entity.LogRecord
		var logLevel string

		err := rows.Scan(&r.ID, &r.Source, &logLevel, &r.Message, &r.Timestamp, &r.Metadata)
		if err != nil {
			return nil, fmt.Errorf("rows scan error: %w", err)
		}

		r.Level = parseLogLevel(logLevel)

		logs = append(logs, r)
	}

	return logs, nil
}
