package ingestor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrGenerateUUID_NotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ingestor.uuid")

	id, err := LoadOrGenerateUUID(path)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil(), id)

	// File should exist and contain the string representation
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, id.String()+"\n", string(content))

	// Second load should return the exact same UUID
	id2, err := LoadOrGenerateUUID(path)
	require.NoError(t, err)
	assert.Equal(t, id, id2)
}

func TestLoadOrGenerateUUID_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ingestor.uuid")

	// Create an empty file
	err := os.WriteFile(path, []byte(""), 0644)
	require.NoError(t, err)

	id, err := LoadOrGenerateUUID(path)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil(), id)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, id.String()+"\n", string(content))
}

func TestLoadOrGenerateUUID_WhitespaceOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ingestor.uuid")

	// Create file with only whitespace and newlines
	err := os.WriteFile(path, []byte("  \n\t\n  "), 0644)
	require.NoError(t, err)

	id, err := LoadOrGenerateUUID(path)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil(), id)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, id.String()+"\n", string(content))
}

func TestLoadOrGenerateUUID_ExistingValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ingestor.uuid")

	expectedID := uuid.New()
	err := os.WriteFile(path, []byte(strings.ToUpper(expectedID.String())+"\n"), 0644)
	require.NoError(t, err)

	id, err := LoadOrGenerateUUID(path)
	require.NoError(t, err)
	assert.Equal(t, expectedID, id)
}

func TestLoadOrGenerateUUID_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ingestor.uuid")

	err := os.WriteFile(path, []byte("not-a-valid-uuid-content"), 0644)
	require.NoError(t, err)

	id, err := LoadOrGenerateUUID(path)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil(), id)
	assert.Contains(t, err.Error(), "invalid uuid in file")
}

func TestLoadOrGenerateUUID_EmptyPath(t *testing.T) {
	id, err := LoadOrGenerateUUID("")
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil(), id)
	assert.Contains(t, err.Error(), "uuid path cannot be empty")
}

func TestLoadOrGenerateUUID_NestedDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "sub", ".ingestor.uuid")

	id, err := LoadOrGenerateUUID(path)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil(), id)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, id.String()+"\n", string(content))
}
