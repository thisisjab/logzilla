package aggregator

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/thisisjab/logzilla/wal"
)

// runWorker periodically scans for newly persisted WAL files and streams
// their records through the processor router until ctx is canceled.
func (a *Aggregator) runWorker(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Initial sweep on startup to catch files already present on disk
	a.processNewWALFiles(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.processNewWALFiles(ctx)
		}
	}
}

// processNewWALFiles scans walDir for closed segments with id > lastProcessedWalID.
func (a *Aggregator) processNewWALFiles(ctx context.Context) {
	candidates, err := wal.ListSegments(a.walDir, a.lastProcessedWalID.Load(), "")
	if err != nil {
		a.logger.Error("failed to list WAL segments in pipeline worker", "dir", a.walDir, "error", err)
		return
	}

	for _, cand := range candidates {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := a.processWALFile(cand.Path, cand.ID); err != nil {
			a.logger.Error("failed to process WAL file in pipeline", "file", cand.Name, "wal_id", cand.ID, "error", err)
			return
		}

		a.lastProcessedWalID.Store(cand.ID)
	}
}

func (a *Aggregator) processWALFile(filePath string, walID int64) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read wal file: %w", err)
	}

	it := wal.NewRecordIterator(data)
	for it.Next() {
		record := it.Record()
		if a.router != nil {
			processed, err := a.router.Process(record)
			if err != nil {
				a.logger.Error("error processing record in pipeline", "collector", record.CollectorName, "error", err)
				continue
			}
			if processed != nil && a.storage != nil {
				if err := a.storage.Put(*processed); err != nil {
					a.logger.Error("error storing record in memtable", "error", err)
				}
			}
		}
	}

	if err := it.Err(); err != nil {
		return fmt.Errorf("wal decode error in file %s: %w", filePath, err)
	}

	a.logger.Debug("wal file processed in pipeline", "file", filePath, "wal_id", walID)
	return nil
}
