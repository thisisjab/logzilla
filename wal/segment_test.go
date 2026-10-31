package wal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thisisjab/logzilla/wal"
)

func TestListSegments(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	files := []string{
		"100.wal",
		"300.wal",
		"200.wal",
		"150.wal.tmp-1234",
		"not-a-wal.txt",
		"invalid-id.wal",
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, f), []byte("test"), 0644))
	}
	// Subdirectory should be ignored
	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "sub.wal"), 0755))

	t.Run("lists all valid segments sorted ascending", func(t *testing.T) {
		segs, err := wal.ListSegments(tempDir, 0, "")
		require.NoError(t, err)
		require.Len(t, segs, 3)

		assert.Equal(t, int64(100), segs[0].ID)
		assert.Equal(t, "100.wal", segs[0].Name)
		assert.Equal(t, filepath.Join(tempDir, "100.wal"), segs[0].Path)

		assert.Equal(t, int64(200), segs[1].ID)
		assert.Equal(t, "200.wal", segs[1].Name)

		assert.Equal(t, int64(300), segs[2].ID)
		assert.Equal(t, "300.wal", segs[2].Name)
	})

	t.Run("filters by afterID", func(t *testing.T) {
		segs, err := wal.ListSegments(tempDir, 150, "")
		require.NoError(t, err)
		require.Len(t, segs, 2)

		assert.Equal(t, int64(200), segs[0].ID)
		assert.Equal(t, int64(300), segs[1].ID)
	})

	t.Run("excludes specified active file", func(t *testing.T) {
		segs, err := wal.ListSegments(tempDir, 0, "300.wal")
		require.NoError(t, err)
		require.Len(t, segs, 2)

		assert.Equal(t, int64(100), segs[0].ID)
		assert.Equal(t, int64(200), segs[1].ID)
	})

	t.Run("returns error on missing directory", func(t *testing.T) {
		_, err := wal.ListSegments(filepath.Join(tempDir, "missing"), 0, "")
		assert.Error(t, err)
	})
}
