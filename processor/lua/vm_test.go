package lua_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gopherlua "github.com/yuin/gopher-lua"

	"github.com/thisisjab/logzilla/processor/lua"
)

func TestSandboxIsolation(t *testing.T) {
	tests := []struct {
		name       string
		script     string
		shouldFail bool
		assertFn   func(t *testing.T, L *gopherlua.LState, err error)
	}{
		{
			name: "forbidden_globals_are_nil",
			script: `
				assert(os == nil, "os should be nil")
				assert(io == nil, "io should be nil")
				assert(debug == nil, "debug should be nil")
				assert(dofile == nil, "dofile should be nil")
				assert(loadfile == nil, "loadfile should be nil")
			`,
			shouldFail: false,
		},
		{
			name: "require_os_fails",
			script: `
				local os = require("os")
			`,
			shouldFail: true,
		},
		{
			name: "require_io_fails",
			script: `
				local io = require("io")
			`,
			shouldFail: true,
		},
		{
			name: "allowed_globals_are_present",
			script: `
				assert(type(string) == "table", "string should exist")
				assert(type(table) == "table", "table should exist")
				assert(type(math) == "table", "math should exist")
				assert(type(pairs) == "function", "pairs should exist")
				assert(type(ipairs) == "function", "ipairs should exist")
			`,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto, datetimeFormat, err := lua.Compile(tt.script)
			if tt.shouldFail {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, time.RFC3339, datetimeFormat)

			pool := lua.NewPool(proto, datetimeFormat)
			L := pool.Get()
			defer pool.Put(L)
			require.NotNil(t, L)
		})
	}
}

func TestJSONModule(t *testing.T) {
	script := `
		local json = require("json")
		local payload = '{"service":"api","status":200,"active":true,"tags":["go","lua"]}'
		local decoded = json.decode(payload)

		assert(decoded.service == "api", "service mismatch")
		assert(decoded.status == 200, "status mismatch")
		assert(decoded.active == true, "active mismatch")
		assert(decoded.tags[1] == "go", "tags[1] mismatch")
		assert(decoded.tags[2] == "lua", "tags[2] mismatch")

		local encoded = json.encode({ status = "ok" })
		assert(encoded == '{"status":"ok"}', "json.encode mismatch: " .. encoded)
	`

	proto, _, err := lua.Compile(script)
	require.NoError(t, err)

	pool := lua.NewPool(proto, "")
	L := pool.Get()
	defer pool.Put(L)
}

func TestScriptGlobalDatetimeFormat(t *testing.T) {
	tests := []struct {
		name           string
		script         string
		expectedLayout string
	}{
		{
			name: "custom_datetime_format_defined",
			script: `
				DATETIME_FORMAT = "2006-01-02 15:04:05"
				function process() end
			`,
			expectedLayout: "2006-01-02 15:04:05",
		},
		{
			name: "custom_apache_datetime_format",
			script: `
				DATETIME_FORMAT = "02/Jan/2006:15:04:05 -0700"
			`,
			expectedLayout: "02/Jan/2006:15:04:05 -0700",
		},
		{
			name: "empty_datetime_format_falls_back_to_default",
			script: `
				DATETIME_FORMAT = ""
			`,
			expectedLayout: time.RFC3339,
		},
		{
			name: "omitted_datetime_format_defaults_to_rfc3339",
			script: `
				function process() end
			`,
			expectedLayout: time.RFC3339,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto, datetimeFormat, err := lua.Compile(tt.script)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedLayout, datetimeFormat)

			pool := lua.NewPool(proto, datetimeFormat)
			assert.Equal(t, tt.expectedLayout, pool.DatetimeFormat())
		})
	}
}

func TestLuaPatternMatching(t *testing.T) {
	script := `
		local log = '127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326'
		local ip, user, ts, method, path, status = string.match(
			log,
			'^(%S+) %S+ (%S+) %[(.-)%] "(%S+) (%S+) HTTP/%S+" (%d+)'
		)

		assert(ip == "127.0.0.1", "ip mismatch: " .. tostring(ip))
		assert(user == "frank", "user mismatch: " .. tostring(user))
		assert(ts == "10/Oct/2000:13:55:36 -0700", "ts mismatch: " .. tostring(ts))
		assert(method == "GET", "method mismatch: " .. tostring(method))
		assert(path == "/apache_pb.gif", "path mismatch: " .. tostring(path))
		assert(status == "200", "status mismatch: " .. tostring(status))
	`

	proto, _, err := lua.Compile(script)
	require.NoError(t, err)

	pool := lua.NewPool(proto, "")
	L := pool.Get()
	defer pool.Put(L)
}

func TestPoolConcurrency(t *testing.T) {
	script := `
		function add(a, b)
			return a + b
		end
	`

	proto, _, err := lua.Compile(script)
	require.NoError(t, err)

	pool := lua.NewPool(proto, time.RFC3339)

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				L := pool.Get()

				err := L.CallByParam(gopherlua.P{
					Fn:      L.GetGlobal("add"),
					NRet:    1,
					Protect: true,
				}, gopherlua.LNumber(gID), gopherlua.LNumber(i))

				assert.NoError(t, err)
				res := L.ToInt(-1)
				assert.Equal(t, gID+i, res)

				pool.Put(L)
			}
		}(g)
	}

	wg.Wait()
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "syntax_error",
			source: "function invalid( return end",
		},
		{
			name:   "runtime_error_at_top_level",
			source: "error('fatal init failure')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto, _, err := lua.Compile(tt.source)
			require.Error(t, err)
			assert.Nil(t, proto)
		})
	}
}
