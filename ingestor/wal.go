package ingestor

import (
	"encoding/binary"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/thisisjab/logzilla/wal"
)

// encodeRecord marshals a collector identifier and log payload into the binary record layout:
// [16B UUID] [8B BigEndian UnixMilli] [16B ULID] [4B BigEndian ColLen] [ColName] [4B BigEndian DataLen] [LogData]
func encodeRecord(ingestorID uuid.UUID, collector, data string) wal.Record {
	timestamp := uint64(time.Now().UnixMilli())
	logID := ulid.Make()

	colLen := uint32(len(collector))
	dataLen := uint32(len(data))

	totalLen := 16 + 8 + 16 + 4 + int(colLen) + 4 + int(dataLen)
	rec := make(wal.Record, totalLen)

	// Write Ingestor UUID (16 bytes)
	copy(rec[0:16], ingestorID[:])

	// Write Unix Milliseconds (8 bytes)
	binary.BigEndian.PutUint64(rec[16:24], timestamp)

	// Write ULID (16 bytes)
	copy(rec[24:40], logID[:])

	// Write Collector Name Length (4 bytes)
	binary.BigEndian.PutUint32(rec[40:44], colLen)

	// Write Collector Name
	copy(rec[44:44+colLen], collector)

	// Write Log Data Length (4 bytes)
	binary.BigEndian.PutUint32(rec[44+colLen:48+colLen], dataLen)

	// Write Log Data
	copy(rec[48+colLen:48+colLen+dataLen], data)

	return rec
}
