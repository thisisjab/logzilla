# Lua Processing Engine & Sandboxing

Logzilla processes incoming raw log streams using sandboxed Lua scripts. Scripts parse, normalize, and enrich log entries before they are inserted into the MemTable.

---

## 1. Sandboxing & Security Boundaries

To ensure safe multi-tenant or untrusted script execution, the Lua VM environment is strictly sandboxed:

- **Disabled Packages & Primitives**:
  - `os`: No shell execution, environment access, or process manipulation.
  - `io`: No file system access or direct I/O streams.
  - `debug`: No introspection or stack manipulation.
  - `channel` & `coroutine`: Goroutine spawning and async Lua primitives are omitted.
  - `dofile` & `loadfile`: Blocked in the global scope to prevent loading unvetted code from disk.
- **Allowed Standard Libraries**:
  - `package`: Module resolution for preloaded libraries (`json`).
  - `base`: Core built-in functions (`pairs`, `ipairs`, `tostring`, `tonumber`, `type`, `assert`, `error`).
  - `table`: Table manipulation (`table.insert`, `table.concat`, `table.sort`, etc.).
  - `string`: String manipulation and native Lua pattern matching (`string.match`, `string.find`, `string.gsub`).
  - `math`: Standard mathematical functions.

---

## 2. Utility Modules

### JSON Module
Scripts can import the built-in JSON parser using `require("json")`:

```lua
local json = require("json")

-- Decode JSON string into a Lua table
local data = json.decode(raw_json_string)

-- Encode Lua table into a JSON string
local json_str = json.encode(my_table)
```

### Pattern Matching
Lua's native pattern matching is available on string instances and the `string` module:

```lua
local ip, path = string.match(raw, '^(%S+) %S+ %S+ %[.-%] "%S+ (%S+)')
```

---

## 3. Timestamp Layout (`DATETIME_FORMAT`)

To eliminate runtime format guessing overhead, scripts declare an optional global `DATETIME_FORMAT` constant using Go's reference time layout (or standard constants):

```lua
-- Specify custom timestamp layout (e.g. Apache Common Log format)
DATETIME_FORMAT = "02/Jan/2006:15:04:05 -0700"

-- If omitted, Logzilla defaults to time.RFC3339 ("2006-01-02T15:04:05Z07:00")
```

When the processor executes, it uses the script's `DATETIME_FORMAT` layout to deterministically parse string timestamps returned by the script into Unix millisecond timestamps.

---

## 4. Script Contract

Every processor script defines a global `process(raw, collector, node_id, timestamp)` function:

### Parameters
- `raw` (`string`): The raw unparsed log payload.
- `collector` (`string`): Name of the collector that ingested the log.
- `node_id` (`string`): UUID of the ingestor node.
- `timestamp` (`number`): Unix arrival timestamp in milliseconds.

### Return Values
- **Table**: A structured table containing:
  - `level` (`string`): Severity (`"debug"`, `"info"`, `"warn"`, `"error"`, `"fatal"`).
  - `message` (`string`): Parsed main message string.
  - `timestamp` (`string` or `number`, optional): Extracted event timestamp. If a string is returned, it is parsed via `DATETIME_FORMAT`. If omitted, the incoming WAL timestamp is preserved.
  - `metadata` (`table`, optional): Key-value dictionary of structured attributes.
- **`nil`**: Drops the record (filtering out unwanted logs).

---

## 5. Examples

### JSON Application Logs
```lua
local json = require("json")

function process(raw, collector, node_id, timestamp)
    local ok, data = pcall(json.decode, raw)
    if not ok or data == nil then
        return {
            level = "error",
            message = raw,
            metadata = { error = "invalid_json" }
        }
    end

    -- Drop health checks
    if data.path == "/health" or data.path == "/ready" then
        return nil
    end

    return {
        level = data.level or "info",
        message = data.message or "",
        timestamp = data.timestamp, -- Parsed via RFC3339 by default
        metadata = data.meta or {}
    }
end
```

### Nginx / Apache Access Logs
```lua
DATETIME_FORMAT = "02/Jan/2006:15:04:05 -0700"

function process(raw, collector, node_id, timestamp)
    local ip, user, ts, method, path, status, bytes = string.match(
        raw,
        '^(%S+) %S+ (%S+) %[(.-)%] "(%S+) (%S+) %S+" (%d+) (%d+)'
    )

    if not ip then
        return {
            level = "warn",
            message = raw,
            metadata = { parse_error = true }
        }
    end

    local statusCode = tonumber(status)
    local level = "info"
    if statusCode >= 500 then
        level = "error"
    elseif statusCode >= 400 then
        level = "warn"
    end

    return {
        level = level,
        message = method .. " " .. path .. " (" .. status .. ")",
        timestamp = ts,
        metadata = {
            client_ip = ip,
            user = user,
            status = statusCode,
            bytes_sent = tonumber(bytes)
        }
    }
end
```
