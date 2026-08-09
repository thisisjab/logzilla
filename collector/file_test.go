package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileCollector(t *testing.T) {
	// Test FileCollector is created when file exists.
	t.Run("succeeds when file exists", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "app.log")

		_, err := os.Create(path)
		require.NoError(t, err)

		// Act
		fc, err := NewFileCollector(path)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, fc)
	})

	// Test FileCollector is not created when file does not exist.
	t.Run("fails when file does not exist", func(t *testing.T) {
		// Act
		fc, err := NewFileCollector("/a/path/that/does/not/exist")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, fc)
	})
}

func TestFileCollector_Collect(t *testing.T) {
	t.Run("returns error on no file", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "app.log")

		_, err := os.Create(path)
		require.NoError(t, err)

		fc, err := NewFileCollector(path)
		require.NoError(t, err)
		require.NotNil(t, fc)

		err = os.Remove(path)
		require.NoError(t, err)

		// Act
		err = fc.Collect(context.Background(), make(chan string, 1))

		// Assert
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	// Test FileCollector sends new lines on write events.
	t.Run("sends new lines on write", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "app.log")

		_, err := os.Create(path)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		// Act: create collector and write to file after a short delay
		fc, err := NewFileCollector(path)
		require.NoError(t, err)

		dest := make(chan string, 2)
		collectErr := make(chan error)

		go func() {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
			require.NoError(t, err)

			_, err = f.WriteString("line1\nline2\n")
			require.NoError(t, err, "cannot write to file")

			f.Close()
		}()

		go func() {
			collectErr <- fc.Collect(ctx, dest)
		}()

		var lines []string
		for range 2 {
			lines = append(lines, <-dest)
		}

		cancel()

		// Assert
		assert.ErrorIs(t, <-collectErr, context.Canceled)
		assert.Equal(t, []string{"line1", "line2"}, lines)
	})

	// Test FileCollector returns context.Canceled when context is cancelled.
	t.Run("returns context cancelled error", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "app.log")

		_, err := os.Create(path)
		require.NoError(t, err)

		fc, err := NewFileCollector(path)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		errChan := make(chan error, 1)

		// Act
		collect := func() {
			errChan <- fc.Collect(ctx, make(chan string, 1))
		}

		go collect()
		go cancel()

		err = <-errChan

		// Assert
		assert.ErrorIs(t, err, context.Canceled)
	})

	// Test FileCollector ignores empty writes (no new content).
	t.Run("ignores empty writes", func(t *testing.T) {
		// Arrange
		path := filepath.Join(t.TempDir(), "app.log")

		_, err := os.Create(path)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		// Act
		fc, err := NewFileCollector(path)
		require.NoError(t, err)

		collectErr := make(chan error)
		dest := make(chan string, 1)

		go func() {
			collectErr <- fc.Collect(ctx, dest)
		}()


		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
		require.NoError(t, err)

		_, err = f.WriteString("")
		require.NoError(t, err, "cannot write to file")

		f.Close()
		cancel()

		// Assert
		assert.ErrorIs(t, <-collectErr, context.Canceled)
		assert.Len(t, dest, 0)
	})
}
