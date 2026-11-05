package sstable

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"sort"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/thisisjab/logzilla/entity"
)

// Metadata value type discriminators.
const (
	TypeString  byte = 0x01
	TypeBool    byte = 0x02
	TypeInt64   byte = 0x03
	TypeFloat64 byte = 0x04
)

// MinRecordPayloadSize defines the minimal size of a record without variable string lengths.
// ts(8) + ulid(16) + node_id(16) + level(1) + col_len(2) + node_len(2) + msg_len(4) + meta_n(2) + raw_len(4) + crc32(4) = 59 bytes.
const MinRecordPayloadSize = 8 + 16 + 16 + 1 + 2 + 2 + 4 + 2 + 4 + 4

var (
	ErrRecordCorrupt       = errors.New("sstable record checksum mismatch")
	ErrUnsupportedMetaType = errors.New("unsupported metadata value type")
	ErrInvalidRecordLength = errors.New("invalid record payload length")
)

// EncodeRecordToBytes serializes an entity.LogRecord into SSTable binary format.
// Format: rec_len (u32) | payload | crc32 (u32).
func EncodeRecordToBytes(record entity.LogRecord) ([]byte, error) {
	var buf bytes.Buffer
	// Reserve 4 bytes for rec_len
	buf.Write([]byte{0, 0, 0, 0})

	var scratch [8]byte

	// 1. Timestamp (u64 unix ms)
	binary.BigEndian.PutUint64(scratch[:8], uint64(record.Timestamp))
	buf.Write(scratch[:8])

	// 2. ULID (16 bytes)
	buf.Write(record.ID[:])

	// 3. NodeID (16 bytes)
	buf.Write(record.NodeID[:])

	// 4. Level (u8)
	buf.WriteByte(byte(record.Level))

	// 5. Collector name (u16 len | bytes)
	if len(record.CollectorName) > math.MaxUint16 {
		return nil, fmt.Errorf("collector name exceeds max u16 length: %d", len(record.CollectorName))
	}
	binary.BigEndian.PutUint16(scratch[:2], uint16(len(record.CollectorName)))
	buf.Write(scratch[:2])
	buf.WriteString(record.CollectorName)

	// 6. Node name (u16 len | bytes)
	if len(record.NodeName) > math.MaxUint16 {
		return nil, fmt.Errorf("node name exceeds max u16 length: %d", len(record.NodeName))
	}
	binary.BigEndian.PutUint16(scratch[:2], uint16(len(record.NodeName)))
	buf.Write(scratch[:2])
	buf.WriteString(record.NodeName)

	// 7. Message (u32 len | bytes)
	if len(record.Message) > math.MaxUint32 {
		return nil, fmt.Errorf("message exceeds max u32 length: %d", len(record.Message))
	}
	binary.BigEndian.PutUint32(scratch[:4], uint32(len(record.Message)))
	buf.Write(scratch[:4])
	buf.WriteString(record.Message)

	// 8. Metadata (u16 n | n x (u16 klen | key | u8 type | u32 vlen | value))
	if len(record.Metadata) > math.MaxUint16 {
		return nil, fmt.Errorf("metadata key count exceeds max u16: %d", len(record.Metadata))
	}

	keys := make([]string, 0, len(record.Metadata))
	for k := range record.Metadata {
		keys = append(keys, k)
	}
	// Sort keys lexicographically for deterministic CRC32
	sort.Strings(keys)

	binary.BigEndian.PutUint16(scratch[:2], uint16(len(keys)))
	buf.Write(scratch[:2])

	for _, k := range keys {
		if len(k) > math.MaxUint16 {
			return nil, fmt.Errorf("metadata key %q exceeds max u16 length: %d", k, len(k))
		}
		binary.BigEndian.PutUint16(scratch[:2], uint16(len(k)))
		buf.Write(scratch[:2])
		buf.WriteString(k)

		val := record.Metadata[k]
		if err := encodeMetadataValue(&buf, scratch[:], k, val); err != nil {
			return nil, err
		}
	}

	// 9. Raw (u32 len | bytes)
	if len(record.Raw) > math.MaxUint32 {
		return nil, fmt.Errorf("raw text exceeds max u32 length: %d", len(record.Raw))
	}
	binary.BigEndian.PutUint32(scratch[:4], uint32(len(record.Raw)))
	buf.Write(scratch[:4])
	buf.WriteString(record.Raw)

	payload := buf.Bytes()[4:]

	// 10. CRC32 over payload
	crc := crc32.ChecksumIEEE(payload)
	binary.BigEndian.PutUint32(scratch[:4], crc)
	buf.Write(scratch[:4])

	res := buf.Bytes()
	// Total bytes following rec_len = len(payload) + 4 (crc)
	recLen := uint32(len(res) - 4)
	binary.BigEndian.PutUint32(res[:4], recLen)

	return res, nil
}

