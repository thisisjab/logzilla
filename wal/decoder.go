package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"uuid"

	"github.com/oklog/ulid/v2"
)

var (
	ErrUnexpectedEOF = errors.New("unexpected end of WAL data")
	ErrCorruptRecord = errors.New("corrupt WAL record header or length")
)

const (
	nodeIDSize    = 16
	timestampSize = 8
	recordIDSize  = 16
	colLenSize    = 4
	dataLenSize   = 4

	minRecordHeaderSize = nodeIDSize + timestampSize + recordIDSize + colLenSize + dataLenSize // 48 bytes
)

// UnpackedRecord represents a decoded record from a WAL file.
type UnpackedRecord struct {
	NodeID        uuid.UUID
	Timestamp     int64
	ID            ulid.ULID
	CollectorName string
	Raw           string
}

// RecordIterator streams UnpackedRecords from raw WAL bytes.
type RecordIterator struct {
	data []byte
	pos  int
	curr UnpackedRecord
	err  error
}

// NewRecordIterator creates a new iterator over the provided WAL byte slice.
func NewRecordIterator(data []byte) *RecordIterator {
	return &RecordIterator{
		data: data,
		pos:  0,
	}
}

// Next advances the iterator to the next record.
// Returns true if a record was successfully decoded, false on EOF or error.
func (it *RecordIterator) Next() bool {
	if it.err != nil || it.pos >= len(it.data) {
		return false
	}

	remaining := len(it.data) - it.pos
	if remaining < minRecordHeaderSize {
		it.err = fmt.Errorf("%w: remaining %d bytes, need at least %d", ErrUnexpectedEOF, remaining, minRecordHeaderSize)
		return false
	}

	// 1. Ingestor UUID (16 bytes)
	var nodeID uuid.UUID
	copy(nodeID[:], it.data[it.pos:it.pos+nodeIDSize])
	it.pos += nodeIDSize

	// 2. Timestamp (8 bytes BigEndian uint64)
	ts := int64(binary.BigEndian.Uint64(it.data[it.pos : it.pos+timestampSize]))
	it.pos += timestampSize

	// 3. ULID (16 bytes)
	var recordID ulid.ULID
	copy(recordID[:], it.data[it.pos:it.pos+recordIDSize])
	it.pos += recordIDSize

	// 4. Collector Name Length (4 bytes BigEndian uint32)
	colLen := int(binary.BigEndian.Uint32(it.data[it.pos : it.pos+colLenSize]))
	it.pos += colLenSize
	if colLen < 0 || colLen > remaining-minRecordHeaderSize {
		it.err = fmt.Errorf("%w: invalid collector length %d (remaining %d)", ErrCorruptRecord, colLen, remaining)
		return false
	}
	if it.pos+colLen > len(it.data) {
		it.err = fmt.Errorf("%w: collector data truncated", ErrUnexpectedEOF)
		return false
	}
	collectorName := string(it.data[it.pos : it.pos+colLen])
	it.pos += colLen

	// 5. Data Length (4 bytes BigEndian uint32)
	if it.pos+dataLenSize > len(it.data) {
		it.err = fmt.Errorf("%w: missing data length header", ErrUnexpectedEOF)
		return false
	}
	dataLen := int(binary.BigEndian.Uint32(it.data[it.pos : it.pos+dataLenSize]))
	it.pos += dataLenSize
	if dataLen < 0 {
		it.err = fmt.Errorf("%w: negative log data length %d", ErrCorruptRecord, dataLen)
		return false
	}
	if dataLen > len(it.data)-it.pos {
		it.err = fmt.Errorf("%w: expected %d log data bytes, but only %d remaining", ErrUnexpectedEOF, dataLen, len(it.data)-it.pos)
		return false
	}
	if it.pos+dataLen > len(it.data) {
		it.err = fmt.Errorf("%w: log data truncated", ErrUnexpectedEOF)
		return false
	}
	rawData := string(it.data[it.pos : it.pos+dataLen])
	it.pos += dataLen

	it.curr = UnpackedRecord{
		NodeID:        nodeID,
		Timestamp:     ts,
		ID:            recordID,
		CollectorName: collectorName,
		Raw:           rawData,
	}

	return true
}

// Record returns the current decoded UnpackedRecord.
func (it *RecordIterator) Record() UnpackedRecord {
	return it.curr
}

// Err returns the error encountered during iteration, if any.
func (it *RecordIterator) Err() error {
	if errors.Is(it.err, io.EOF) {
		return nil
	}
	return it.err
}

// DecodeAll decodes all records in data into a slice. Useful for small files or tests.
func DecodeAll(data []byte) ([]UnpackedRecord, error) {
	it := NewRecordIterator(data)
	var records []UnpackedRecord
	for it.Next() {
		records = append(records, it.Record())
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	return records, nil
}
