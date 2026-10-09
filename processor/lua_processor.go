package processor

import (
	"fmt"
	"os"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/thisisjab/logzilla/entity"
	luavm "github.com/thisisjab/logzilla/processor/lua"
	"github.com/thisisjab/logzilla/wal"
)

const (
	luaProcessFn = "process"
	fieldLevel   = "level"
	fieldMessage = "message"
	fieldTime    = "timestamp"
	fieldMeta    = "metadata"
)

// LuaProcessor processes raw WAL records using sandboxed Lua scripts.
type LuaProcessor struct {
	name string
	pool *luavm.Pool
}

// NewLuaProcessor compiles the Lua source code and initializes a VM pool.
func NewLuaProcessor(name string, scriptSource string) (*LuaProcessor, error) {
	if name == "" {
		return nil, fmt.Errorf("processor name cannot be empty")
	}

	proto, datetimeFormat, err := luavm.Compile(scriptSource)
	if err != nil {
		return nil, fmt.Errorf("failed to compile processor %q: %w", name, err)
	}

	pool := luavm.NewPool(proto, datetimeFormat)

	// Validate that the global process function is registered in the VM.
	state := pool.Get()
	defer pool.Put(state)

	if state.GetGlobal(luaProcessFn).Type() != lua.LTFunction {
		return nil, fmt.Errorf("processor %q script must define a global %q function", name, luaProcessFn)
	}

	return &LuaProcessor{
		name: name,
		pool: pool,
	}, nil
}

// NewLuaProcessorFromFile reads a script file from disk and initializes a LuaProcessor.
func NewLuaProcessorFromFile(name string, scriptPath string) (*LuaProcessor, error) {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read lua script %q: %w", scriptPath, err)
	}

	return NewLuaProcessor(name, string(data))
}

// Name returns the identifier of this processor.
func (p *LuaProcessor) Name() string {
	return p.name
}

// Process executes the script against an unpacked WAL record.
// Returns (nil, nil) if the script returns nil, filtering out the log.
// On script runtime error or datetime parse failure, returns a fallback record with Err() set.
func (p *LuaProcessor) Process(record wal.UnpackedRecord) (*entity.LogRecord, error) {
	state := p.pool.Get()
	defer p.pool.Put(state)

	err := state.CallByParam(lua.P{
		Fn:      state.GetGlobal(luaProcessFn),
		NRet:    1,
		Protect: true,
	},
		lua.LString(record.Raw),
		lua.LString(record.CollectorName),
		lua.LString(record.NodeID.String()),
		lua.LNumber(record.Timestamp),
	)

	if err != nil {
		fallback := entity.New(
			record.ID,
			record.NodeID,
			"",
			record.CollectorName,
			record.Timestamp,
			entity.LevelUnknown,
			"",
			nil,
			record.Raw,
		).WithError(fmt.Errorf("lua runtime error: %w", err))

		return &fallback, nil
	}

	retVal := state.Get(-1)

	// Returning nil drops the record from the pipeline.
	if retVal == lua.LNil {
		return nil, nil
	}

	tbl, ok := retVal.(*lua.LTable)
	if !ok {
		fallback := entity.New(
			record.ID,
			record.NodeID,
			"",
			record.CollectorName,
			record.Timestamp,
			entity.LevelUnknown,
			"",
			nil,
			record.Raw,
		).WithError(fmt.Errorf("expected process() to return table or nil, got %s", retVal.Type().String()))

		return &fallback, nil
	}

	level := entity.LevelInfo
	if lvlVal := tbl.RawGetString(fieldLevel); lvlVal.Type() == lua.LTString {
		level = entity.ParseLogLevel(lvlVal.String())
	}

	var message string
	if msgVal := tbl.RawGetString(fieldMessage); msgVal.Type() == lua.LTString {
		message = msgVal.String()
	}

	timestamp := record.Timestamp
	var parseErr error

	switch tsVal := tbl.RawGetString(fieldTime).(type) {
	case lua.LNumber:
		timestamp = int64(tsVal)
	case lua.LString:
		tsStr := tsVal.String()
		parsedTime, err := time.Parse(p.pool.DatetimeFormat(), tsStr)
		if err != nil {
			timestamp = 0
			parseErr = fmt.Errorf("failed to parse timestamp %q with format %q: %w", tsStr, p.pool.DatetimeFormat(), err)
		} else {
			timestamp = parsedTime.UnixMilli()
		}
	}

	var metadata map[string]any
	if metaVal := tbl.RawGetString(fieldMeta); metaVal.Type() == lua.LTTable {
		metadata = luaTableToMap(metaVal.(*lua.LTable))
	} else {
		metadata = make(map[string]any)
	}

	rec := entity.New(
		record.ID,
		record.NodeID,
		"",
		record.CollectorName,
		timestamp,
		level,
		message,
		metadata,
		record.Raw,
	)

	if parseErr != nil {
		rec.Timestamp = 0
		rec = rec.WithError(parseErr)
	}

	return &rec, nil
}

func luaTableToMap(table *lua.LTable) map[string]any {
	if table == nil {
		return make(map[string]any)
	}

	res := make(map[string]any)
	table.ForEach(func(key, val lua.LValue) {
		res[key.String()] = convertLuaValue(val)
	})
	return res
}

func convertLuaValue(value lua.LValue) any {
	switch v := value.(type) {
	case *lua.LTable:
		if isArray(v) {
			return convertLuaArray(v)
		}
		return luaTableToMap(v)
	case lua.LNumber:
		f := float64(v)
		if f == float64(int64(f)) {
			return int64(f)
		}
		return f
	case lua.LString:
		return string(v)
	case lua.LBool:
		return bool(v)
	default:
		if value == lua.LNil {
			return nil
		}
		return v.String()
	}
}

func isArray(table *lua.LTable) bool {
	if table.RawGetInt(1) == lua.LNil {
		return false
	}

	var hasNonNumberKey bool
	table.ForEach(func(k, _ lua.LValue) {
		if _, ok := k.(lua.LNumber); !ok {
			hasNonNumberKey = true
		}
	})
	return !hasNonNumberKey
}

func convertLuaArray(table *lua.LTable) []any {
	var res []any
	for i := 1; ; i++ {
		val := table.RawGetInt(i)
		if val == lua.LNil {
			break
		}
		res = append(res, convertLuaValue(val))
	}
	return res
}
