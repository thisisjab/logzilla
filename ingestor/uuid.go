package ingestor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"uuid"
)

// LoadOrGenerateUUID reads an existing UUID from the given path.
// If the file does not exist or is empty, it generates a new UUID,
// writes it to the file, and returns it.
func LoadOrGenerateUUID(path string) (uuid.UUID, error) {
	if strings.TrimSpace(path) == "" {
		return uuid.Nil(), errors.New("uuid path cannot be empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return generateAndSaveUUID(path)
		}
		return uuid.Nil(), fmt.Errorf("failed to read uuid file %s: %w", path, err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return generateAndSaveUUID(path)
	}

	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("invalid uuid in file %s: %w", path, err)
	}

	return parsed, nil
}

func generateAndSaveUUID(path string) (uuid.UUID, error) {
	id := uuid.New()

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return uuid.Nil(), fmt.Errorf("failed to create directory for uuid file %s: %w", path, err)
		}
	}

	content := []byte(id.String() + "\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to write uuid to file %s: %w", path, err)
	}

	return id, nil
}