func encodeMetadataValue(buf *bytes.Buffer, scratch []byte, key string, val any) error {
	switch v := val.(type) {
	case string:
		buf.WriteByte(TypeString)
		binary.BigEndian.PutUint32(scratch[:4], uint32(len(v)))
		buf.Write(scratch[:4])
		buf.WriteString(v)
	case bool:
		buf.WriteByte(TypeBool)
		binary.BigEndian.PutUint32(scratch[:4], 1)
		buf.Write(scratch[:4])
		if v {
			buf.WriteByte(0x01)
		} else {
			buf.WriteByte(0x00)
		}
	case int:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case int64:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case int32:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case int16:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case int8:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case uint:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case uint64:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], v)
		buf.Write(scratch[:8])
	case uint32:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case uint16:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case uint8:
		buf.WriteByte(TypeInt64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], uint64(v))
		buf.Write(scratch[:8])
	case float64:
		buf.WriteByte(TypeFloat64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], math.Float64bits(v))
		buf.Write(scratch[:8])
	case float32:
		buf.WriteByte(TypeFloat64)
		binary.BigEndian.PutUint32(scratch[:4], 8)
		buf.Write(scratch[:4])
		binary.BigEndian.PutUint64(scratch[:8], math.Float64bits(float64(v)))
		buf.Write(scratch[:8])
	default:
		return fmt.Errorf("%w: key %q has unsupported type %T", ErrUnsupportedMetaType, key, val)
	}
	return nil
}

// EncodeRecord writes an encoded LogRecord directly to an io.Writer.
func EncodeRecord(w io.Writer, record entity.LogRecord) (int, error) {
	data, err := EncodeRecordToBytes(record)
	if err != nil {
		return 0, fmt.Errorf("encode record: %w", err)
	}
	n, err := w.Write(data)
	if err != nil {
		return n, fmt.Errorf("write record: %w", err)
	}
	return n, nil
}

// DecodeRecord reads an encoded LogRecord from an io.Reader.
// Returns io.EOF if reader reaches end before any bytes are read.
func DecodeRecord(r io.Reader) (entity.LogRecord, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return entity.LogRecord{}, io.EOF
		}
		return entity.LogRecord{}, fmt.Errorf("read record length: %w", err)
	}

	recLen := binary.BigEndian.Uint32(lenBuf[:])
	if recLen < MinRecordPayloadSize {
		return entity.LogRecord{}, fmt.Errorf("%w: %d bytes (min %d)", ErrInvalidRecordLength, recLen, MinRecordPayloadSize)
	}

	payload := make([]byte, recLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return entity.LogRecord{}, fmt.Errorf("read record payload: %w", err)
	}

	return decodeRecordPayload(payload)
}

// DecodeRecordFromBytes decodes a record from a byte slice.
// Returns the record and the total number of bytes consumed (4 + rec_len).
func DecodeRecordFromBytes(data []byte) (entity.LogRecord, int, error) {
	if len(data) < 4 {
		return entity.LogRecord{}, 0, fmt.Errorf("%w: less than 4 bytes", ErrInvalidRecordLength)
	}

	recLen := int(binary.BigEndian.Uint32(data[:4]))
	totalLen := 4 + recLen
	if recLen < MinRecordPayloadSize || len(data) < totalLen {
		return entity.LogRecord{}, 0, fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidRecordLength, totalLen, len(data))
	}

	record, err := decodeRecordPayload(data[4:totalLen])
	if err != nil {
		return entity.LogRecord{}, 0, err
	}

	return record, totalLen, nil
}

