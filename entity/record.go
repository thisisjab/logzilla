package entity

import (
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
)

// LogRecord represents a unified, processed log record ready for in-memory indexing (MemTable)
// and on-disk persistence (SSTables).
type LogRecord struct {
	ID            ulid.ULID
	NodeID        uuid.UUID
	NodeName      string
	CollectorName string
	Timestamp     int64 // Unix timestamp in milliseconds
	Level         LogLevel
	Message       string
	Metadata      map[string]any
	Raw           string
	err           error
}

// DateTime returns the UTC time.Time representation of the log record's millisecond timestamp.
func (r LogRecord) DateTime() time.Time {
	return time.UnixMilli(r.Timestamp).UTC()
}

// Err returns the processing error encountered for this record, or nil if processing succeeded.
func (r LogRecord) Err() error {
	return r.err
}

// HasError reports whether the log record encountered a processing error.
func (r LogRecord) HasError() bool {
	return r.err != nil
}

// WithError returns a copy of the LogRecord with the given processing error attached.
func (r LogRecord) WithError(err error) LogRecord {
	r.err = err
	return r
}

// New creates a new LogRecord with normalized timestamp and metadata.
// If timestamp <= 0, the current UTC time is used.
// If metadata is nil, an empty map is initialized.
func New(
	id ulid.ULID,
	nodeID uuid.UUID,
	nodeName string,
	collectorName string,
	timestamp int64,
	level LogLevel,
	message string,
	metadata map[string]any,
	raw string,
) LogRecord {
	if timestamp <= 0 {
		timestamp = time.Now().UTC().UnixMilli()
	}

	if metadata == nil {
		metadata = make(map[string]any)
	}

	return LogRecord{
		ID:            id,
		NodeID:        nodeID,
		NodeName:      nodeName,
		CollectorName: collectorName,
		Timestamp:     timestamp,
		Level:         level,
		Message:       message,
		Metadata:      metadata,
		Raw:           raw,
	}
}
