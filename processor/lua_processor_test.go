package processor_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/wal"
)

func sampleRecord(raw string) wal.UnpackedRecord {
	return wal.UnpackedRecord{
		NodeID:        uuid.MustParse("01999999-0000-7000-8000-000000000001"),
		Timestamp:     1760000000000,
		ID:            ulid.Make(),
		CollectorName: "app-service",
		Raw:           raw,
	}
}

func TestLuaProcessor_JSON(t *testing.T) {
	script := `
		local json = require("json")

		function process(raw, collector, node_id, timestamp)
			local data = json.decode(raw)
			return {
				level = data.level,
				message = data.msg,
				timestamp = data.time,
				metadata = {
					service = data.service,
					count = data.count,
					ratio = data.ratio,
					enabled = data.enabled,
					tags = data.tags,
					nested = data.nested
				}
			}
		end
	`

	proc, err := processor.NewLuaProcessor("json-proc", script)
	require.NoError(t, err)
	assert.Equal(t, "json-proc", proc.Name())

	rec := sampleRecord(`{"level":"warn","msg":"cache miss","time":1760000005000,"service":"auth","count":10,"ratio":3.14,"enabled":true,"tags":["auth","cache"],"nested":{"k":"v"}}`)

	out, err := proc.Process(rec)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.False(t, out.HasError())
	assert.NoError(t, out.Err())
	assert.Equal(t, entity.LevelWarn, out.Level)
	assert.Equal(t, "cache miss", out.Message)
	assert.Equal(t, int64(1760000005000), out.Timestamp)
	assert.Equal(t, rec.Raw, out.Raw)
	assert.Equal(t, "auth", out.Metadata["service"])
	assert.Equal(t, int64(10), out.Metadata["count"])
	assert.Equal(t, 3.14, out.Metadata["ratio"])
	assert.Equal(t, true, out.Metadata["enabled"])
	assert.Equal(t, []any{"auth", "cache"}, out.Metadata["tags"])
	assert.Equal(t, map[string]any{"k": "v"}, out.Metadata["nested"])
}

func TestLuaProcessor_PatternMatching(t *testing.T) {
	script := `
		DATETIME_FORMAT = "02/Jan/2006:15:04:05 -0700"

		function process(raw, collector, node_id, timestamp)
			local ip, ts, method, path, status = string.match(
				raw,
				'^(%S+) %S+ %S+ %[(.-)%] "(%S+) (%S+) %S+" (%d+)'
			)

			if not ip then
				return nil
			end

			return {
				level = "info",
				message = method .. " " .. path,
				timestamp = ts,
				metadata = {
					client_ip = ip,
					status = tonumber(status)
				}
			}
		end
	`

	proc, err := processor.NewLuaProcessor("nginx-proc", script)
	require.NoError(t, err)

	rawLine := `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326`
	rec := sampleRecord(rawLine)

	out, err := proc.Process(rec)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.False(t, out.HasError())
	assert.Equal(t, entity.LevelInfo, out.Level)
	assert.Equal(t, "GET /index.html", out.Message)
	assert.Equal(t, "127.0.0.1", out.Metadata["client_ip"])
	assert.Equal(t, int64(200), out.Metadata["status"])

	expectedTime, err := time.Parse("02/Jan/2006:15:04:05 -0700", "10/Oct/2000:13:55:36 -0700")
	require.NoError(t, err)
	assert.Equal(t, expectedTime.UnixMilli(), out.Timestamp)
}

