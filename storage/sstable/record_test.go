package sstable

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/thisisjab/logzilla/entity"
)

func TestRecordCodec_Roundtrip(t *testing.T) {
	nodeID := uuid.MustParse("a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d")
	now := time.Now().UTC()
	entropy := ulid.Monotonic(bytes.NewReader([]byte("deterministic_entropy_source_")), 0)
	recordID := ulid.MustNew(ulid.Timestamp(now), entropy)

	tests := []struct {
		name   string
		record entity.LogRecord
	}{
		{
			name: "basic record with string metadata",
			record: entity.New(
				recordID,
				nodeID,
				"worker-node-1",
				"system-auth",
				now.UnixMilli(),
				entity.LevelInfo,
				"User logged in successfully",
				map[string]any{
					"username": "alice",
					"ip":       "192.168.1.100",
				},
				"Nov 04 22:15:00 worker-node-1 system-auth: User logged in successfully",
			),
		},
		{
			name: "record with diverse metadata types",
			record: entity.New(
				recordID,
				nodeID,
				"k8s-pod-7",
				"app-orders",
				now.UnixMilli(),
				entity.LevelWarn,
				"Order payment delayed",
				map[string]any{
					"is_retry":   true,
					"is_sandbox": false,
					"attempts":   int64(3),
					"latency_ms": float64(142.85),
					"status":     "PENDING",
				},
				`{"level":"warn","message":"Order payment delayed","attempts":3}`,
			),
		},
		{
			name: "record with empty metadata and empty strings",
			record: entity.New(
				recordID,
				nodeID,
				"",
				"",
				now.UnixMilli(),
				entity.LevelDebug,
				"",
				nil,
				"",
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := EncodeRecordToBytes(tc.record)
			require.NoError(t, err)
			require.NotEmpty(t, data)

			decoded, consumed, err := DecodeRecordFromBytes(data)
			require.NoError(t, err)
			require.Equal(t, len(data), consumed)

			require.Equal(t, tc.record.ID, decoded.ID)
			require.Equal(t, tc.record.NodeID, decoded.NodeID)
			require.Equal(t, tc.record.NodeName, decoded.NodeName)
			require.Equal(t, tc.record.CollectorName, decoded.CollectorName)
			require.Equal(t, tc.record.Timestamp, decoded.Timestamp)
			require.Equal(t, tc.record.Level, decoded.Level)
			require.Equal(t, tc.record.Message, decoded.Message)
			require.Equal(t, tc.record.Raw, decoded.Raw)

			require.Equal(t, len(tc.record.Metadata), len(decoded.Metadata))
			for k, v := range tc.record.Metadata {
				switch expected := v.(type) {
				case int:
					require.Equal(t, int64(expected), decoded.Metadata[k])
				case float32:
					require.InDelta(t, float64(expected), decoded.Metadata[k], 1e-6)
				default:
					require.Equal(t, v, decoded.Metadata[k])
				}
			}
		})
	}
}

func TestRecordCodec_DeterministicEncoding(t *testing.T) {
	nodeID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	var id ulid.ULID

	meta1 := map[string]any{
		"zebra": "last",
		"apple": "first",
		"mango": "middle",
		"count": int64(42),
	}
	meta2 := map[string]any{
		"apple": "first",
		"count": int64(42),
		"zebra": "last",
		"mango": "middle",
	}

	r1 := entity.New(id, nodeID, "node", "collector", 1000, entity.LevelInfo, "msg", meta1, "raw")
	r2 := entity.New(id, nodeID, "node", "collector", 1000, entity.LevelInfo, "msg", meta2, "raw")

	b1, err := EncodeRecordToBytes(r1)
	require.NoError(t, err)

	b2, err := EncodeRecordToBytes(r2)
	require.NoError(t, err)

	require.Equal(t, b1, b2, "encoded byte streams must be identical regardless of map iteration order")
}

func TestRecordCodec_CRCValidation(t *testing.T) {
	record := entity.New(
		ulid.Make(),
		uuid.New(),
		"node-1",
		"col-1",
		123456789,
		entity.LevelError,
		"Something failed",
		map[string]any{"code": int64(500)},
		"raw text",
	)

	data, err := EncodeRecordToBytes(record)
	require.NoError(t, err)

	// Flip a bit in the payload
	corrupted := make([]byte, len(data))
	copy(corrupted, data)
	corrupted[10] ^= 0xFF

	_, _, err = DecodeRecordFromBytes(corrupted)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrRecordCorrupt))
}

