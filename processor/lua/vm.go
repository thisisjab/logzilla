package lua

import (
	"fmt"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
	luajson "layeh.com/gopher-json"
)

const (
	// DefaultDatetimeFormat is used when the script does not specify DATETIME_FORMAT.
	DefaultDatetimeFormat = time.RFC3339

	// GlobalDatetimeFormat is the global constant name in Lua scripts defining the timestamp format.
	GlobalDatetimeFormat = "DATETIME_FORMAT"
)

// Compile parses and compiles Lua source code into a reusable FunctionProto
// and extracts the top-level DATETIME_FORMAT global constant if defined.
func Compile(source string) (*lua.FunctionProto, string, error) {
	// Parse into an AST chunk and compile to bytecode upfront so pooled
	// VMs can instantiate FunctionProto directly without re-parsing source text.
	chunk, err := parse.Parse(strings.NewReader(source), "<script>")
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse lua script: %w", err)
	}

	proto, err := lua.Compile(chunk, "<script>")
	if err != nil {
		return nil, "", fmt.Errorf("failed to compile lua script: %w", err)
	}

	// Spin up an ephemeral sandboxed state to validate execution and extract DATETIME_FORMAT.
	tmpState := newSandboxedState()
	defer tmpState.Close()

	// Execute top-level chunk once to validate runtime syntax and populate script globals.
	fn := tmpState.NewFunctionFromProto(proto)
	tmpState.Push(fn)
	if err := tmpState.PCall(0, 0, nil); err != nil { // 0, 0, nil means no arguments, no return values, no error handling
		return nil, "", fmt.Errorf("failed to initialize lua script globals: %w", err)
	}

	// Read optional user-defined timestamp layout, falling back to RFC3339 default.
	datetimeFormat := DefaultDatetimeFormat
	if formatVal := tmpState.GetGlobal(GlobalDatetimeFormat); formatVal.Type() == lua.LTString {
		if s := formatVal.String(); s != "" {
			datetimeFormat = s
		}
	}

	return proto, datetimeFormat, nil
}

// Pool manages a concurrent pool of sandboxed Lua virtual machines initialized with a compiled script.
type Pool struct {
	proto          *lua.FunctionProto
	datetimeFormat string
	pool           sync.Pool
}

// NewPool initializes a VM pool for the given script prototype and timestamp format.
func NewPool(proto *lua.FunctionProto, datetimeFormat string) *Pool {
	if datetimeFormat == "" {
		datetimeFormat = DefaultDatetimeFormat
	}

	p := &Pool{
		proto:          proto,
		datetimeFormat: datetimeFormat,
	}

	p.pool.New = func() any {
		L := newSandboxedState()
		fn := L.NewFunctionFromProto(proto)
		L.Push(fn)
		if err := L.PCall(0, 0, nil); err != nil {
			L.Close()
			panic(fmt.Sprintf("failed to initialize pooled lua state: %v", err))
		}
		return L
	}

	return p
}

// Get retrieves a sandboxed Lua state from the pool.
func (p *Pool) Get() *lua.LState {
	return p.pool.Get().(*lua.LState)
}

// Put cleans the stack and returns the Lua state to the pool.
func (p *Pool) Put(L *lua.LState) {
	if L == nil {
		return
	}
	L.SetTop(0)
	p.pool.Put(L)
}

// DatetimeFormat returns the resolved timestamp layout string for this script.
func (p *Pool) DatetimeFormat() string {
	return p.datetimeFormat
}

// newSandboxedState creates a new LState with restricted capabilities (no os, io, debug).
func newSandboxedState() *lua.LState {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})

	// Open only safe standard libraries
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}

	// Remove filesystem execution primitives from base library
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)

	// Preload json module: local json = require("json")
	luajson.Preload(L)

	return L
}
