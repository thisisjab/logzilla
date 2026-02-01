package source

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/thisisjab/logzilla/entity"
)

// FileLogSource works by watching a file for changes and reading new lines as they are written.
type FileLogSource struct {
	sourceName string
	filePath   string
}

// NewFileLogSource creates a new FileLogSource instance.
func NewFileLogSource(sourceName, filePath string) *FileLogSource {
	return &FileLogSource{
		filePath:   filePath,
		sourceName: sourceName,
	}
}

func (f *FileLogSource) SourceName() string {
	return f.sourceName
}

func (f *FileLogSource) Provide(ctx context.Context, logChan chan<- entity.RawLogRecord) error {
	file, err := os.Open(f.filePath)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}

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

	if err := watcher.Add(f.filePath); err != nil {
		return fmt.Errorf("cannot add file to watcher: %w", err)
	}

	reader := bufio.NewReader(file)

	go func() {
		defer watcher.Close()
		defer file.Close()

		for {
			select {
			case <-ctx.Done():
				return

			case event := <-watcher.Events:
				if !event.Has(fsnotify.Write) {
					continue
				}

				for {
					line, err := reader.ReadBytes('\n')
					if len(line) > 0 {
						logChan <- entity.RawLogRecord{
							Source:    f.SourceName(),
							Data:      line,
							Timestamp: time.Now(),
						}
					}

					if err == io.EOF {
						break
					}

					if err != nil {
						return
					}
				}

			case <-watcher.Errors:
				return
			}
		}
	}()

	return nil
}
