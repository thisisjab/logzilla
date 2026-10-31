package ingestor

import (
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/thisisjab/logzilla/wal"
)

// encodeRecord marshals a collector identifier and log payload into a WAL record.
func encodeRecord(ingestorID uuid.UUID, collector, data string) wal.Record {
	return wal.EncodeRecord(ingestorID, 0, ulid.ULID{}, collector, data)
}
