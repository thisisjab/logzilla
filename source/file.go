package source

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/thisisjab/logzilla/entity"
)

// FileLogSource works by watching a file for changes and reading new lines as they are written.
type FileLogSource struct {
	sourceName     string
	filePath       string
	processorNames []string
	logger         *slog.Logger
}

// NewFileLogSource creates a new FileLogSource instance.
func NewFileLogSource(logger *slog.Logger, sourceName, filePath string, processorNames []string) *FileLogSource {
	return &FileLogSource{
		logger:         logger,
		filePath:       filePath,
		sourceName:     sourceName,
		processorNames: processorNames,
	}
}

func (f *FileLogSource) SourceName() string {
	return f.sourceName
}

func (f *FileLogSource) ProcessorNames() []string {
	return f.processorNames
}

func (f *FileLogSource) Provide(ctx context.Context, logChan chan<- entity.LogRecord) error {
	file, err := os.Open(f.filePath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	// Always seek to the end of the file
	// Note that when file is read (when notified by fsnotify), the cursor will move to end of file
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("cannot create watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(f.filePath); err != nil {
		return fmt.Errorf("cannot add file to watcher: %w", err)
	}

	reader := bufio.NewReader(file)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-watcher.Events:
			if !ok {
				f.logger.Debug("fsnotify watcher channel is closed.")
				return nil
			}
			if !event.Has(fsnotify.Write) {
				// TODO: handle file rotation
				// Editors like vim, create a new file and rewrite all changes, when even a single line is appended.
				// This creates a new inode and file watcher will not be notified about the change, since it tracks files
				// based on the inode.
				// I should handle this issue, by checking if the file has been rotated and if so, reopen the file and
				// start reading from the beginning.
				// Btw, in normal environment, no one performs such actions and they use linux append to append to file
				// which preserves the inode.
				f.logger.Debug("Received unhandled event from fsnotify.", "event", event.String())
				continue
			}

			for {
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					l := entity.LogRecord{
						Source:    f.SourceName(),
						RawData:   line,
						Timestamp: time.Now(),
					}
					logChan <- l
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}