func TestLuaProcessor_Filtering(t *testing.T) {
	script := `
		local json = require("json")

		function process(raw, collector, node_id, timestamp)
			local data = json.decode(raw)
			if data.drop == true then
				return nil
			end
			return {
				level = "info",
				message = data.msg
			}
		end
	`

	proc, err := processor.NewLuaProcessor("filter-proc", script)
	require.NoError(t, err)

	t.Run("dropped record returns nil", func(t *testing.T) {
		rec := sampleRecord(`{"drop":true,"msg":"ignore me"}`)
		out, err := proc.Process(rec)
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("retained record returns LogRecord", func(t *testing.T) {
		rec := sampleRecord(`{"drop":false,"msg":"keep me"}`)
		out, err := proc.Process(rec)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "keep me", out.Message)
	})
}

func TestLuaProcessor_ScriptCrash(t *testing.T) {
	script := `
		function process(raw, collector, node_id, timestamp)
			error("simulated runtime crash")
		end
	`

	proc, err := processor.NewLuaProcessor("crash-proc", script)
	require.NoError(t, err)

	rec := sampleRecord("unparsed crash payload")
	out, err := proc.Process(rec)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.True(t, out.HasError())
	require.Error(t, out.Err())
	assert.Contains(t, out.Err().Error(), "simulated runtime crash")
	assert.Equal(t, entity.LevelUnknown, out.Level)
	assert.Equal(t, rec.Raw, out.Raw)
	assert.Equal(t, rec.Timestamp, out.Timestamp)
	assert.Equal(t, rec.NodeID, out.NodeID)
	assert.Equal(t, rec.CollectorName, out.CollectorName)
}

func TestLuaProcessor_InvalidDatetime(t *testing.T) {
	script := `
		DATETIME_FORMAT = "2006-01-02 15:04:05"

		function process(raw, collector, node_id, timestamp)
			return {
				level = "info",
				message = "bad date",
				timestamp = "not-a-valid-date"
			}
		end
	`

	proc, err := processor.NewLuaProcessor("bad-date-proc", script)
	require.NoError(t, err)

	rec := sampleRecord("payload")
	out, err := proc.Process(rec)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.True(t, out.HasError())
	assert.Equal(t, int64(0), out.Timestamp)
	assert.Contains(t, out.Err().Error(), "failed to parse timestamp")
}

func TestLuaProcessor_InvalidReturnType(t *testing.T) {
	script := `
		function process(raw, collector, node_id, timestamp)
			return "string-instead-of-table"
		end
	`

	proc, err := processor.NewLuaProcessor("bad-return-proc", script)
	require.NoError(t, err)

	rec := sampleRecord("payload")
	out, err := proc.Process(rec)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.True(t, out.HasError())
	assert.Contains(t, out.Err().Error(), "expected process() to return table or nil")
}

func TestLuaProcessor_ConstructorErrors(t *testing.T) {
	t.Run("empty_name", func(t *testing.T) {
		_, err := processor.NewLuaProcessor("", "function process() end")
		require.Error(t, err)
	})

	t.Run("missing_process_function", func(t *testing.T) {
		_, err := processor.NewLuaProcessor("missing-proc", "local x = 1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must define a global \"process\" function")
	})

	t.Run("syntax_error", func(t *testing.T) {
		_, err := processor.NewLuaProcessor("syntax-err", "function invalid(")
		require.Error(t, err)
	})

	t.Run("file_not_found", func(t *testing.T) {
		_, err := processor.NewLuaProcessorFromFile("missing-file", "/non/existent/path.lua")
		require.Error(t, err)
	})

	t.Run("valid_file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "script.lua")
		err := os.WriteFile(filePath, []byte("function process() return nil end"), 0644)
		require.NoError(t, err)

		proc, err := processor.NewLuaProcessorFromFile("file-proc", filePath)
		require.NoError(t, err)
		assert.Equal(t, "file-proc", proc.Name())
	})
}

func BenchmarkLuaProcessor_JSON(b *testing.B) {
	script := `
		local json = require("json")

		function process(raw, collector, node_id, timestamp)
			local data = json.decode(raw)
			return {
				level = data.level,
				message = data.msg,
				timestamp = data.time,
				metadata = {
					service = data.service,
					count = data.count
				}
			}
		end
	`

	proc, err := processor.NewLuaProcessor("bench-json", script)
	if err != nil {
		b.Fatal(err)
	}

	rec := sampleRecord(`{"level":"info","msg":"request handled","time":1760000000000,"service":"api","count":42}`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := proc.Process(rec)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLuaProcessor_PatternMatching(b *testing.B) {
	script := `
		DATETIME_FORMAT = "02/Jan/2006:15:04:05 -0700"

		function process(raw, collector, node_id, timestamp)
			local ip, ts, method, path, status = string.match(
				raw,
				'^(%S+) %S+ %S+ %[(.-)%] "(%S+) (%S+) %S+" (%d+)'
			)
			return {
				level = "info",
				message = method .. " " .. path,
				timestamp = ts,
				metadata = {
					client_ip = ip,
					status = tonumber(status)
				}
			}
		end
	`

	proc, err := processor.NewLuaProcessor("bench-pattern", script)
	if err != nil {
		b.Fatal(err)
	}

	rawLine := `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326`
	rec := sampleRecord(rawLine)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := proc.Process(rec)
		if err != nil {
			b.Fatal(err)
		}
	}
}