func TestRecordCodec_StreamIO(t *testing.T) {
	records := []entity.LogRecord{
		entity.New(ulid.Make(), uuid.New(), "node1", "col1", 100, entity.LevelInfo, "msg1", nil, "raw1"),
		entity.New(ulid.Make(), uuid.New(), "node2", "col2", 200, entity.LevelWarn, "msg2", map[string]any{"k": "v"}, "raw2"),
		entity.New(ulid.Make(), uuid.New(), "node3", "col3", 300, entity.LevelError, "msg3", map[string]any{"n": int64(1)}, "raw3"),
	}

	var buf bytes.Buffer
	for _, rec := range records {
		n, err := EncodeRecord(&buf, rec)
		require.NoError(t, err)
		require.Positive(t, n)
	}

	reader := bytes.NewReader(buf.Bytes())
	for i, expected := range records {
		decoded, err := DecodeRecord(reader)
		require.NoError(t, err, "failed to decode record %d", i)
		require.Equal(t, expected.Timestamp, decoded.Timestamp)
		require.Equal(t, expected.Message, decoded.Message)
	}

	// EOF check on clean stream termination
	_, err := DecodeRecord(reader)
	require.ErrorIs(t, err, io.EOF)
}

func TestRecordCodec_InvalidLengths(t *testing.T) {
	t.Run("less than 4 bytes", func(t *testing.T) {
		_, _, err := DecodeRecordFromBytes([]byte{0, 1, 2})
		require.ErrorIs(t, err, ErrInvalidRecordLength)
	})

	t.Run("rec_len claims more than buffer has", func(t *testing.T) {
		buf := make([]byte, 20)
		// claim 100 bytes
		buf[3] = 100
		_, _, err := DecodeRecordFromBytes(buf)
		require.ErrorIs(t, err, ErrInvalidRecordLength)
	})

	t.Run("rec_len smaller than minimal record payload", func(t *testing.T) {
		buf := make([]byte, 10)
		buf[3] = 5
		_, _, err := DecodeRecordFromBytes(buf)
		require.ErrorIs(t, err, ErrInvalidRecordLength)
	})
}

func TestRecordCodec_AllNumericTypes(t *testing.T) {
	meta := map[string]any{
		"int":     int(-42),
		"int8":    int8(-8),
		"int16":   int16(-16),
		"int32":   int32(-32),
		"int64":   int64(-64),
		"uint":    uint(42),
		"uint8":   uint8(8),
		"uint16":  uint16(16),
		"uint32":  uint32(32),
		"uint64":  uint64(64),
		"float32": float32(3.14),
		"float64": float64(2.71828),
		"bool_t":  true,
		"bool_f":  false,
		"str":     "hello",
	}

	record := entity.New(ulid.Make(), uuid.New(), "node", "collector", 1000, entity.LevelInfo, "msg", meta, "raw")
	data, err := EncodeRecordToBytes(record)
	require.NoError(t, err)

	decoded, _, err := DecodeRecordFromBytes(data)
	require.NoError(t, err)

	require.Equal(t, int64(-42), decoded.Metadata["int"])
	require.Equal(t, int64(-8), decoded.Metadata["int8"])
	require.Equal(t, int64(-16), decoded.Metadata["int16"])
	require.Equal(t, int64(-32), decoded.Metadata["int32"])
	require.Equal(t, int64(-64), decoded.Metadata["int64"])
	require.Equal(t, int64(42), decoded.Metadata["uint"])
	require.Equal(t, int64(8), decoded.Metadata["uint8"])
	require.Equal(t, int64(16), decoded.Metadata["uint16"])
	require.Equal(t, int64(32), decoded.Metadata["uint32"])
	require.Equal(t, int64(64), decoded.Metadata["uint64"])
	require.InDelta(t, float64(float32(3.14)), decoded.Metadata["float32"], 1e-6)
	require.Equal(t, float64(2.71828), decoded.Metadata["float64"])
	require.Equal(t, true, decoded.Metadata["bool_t"])
	require.Equal(t, false, decoded.Metadata["bool_f"])
	require.Equal(t, "hello", decoded.Metadata["str"])
}

func TestRecordCodec_DecodeMetadataValueErrors(t *testing.T) {
	_, err := decodeMetadataValue(TypeBool, []byte{1, 2})
	require.ErrorIs(t, err, ErrInvalidRecordLength)

	_, err = decodeMetadataValue(TypeInt64, []byte{1, 2, 3, 4})
	require.ErrorIs(t, err, ErrInvalidRecordLength)

	_, err = decodeMetadataValue(TypeFloat64, []byte{1, 2, 3, 4})
	require.ErrorIs(t, err, ErrInvalidRecordLength)

	_, err = decodeMetadataValue(0x99, []byte{1})
	require.ErrorIs(t, err, ErrUnsupportedMetaType)
}

func TestRecordCodec_UnsupportedMetadataType(t *testing.T) {
	record := entity.New(
		ulid.Make(),
		uuid.New(),
		"node",
		"col",
		100,
		entity.LevelInfo,
		"msg",
		map[string]any{
			"nested": map[string]string{"foo": "bar"},
		},
		"raw",
	)

	_, err := EncodeRecordToBytes(record)
	require.ErrorIs(t, err, ErrUnsupportedMetaType)
}
