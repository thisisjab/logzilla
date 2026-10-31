package wal

import (
	"encoding/binary"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
)

// EncodeRecord serializes log metadata and payload into the binary WAL record layout:
// [16B Ingestor UUID] [8B UnixMilli] [16B ULID] [4B ColLen] [ColName] [4B DataLen] [LogData]
// If timestamp <= 0, the current Unix millisecond timestamp is used.
// If id is empty, a new ULID is generated.
func EncodeRecord(nodeID uuid.UUID, timestamp int64, id ulid.ULID, collector, data string) Record {
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}
	if id == (ulid.ULID{}) {
		id = ulid.Make()
	}

	colLen := uint32(len(collector))
	dataLen := uint32(len(data))

	totalLen := nodeIDSize + timestampSize + recordIDSize + colLenSize + int(colLen) + dataLenSize + int(dataLen)
	rec := make(Record, totalLen)

	// 1. Ingestor UUID (16 bytes)
	copy(rec[0:16], nodeID[:])

	// 2. Unix Milliseconds (8 bytes BigEndian uint64)
	binary.BigEndian.PutUint64(rec[16:24], uint64(timestamp))

	// 3. ULID (16 bytes)
	copy(rec[24:40], id[:])

	// 4. Collector Name Length (4 bytes BigEndian uint32)
	binary.BigEndian.PutUint32(rec[40:44], colLen)

	// 5. Collector Name
	copy(rec[44:44+colLen], collector)

	// 6. Log Data Length (4 bytes BigEndian uint32)
	offset := 44 + int(colLen)
	binary.BigEndian.PutUint32(rec[offset:offset+4], dataLen)

	// 7. Log Data
	copy(rec[offset+4:], data)

	return rec
}
