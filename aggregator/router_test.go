package aggregator

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessorConfigFromYAML(t *testing.T) {
	tempDir := t.TempDir()
	walDir := filepath.Join(tempDir, "wal")

	luaScriptPath := filepath.Join(tempDir, "custom.lua")
	luaScript := `
function process(raw, collector, node_id, timestamp)
    return {
        level = "info",
        message = "from-file: " .. raw,
    }
end
`
	require.NoError(t, os.WriteFile(luaScriptPath, []byte(luaScript), 0644))

	configPath := filepath.Join(tempDir, "aggregator.yaml")
	yamlContent := fmt.Sprintf(`
wal:
  dir: %q
processors:
  - name: inline-proc
    script: |
      function process(raw, collector, node_id, timestamp)
          return { level = "warn", message = "inline: " .. raw }
      end
    collectors:
      - "app-*"
      - "postgres-db"
  - name: file-proc
    script_path: %q
    collectors: "*.json"
`, walDir, luaScriptPath)

	require.NoError(t, os.WriteFile(configPath, []byte(yamlContent), 0644))

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	agg, err := New(Config{
		ConfigPath: configPath,
		Logger:     logger,
	})
	require.NoError(t, err)

	router := agg.Router()
	require.NotNil(t, router)

	// Verify routing to inline-proc
	proc1 := router.Route("app-service")
	require.NotNil(t, proc1)
	assert.Equal(t, "inline-proc", proc1.Name())

	proc2 := router.Route("postgres-db")
	require.NotNil(t, proc2)
	assert.Equal(t, "inline-proc", proc2.Name())

	// Verify routing to file-proc
	proc3 := router.Route("events.json")
	require.NotNil(t, proc3)
	assert.Equal(t, "file-proc", proc3.Name())
}

func TestBuildRouterFromViper_Validation(t *testing.T) {
	t.Run("empty processor name returns error", func(t *testing.T) {
		v := viper.New()
		v.Set("processors", []map[string]any{
			{"name": "  "},
		})
		_, err := buildRouterFromViper(v)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processor name cannot be empty")
	})

	t.Run("missing script and script_path returns error", func(t *testing.T) {
		v := viper.New()
		v.Set("processors", []map[string]any{
			{"name": "test-proc"},
		})
		_, err := buildRouterFromViper(v)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires either 'script' or 'script_path'")
	})

	t.Run("empty collectors returns error", func(t *testing.T) {
		v := viper.New()
		v.Set("processors", []map[string]any{
			{
				"name":       "test-proc",
				"script":     "function process() return nil end",
				"collectors": []string{},
			},
		})
		_, err := buildRouterFromViper(v)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must define at least one collector pattern")
	})

	t.Run("invalid lua script returns error", func(t *testing.T) {
		v := viper.New()
		v.Set("processors", []map[string]any{
			{
				"name":       "test-proc",
				"script":     "invalid syntax {{{{",
				"collectors": "*",
			},
		})
		_, err := buildRouterFromViper(v)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize processor")
	})
}
