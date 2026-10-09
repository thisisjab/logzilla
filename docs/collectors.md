# Collectors Subsystem

The Collectors subsystem provides a modular abstraction for reading log data from diverse sources (local files, standard streams, sockets, journald) and emitting them to the Ingestor's WAL pipeline.

---

## 1. The `Collector` Interface

Every log source must satisfy the `Collector` interface in `collector/interface.go`:

```go
package collector

import "context"

type Collector interface {
    Collect(ctx context.Context, cb func(string) error) error
}
```

- `ctx`: A context tied to the collector's lifecycle (used for graceful cancellation during hot reloads or ingestor shutdown).
- `cb`: A callback function provided by the Ingestor. For each line or record collected, calling `cb(data)` appends the record into the active WAL file.

---

## 2. File Collector Implementation

The File Collector (`collector/file.go`) tails a local file continuously from its existing end:

### Key Characteristics
1. **Seek to End on Startup**: When started, it seeks to the end of the file (`io.SeekEnd`) so historical log lines are not duplicated unless explicitly requested.
2. **Chunked Reading & Line Splitting**:
   - Reads bytes in 4KB chunks (`buf := make([]byte, 4096)`).
   - Scans for newline characters (`\n`).
   - Maintains an in-memory buffer for incomplete lines spanning multiple read chunks.
3. **Non-Blocking Sleep on EOF**:
   - When reaching EOF, it sleeps for a short poll duration (`100ms`) before checking for newly appended bytes.
4. **Context-Aware Exit**:
   - Checks `ctx.Done()` on every iteration so hot-reload stops take effect immediately.

---

## 3. Dynamic Factory / Builder

The `collector/builder.go` factory instantiates collectors based on configuration types:

```go
func New(collectorType string, args map[string]interface{}) (Collector, error) {
    switch collectorType {
    case "file":
        path, ok := args["path"].(string)
        if !ok || path == "" {
            return nil, errors.New("file collector requires 'path' argument")
        }
        return NewFileCollector(path), nil
    default:
        return nil, fmt.Errorf("unknown collector type: %s", collectorType)
    }
}
```

---

## 4. Extending Logzilla with New Collectors

To add a new collector (e.g. Syslog UDP listener):

1. Create a struct implementing `collector.Collector`:
   ```go
   type SyslogCollector struct {
       addr string
   }

   func (s *SyslogCollector) Collect(ctx context.Context, cb func(string) error) error {
       // listen on UDP socket, call cb(msg) on arrival, exit on ctx.Done()
   }
   ```
2. Register the type in `collector/builder.go`:
   ```go
   case "syslog":
       addr := args["addr"].(string)
       return NewSyslogCollector(addr), nil
   ```
3. Configure it in `collectors.yaml`:
   ```yaml
   collectors:
     syslog-listener:
       type: syslog
       args:
         addr: "0.0.0.0:514"
   ```
