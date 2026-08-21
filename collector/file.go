package collector

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// FileCollector implements Collector interface. It's used to read
// logs from a file by tailing it.
type FileCollector struct {
	path string
}

// NewFileCollector creates a new file collector.
// Validates that the file exists before returning.
func NewFileCollector(path string) (*FileCollector, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("file does not exist: %w", err)
	}

	return &FileCollector{path: path}, nil
}

// Collect tails the configured file until ctx is cancelled.
//
// Behaviour:
//   - Opens the file and starts reading from its current end (like `tail -f`).
//   - Watches the file for write events using fsnotify.
//   - Sends each newly appended line to dest.
//   - Exits cleanly on context cancellation or watcher failure.
func (c *FileCollector) Collect(ctx context.Context, dest chan<- string) error {
	// Open the log file.
	file, err := os.Open(c.path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	// Ignore existing content and only read newly appended logs.
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("cannot seek to end: %w", err)
	}

	// Create a filesystem watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("cannot create watcher: %w", err)
	}
	defer watcher.Close()

	// Watch the target file.
	if err := watcher.Add(c.path); err != nil {
		return fmt.Errorf("cannot watch file: %w", err)
	}

	// Reader keeps its position on the same file descriptor,
	// allowing continuous reads as the file grows.
	reader := bufio.NewReader(file)

	// Receives exactly one terminal error from the worker goroutine.
	// The channel is closed on every exit path.
	watcherErr := make(chan error, 1)

	go func() {
		defer close(watcherErr)

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if !event.Has(fsnotify.Write) {
					continue
				}

				// Read every complete line that was appended by this write.
				for {
					line, err := reader.ReadString('\n')

					// EOF means we've consumed all currently available data.
					// Wait for the next write event.
					if errors.Is(err, io.EOF) {
						break
					}

					if err != nil {
						watcherErr <- fmt.Errorf("reader error: %w", err)
						return
					}

					// Forward the log line unless cancellation wins.
					select {
					case dest <- strings.TrimSuffix(line, "\n"):
					case <-ctx.Done():
						return
					}
				}

			// fsnotify encountered an internal error.
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				watcherErr <- fmt.Errorf("watcher error: %w", err)
				return

			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait until the worker exits. If it reported an error, return it.
	if err := <-watcherErr; err != nil {
		return err
	}

	// Normal termination is always caused by context cancellation.
	return context.Cause(ctx)
}
