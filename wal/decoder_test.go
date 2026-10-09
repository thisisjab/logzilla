package wal_test

import (
	"encoding/binary"
	"testing"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thisisjab/logzilla/wal"
)

// helper to encode a record matching the wire layout
func encodeTestRecord(nodeID uuid.UUID, ts int64, recordID ulid.ULID, collector, data string) []byte {
	colLen := uint32(len(collector))
	dataLen := uint32(len(data))
	totalLen := 16 + 8 + 16 + 4 + int(colLen) + 4 + int(dataLen)
	rec := make([]byte, totalLen)

	copy(rec[0:16], nodeID[:])
	binary.BigEndian.PutUint64(rec[16:24], uint64(ts))
	copy(rec[24:40], recordID[:])
	binary.BigEndian.PutUint32(rec[40:44], colLen)
	copy(rec[44:44+colLen], collector)
	binary.BigEndian.PutUint32(rec[44+colLen:48+colLen], dataLen)
	copy(rec[48+colLen:], data)

	return rec
}

func TestRecordIterator_SingleAndMultiple(t *testing.T) {
	node1 := uuid.New()
	node2 := uuid.New()
	id1 := ulid.Make()
	id2 := ulid.Make()
	now := time.Now().UnixMilli()

	rec1 := encodeTestRecord(node1, now, id1, "collector-1", "hello log line 1")
	rec2 := encodeTestRecord(node2, now+100, id2, "collector-2", "hello log line 2")

	combined := append(rec1, rec2...)

	it := wal.NewRecordIterator(combined)

	// First record
	require.True(t, it.Next())
	r1 := it.Record()
	assert.Equal(t, node1, r1.NodeID)
	assert.Equal(t, now, r1.Timestamp)
	assert.Equal(t, id1, r1.ID)
	assert.Equal(t, "collector-1", r1.CollectorName)
	assert.Equal(t, "hello log line 1", r1.Raw)

	// Second record
	require.True(t, it.Next())
	r2 := it.Record()
	assert.Equal(t, node2, r2.NodeID)
	assert.Equal(t, now+100, r2.Timestamp)
	assert.Equal(t, id2, r2.ID)
	assert.Equal(t, "collector-2", r2.CollectorName)
	assert.Equal(t, "hello log line 2", r2.Raw)

	// End
	assert.False(t, it.Next())
	assert.NoError(t, it.Err())
}

func TestDecodeAll_Empty(t *testing.T) {
	records, err := wal.DecodeAll([]byte{})
	require.NoError(t, err)
	assert.Empty(t, records)

	records, err = wal.DecodeAll(nil)
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestRecordIterator_TruncatedHeaders(t *testing.T) {
	node := uuid.New()
	rec := encodeTestRecord(node, 1000, ulid.Make(), "col", "some data")

	// Truncate before min header length (< 48 bytes)
	it := wal.NewRecordIterator(rec[:30])
	assert.False(t, it.Next())
	assert.ErrorIs(t, it.Err(), wal.ErrUnexpectedEOF)
}

func TestRecordIterator_TruncatedPayload(t *testing.T) {
	node := uuid.New()
	rec := encodeTestRecord(node, 1000, ulid.Make(), "collector-abc", "sample payload 12345")

	// Cut off last 5 bytes of data
	truncated := rec[:len(rec)-5]
	it := wal.NewRecordIterator(truncated)
	assert.False(t, it.Next())
	assert.ErrorIs(t, it.Err(), wal.ErrUnexpectedEOF)
}

func TestRecordIterator_CorruptLengths(t *testing.T) {
	node := uuid.New()
	rec := encodeTestRecord(node, 1000, ulid.Make(), "col", "data")

	// Corrupt collector length to be impossibly large
	corruptCol := make([]byte, len(rec))
	copy(corruptCol, rec)
	binary.BigEndian.PutUint32(corruptCol[40:44], 999999)

	it := wal.NewRecordIterator(corruptCol)
	assert.False(t, it.Next())
	assert.ErrorIs(t, it.Err(), wal.ErrCorruptRecord)
}
