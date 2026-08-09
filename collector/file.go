package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

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

// Collect blocks until ctx is cancelled, sending new log lines to dest.
// Opens the file, seeks to end, then watches for write events.
// On write, scans and sends new lines. Returns context cancellation error.
func (c *FileCollector) Collect(ctx context.Context, dest chan<- string) error {
	file, err := os.Open(c.path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("cannot seek to end: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("cannot create watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(c.path); err != nil {
		return fmt.Errorf("cannot watch file: %w", err)
	}

	// NOTE: We can use larger buffer.
	scanner := bufio.NewScanner(file)

	// Buffered so the goroutine's send below never blocks on us reading it.
	watcherErr := make(chan error, 1)

	go func() {
		// Guarantees watcherErr is ALWAYS closed when this goroutine exits,
		// on every return path below (including the ctx.Done() one). This
		// is what makes the blocking <-watcherErr below safe: it can never
		// hang forever, because this defer always fires eventually.
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

				for scanner.Scan() {
					select {
					case dest <- scanner.Text():
					case <-ctx.Done():
						// Cancelled mid-send: stop immediately, don't
						// block trying to push more lines to dest.
						return
					}
				}

				if err := scanner.Err(); err != nil {
					watcherErr <- fmt.Errorf("scanner error: %w", err)
					return
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				watcherErr <- err
				return

			case <-ctx.Done():
				// Cancelled while idle, waiting for the next event.
				return
			}
		}
	}()

	// Block until the goroutine actually stops and closes watcherErr.
	// This is NOT racing ctx.Done() separately — we deliberately wait
	// for the goroutine's own exit signal instead of returning the
	// instant ctx is cancelled, so we never return to the caller while
	// the goroutine might still be running (e.g. mid dest<- send).
	// Because the goroutine selects on ctx.Done() at every blocking
	// point, it notices cancellation immediately, so this is not slow —
	// it's just correct.
	err = <-watcherErr
	if err != nil {
		return fmt.Errorf("watcher error: %w", err)
	}

	return context.Cause(ctx)
}
