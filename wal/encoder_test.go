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

func TestEncodeRecord(t *testing.T) {
	tests := []struct {
		name      string
		collector string
		data      string
	}{
		{
			name:      "normal",
			collector: "file_collector_1",
			data:      "2026-09-01 20:45:00 INFO application started",
		},
		{
			name:      "no source name",
			collector: "",
			data:      "some log data without collector name",
		},
		{
			name:      "no data",
			collector: "syslog_collector",
			data:      "",
		},
		{
			name:      "multiline and formatted json",
			collector: "json_collector",
			data:      "{\n  \"timestamp\": \"2026-09-01T20:45:00Z\",\n  \"error\": \"panic: nil pointer dereference\\n\\tat main.go:42\"\n}",
		},
		{
			name:      "unicode and emojis",
			collector: "collector_🌍",
			data:      "🚨 Warning: high temperature 🔥 / ログテスト",
		},
		{
			name:      "binary payload with null bytes",
			collector: "bin_col",
			data:      "prefix\x00\x01\x02\xffsuffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID := uuid.New()
			rec := wal.EncodeRecord(nodeID, 0, ulid.ULID{}, tt.collector, tt.data)

			expectedSize := 16 + 8 + 16 + 4 + len(tt.collector) + 4 + len(tt.data)
			assert.Len(t, rec, expectedSize)

			// Verify Ingestor UUID (first 16 bytes)
			assert.Equal(t, nodeID[:], []byte(rec[0:16]))

			// Verify Unix Milliseconds (16..24)
			ts := binary.BigEndian.Uint64(rec[16:24])
			assert.InDelta(t, time.Now().UnixMilli(), int64(ts), 1000)

			// Verify ULID (24..40) is valid non-zero
			var id ulid.ULID
			copy(id[:], rec[24:40])
			assert.NotEqual(t, ulid.ULID{}, id)

			// Verify Collector Name
			colLen := binary.BigEndian.Uint32(rec[40:44])
			assert.Equal(t, uint32(len(tt.collector)), colLen)
			assert.Equal(t, []byte(tt.collector), []byte(rec[44:44+int(colLen)]))

			// Verify Data
			dataOffset := 44 + int(colLen)
			dataLen := binary.BigEndian.Uint32(rec[dataOffset : dataOffset+4])
			assert.Equal(t, uint32(len(tt.data)), dataLen)
			assert.Equal(t, []byte(tt.data), []byte(rec[dataOffset+4:dataOffset+4+int(dataLen)]))
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	nodeID := uuid.New()
	ts := int64(1730000000123)
	logID := ulid.Make()
	collector := "nginx-access"
	raw := "GET /api/v1/status 200 OK"

	rec := wal.EncodeRecord(nodeID, ts, logID, collector, raw)

	it := wal.NewRecordIterator(rec)
	require.True(t, it.Next())
	decoded := it.Record()

	assert.Equal(t, nodeID, decoded.NodeID)
	assert.Equal(t, ts, decoded.Timestamp)
	assert.Equal(t, logID, decoded.ID)
	assert.Equal(t, collector, decoded.CollectorName)
	assert.Equal(t, raw, decoded.Raw)

	assert.False(t, it.Next())
	require.NoError(t, it.Err())
}
