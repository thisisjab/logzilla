package aggregator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thisisjab/logzilla/wal"
)

type walCandidate struct {
	id   int64
	name string
}

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
	entries, err := os.ReadDir(a.walDir)
	if err != nil {
		a.logger.Error("failed to read WAL dir in pipeline worker", "dir", a.walDir, "error", err)
		return
	}

	lastID := a.lastProcessedWalID.Load()

	var candidates []walCandidate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Ignore .tmp files as they represent in-flight uncommitted writes
		if !strings.HasSuffix(name, ".wal") {
			continue
		}

		idStr := strings.TrimSuffix(name, ".wal")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			a.logger.Warn("skipping WAL file with invalid timestamp filename", "file", name, "error", err)
			continue
		}

		// Only process files newer than our last completed cursor
		if id > lastID {
			candidates = append(candidates, walCandidate{id: id, name: name})
		}
	}

	// Process strictly in ascending timestamp order to preserve log chronology
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].id < candidates[j].id
	})

	for _, cand := range candidates {
		select {
		case <-ctx.Done():
			return
		default:
		}

		filePath := filepath.Join(a.walDir, cand.name)
		if err := a.processWALFile(filePath, cand.id); err != nil {
			a.logger.Error("failed to process WAL file in pipeline", "file", cand.name, "wal_id", cand.id, "error", err)
			return
		}

		a.lastProcessedWalID.Store(cand.id)
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
			if _, err := a.router.Process(record); err != nil {
				a.logger.Error("error processing record in pipeline", "collector", record.CollectorName, "error", err)
			}
		}
	}

	if err := it.Err(); err != nil {
		return fmt.Errorf("wal decode error in file %s: %w", filePath, err)
	}

	a.logger.Debug("wal file processed in pipeline", "file", filePath, "wal_id", walID)
	return nil
}