func decodeRecordPayload(payload []byte) (entity.LogRecord, error) {
	if len(payload) < MinRecordPayloadSize {
		return entity.LogRecord{}, fmt.Errorf("%w: payload too short", ErrInvalidRecordLength)
	}

	dataBytes := payload[:len(payload)-4]
	expectedCRC := binary.BigEndian.Uint32(payload[len(payload)-4:])
	actualCRC := crc32.ChecksumIEEE(dataBytes)

	if actualCRC != expectedCRC {
		return entity.LogRecord{}, fmt.Errorf("%w: expected 0x%08x, got 0x%08x", ErrRecordCorrupt, expectedCRC, actualCRC)
	}

	offset := 0

	// 1. Timestamp (8 bytes)
	ts := int64(binary.BigEndian.Uint64(dataBytes[offset : offset+8]))
	offset += 8

	// 2. ULID (16 bytes)
	var id ulid.ULID
	copy(id[:], dataBytes[offset:offset+16])
	offset += 16

	// 3. NodeID (16 bytes)
	var nodeID uuid.UUID
	copy(nodeID[:], dataBytes[offset:offset+16])
	offset += 16

	// 4. Level (1 byte)
	level := entity.LogLevel(dataBytes[offset])
	offset++

	// 5. Collector name
	if offset+2 > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	colLen := int(binary.BigEndian.Uint16(dataBytes[offset : offset+2]))
	offset += 2
	if offset+colLen > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	collectorName := string(dataBytes[offset : offset+colLen])
	offset += colLen

	// 6. Node name
	if offset+2 > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	nodeLen := int(binary.BigEndian.Uint16(dataBytes[offset : offset+2]))
	offset += 2
	if offset+nodeLen > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	nodeName := string(dataBytes[offset : offset+nodeLen])
	offset += nodeLen

	// 7. Message
	if offset+4 > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	msgLen := int(binary.BigEndian.Uint32(dataBytes[offset : offset+4]))
	offset += 4
	if offset+msgLen > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	message := string(dataBytes[offset : offset+msgLen])
	offset += msgLen

	// 8. Metadata
	if offset+2 > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	metaCount := int(binary.BigEndian.Uint16(dataBytes[offset : offset+2]))
	offset += 2

	metadata := make(map[string]any, metaCount)
	for i := 0; i < metaCount; i++ {
		if offset+2 > len(dataBytes) {
			return entity.LogRecord{}, ErrInvalidRecordLength
		}
		klen := int(binary.BigEndian.Uint16(dataBytes[offset : offset+2]))
		offset += 2
		if offset+klen > len(dataBytes) {
			return entity.LogRecord{}, ErrInvalidRecordLength
		}
		k := string(dataBytes[offset : offset+klen])
		offset += klen

		if offset+1+4 > len(dataBytes) {
			return entity.LogRecord{}, ErrInvalidRecordLength
		}
		valType := dataBytes[offset]
		offset++

		vlen := int(binary.BigEndian.Uint32(dataBytes[offset : offset+4]))
		offset += 4
		if offset+vlen > len(dataBytes) {
			return entity.LogRecord{}, ErrInvalidRecordLength
		}
		valBytes := dataBytes[offset : offset+vlen]
		offset += vlen

		val, err := decodeMetadataValue(valType, valBytes)
		if err != nil {
			return entity.LogRecord{}, fmt.Errorf("decode metadata key %q: %w", k, err)
		}
		metadata[k] = val
	}

	// 9. Raw
	if offset+4 > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	rawLen := int(binary.BigEndian.Uint32(dataBytes[offset : offset+4]))
	offset += 4
	if offset+rawLen > len(dataBytes) {
		return entity.LogRecord{}, ErrInvalidRecordLength
	}
	raw := string(dataBytes[offset : offset+rawLen])
	offset += rawLen

	if offset != len(dataBytes) {
		return entity.LogRecord{}, fmt.Errorf("%w: trailing data (%d unused bytes)", ErrInvalidRecordLength, len(dataBytes)-offset)
	}

	record := entity.New(
		id,
		nodeID,
		nodeName,
		collectorName,
		ts,
		level,
		message,
		metadata,
		raw,
	)

	return record, nil
}

func decodeMetadataValue(valType byte, b []byte) (any, error) {
	switch valType {
	case TypeString:
		return string(b), nil
	case TypeBool:
		if len(b) != 1 {
			return nil, fmt.Errorf("%w: bool length is %d (expected 1)", ErrInvalidRecordLength, len(b))
		}
		return b[0] != 0, nil
	case TypeInt64:
		if len(b) != 8 {
			return nil, fmt.Errorf("%w: int64 length is %d (expected 8)", ErrInvalidRecordLength, len(b))
		}
		return int64(binary.BigEndian.Uint64(b)), nil
	case TypeFloat64:
		if len(b) != 8 {
			return nil, fmt.Errorf("%w: float64 length is %d (expected 8)", ErrInvalidRecordLength, len(b))
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
	default:
		return nil, fmt.Errorf("%w: 0x%02x", ErrUnsupportedMetaType, valType)
	}
}
