package processor

import (
	"fmt"
	"sync"
	"time"

	"github.com/thisisjab/logzilla/entity"
	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"
)

type LuaLogProcessorConfig struct {
	Name       string `yaml:"-"`
	ScriptPath string `yaml:"script-path"`
}

// LuaLogProcessor is a log processor that parses logs based on the provided lua script.
// Provided script MUST contain a function named `parse_log` which takes a string as parameter.
// `parse_log` function must return 4 fields:
// 1. level as a string of debug, info, warning, error, fatal or unknown
// 2. message as a string
// 3. timestamp as a string in ISO 8601/RFC3339 format
// 4. metadata as a table
// Note that user can have access to JSON helper using `local json = require("json")`
type LuaLogProcessor struct {
	cfg  LuaLogProcessorConfig
	pool *sync.Pool
}

func NewLuaLogProcessor(cfg LuaLogProcessorConfig) (*LuaLogProcessor, error) {
	pool := &sync.Pool{
		New: func() any {
			L := lua.NewState(lua.Options{
				SkipOpenLibs: true, // Don't load anything by default
			})

			// Manually open only the safe libraries
			// We skip 'os' and 'io' to prevent system commands/file access
			for _, lib := range []struct {
				name string
				fn   lua.LGFunction
			}{
				{lua.LoadLibName, lua.OpenPackage},  // Allows 'require'
				{lua.BaseLibName, lua.OpenBase},     // Allows 'print', 'pairs', etc.
				{lua.TabLibName, lua.OpenTable},     // Allows 'table.insert', etc.
				{lua.StringLibName, lua.OpenString}, // Allows string manipulation
			} {
				L.Push(L.NewFunction(lib.fn))
				L.Push(lua.LString(lib.name))
				L.Call(1, 0)
			}

			// Pre-register the JSON module in this VM
			// This allows the user to do: local json = require("json")
			luajson.Preload(L)

			// Load the user's script once per VM in the pool
			if err := L.DoFile(cfg.ScriptPath); err != nil {
				panic(err)
			}

			return L
		},
	}

	return &LuaLogProcessor{
		cfg:  cfg,
		pool: pool,
	}, nil
}

func (lp *LuaLogProcessor) Name() string {
	return lp.cfg.Name
}

func (lp *LuaLogProcessor) Process(record entity.LogRecord) (entity.LogRecord, error) {
	L := lp.pool.Get().(*lua.LState)
	defer lp.pool.Put(L)

	// Call the "parse_log" function defined in Lua
	err := L.CallByParam(lua.P{
		Fn:      L.GetGlobal("parse_log"),
		NRet:    4,
		Protect: true,
	}, lua.LString(string(record.RawData)))

	if err != nil {
		return record, fmt.Errorf("lua script error: %w", err)
	}

	// Extract values
	luaMeta := L.ToTable(-1)
	tsRaw := L.ToString(-2)
	luaMessage := L.ToString(-3)
	luaLevel := L.ToString(-4)

	// Clean up stack IMMEDIATELY after extraction
	L.Pop(4)

	// Parsing outside of the Lua VM Lock
	luaTimestamp, err := time.Parse(time.RFC3339, tsRaw)
	if err != nil {
		return record, fmt.Errorf("cannot parse timestamp '%s': %w", tsRaw, err)
	}

	return entity.LogRecord{
		Timestamp: luaTimestamp,
		Level:     parseLevel(luaLevel),
		Message:   luaMessage,
		Metadata:  luaTableToMap(luaMeta),
	}, nil
}

func luaTableToMap(table *lua.LTable) map[string]any {
	res := make(map[string]any)
	table.ForEach(func(key, value lua.LValue) {
		// Lua keys are usually strings for metadata, but we ensure string conversion for the map key
		res[key.String()] = convertLuaValue(value)
	})
	return res
}

func convertLuaValue(value lua.LValue) any {
	switch v := value.(type) {
	case *lua.LTable:
		// Check if it's an array (has sequential integer keys starting at 1)
		// Or just treat everything as a map for consistency in log metadata
		return luaTableToMap(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case lua.LBool:
		return bool(v)
	case *lua.LNilType:
		return nil
	default:
		if value == lua.LNil {
			return nil
		}

		// Fallback for types we don't explicitly handle (like functions or userdata)
		return v.String()
	}
}
