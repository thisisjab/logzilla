package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Segment represents a discovered WAL segment file on disk.
type Segment struct {
	ID   int64
	Name string
	Path string
}

// ListSegments scans dir for WAL segment files ending in Extension with ID strictly
// greater than afterID, excluding excludeName, sorted in ascending chronological order.
func ListSegments(dir string, afterID int64, excludeName string) ([]Segment, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read WAL directory: %w", err)
	}

	var segments []Segment
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Skip non-.wal files and active/excluded segment
		if !strings.HasSuffix(name, Extension) || name == excludeName {
			continue
		}

		idStr := strings.TrimSuffix(name, Extension)
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}

		if id > afterID {
			segments = append(segments, Segment{
				ID:   id,
				Name: name,
				Path: filepath.Join(dir, name),
			})
		}
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].ID < segments[j].ID
	})

	return segments, nil
}
