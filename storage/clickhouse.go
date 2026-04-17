package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/querier"
	"github.com/thisisjab/logzilla/storage/querybuilder"
)

type ClickHouseStorageConfig struct {
	Addr     []string `yaml:"addr"`
	Database string   `yaml:"database"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

// TODO: add support for printing generated/executed queries (both for insert and select)
type ClickHouseStorage struct {
	queryBuilder *querybuilder.SQLQueryBuilder
	conn         clickhouse.Conn
	cfg          ClickHouseStorageConfig
}

func NewClickHouseStorage(cfg ClickHouseStorageConfig) (*ClickHouseStorage, error) {
	return &ClickHouseStorage{
		queryBuilder: querybuilder.NewSQLQueryBuilder(),
		cfg:          cfg,
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
		err = batch.Append(uuid.New(), log.Source, log.Timestamp, int(log.Level), log.RawData)

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

		err = batch.Append(log.ID, log.Source, log.Timestamp, int(log.Level), log.Message, m)

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

	q, args, err := s.queryBuilder.BuildQuery(req.Query)
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
