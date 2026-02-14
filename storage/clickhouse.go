package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/entity"
)

type ClickhouseStorage struct {
	conn clickhouse.Conn
}

type ClickhouseStorageConfig struct {
	Addr     []string `yaml:"addr"`
	Database string   `yaml:"database"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

func NewClickhouseStorage(cfg ClickhouseStorageConfig) (*ClickhouseStorage, error) {
	// FIXME: implment a wait-for-ready procedure

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addr,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
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
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	// Since we only have two tables, for now we don't need to introduce go-migrate
	if err := setupClickhouseTables(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to create table: %v", err)
	}

	return &ClickhouseStorage{
		conn: conn,
	}, nil
}

func setupClickhouseTables(ctx context.Context, conn driver.Conn) error {
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

func (s *ClickhouseStorage) Close() error {
	return s.conn.Close()
}

func (s *ClickhouseStorage) StoreRawLogs(ctx context.Context, logs ...entity.LogRecord) error {
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

func (s *ClickhouseStorage) StoreProcessedLogs(ctx context.Context, logs ...entity.LogRecord) error {
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
