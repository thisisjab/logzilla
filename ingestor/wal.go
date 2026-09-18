package ingestor

import (
	"encoding/binary"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/thisisjab/logzilla/wal"
)

// encodeRecord marshals a collector identifier and log payload into the binary record layout:
// [8B BigEndian UnixMilli] [16B ULID] [4B BigEndian ColLen] [ColName] [4B BigEndian DataLen] [LogData]
func encodeRecord(collector, data string) wal.Record {
	timestamp := uint64(time.Now().UnixMilli())
	logID := ulid.Make()

	colLen := uint32(len(collector))
	dataLen := uint32(len(data))

	totalLen := 8 + 16 + 4 + int(colLen) + 4 + int(dataLen)
	rec := make(wal.Record, totalLen)

	// Write Unix Milliseconds (8 bytes)
	binary.BigEndian.PutUint64(rec[0:8], timestamp)

	// Write ULID (16 bytes)
	copy(rec[8:24], logID[:])

	// Write Collector Name Length (4 bytes)
	binary.BigEndian.PutUint32(rec[24:28], colLen)

	// Write Collector Name
	copy(rec[28:28+colLen], collector)

	// Write Log Data Length (4 bytes)
	binary.BigEndian.PutUint32(rec[28+colLen:32+colLen], dataLen)

	// Write Log Data
	copy(rec[32+colLen:32+colLen+dataLen], data)

	return rec
}
