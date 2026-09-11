package aggregator

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
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
			rec := encodeRecord(tt.collector, tt.data)

			// Verify total size matches expected:
			// 8 bytes (UnixMilli) + 16 bytes (ULID) + 4 bytes (collector len) + len(collector) + 4 bytes (data len) + len(data)
			expectedSize := 8 + 16 + 4 + len(tt.collector) + 4 + len(tt.data)
			assert.Len(t, rec, expectedSize)

			// Verify Unix Milliseconds (first 8 bytes)
			ts := binary.BigEndian.Uint64(rec[0:8])
			assert.InDelta(t, time.Now().UnixMilli(), int64(ts), 1000)

			// Verify ULID (next 16 bytes) is non-zero and valid
			var id ulid.ULID
			copy(id[:], rec[8:24])
			assert.NotEqual(t, ulid.ULID{}, id)

			// Verify Collector Name Length and Collector Name bytes
			colLen := binary.BigEndian.Uint32(rec[24:28])
			assert.Equal(t, uint32(len(tt.collector)), colLen)
			assert.Equal(t, []byte(tt.collector), []byte(rec[28:28+int(colLen)]))

			// Verify Log Data Length and Log Data bytes
			dataOffset := 28 + int(colLen)
			dataLen := binary.BigEndian.Uint32(rec[dataOffset : dataOffset+4])
			assert.Equal(t, uint32(len(tt.data)), dataLen)
			assert.Equal(t, []byte(tt.data), []byte(rec[dataOffset+4:dataOffset+4+int(dataLen)]))
		})
	}
}
