# Configuration Guide

Logzilla components are configured using standard YAML files, environment variables, or CLI flags.

---

## 1. Ingestor Configuration (`collectors.yaml`)

The ingestor reads source collector definitions, local WAL persistence settings, and push targets from `collectors.yaml`.

```yaml
# Source Collectors
collectors:
  app-service:
    type: file
    args:
      path: /var/log/app.log
  nginx-access:
    type: file
    args:
      path: /var/log/nginx/access.log

# Write-Ahead Log (WAL) Settings
wal:
  dir: "./data/wal"              # Directory where .wal files are written
  max_bytes: 10485760            # Maximum active WAL size before rotation (10 MB)
  sync_interval: 100ms           # Background fsync and rotation check interval

# Pusher Engine Settings
push:
  server: "127.0.0.1:50051"      # Central aggregator gRPC address
  interval: 10s                  # Frequency of background push sweeps
```

### Ingestor CLI Flags & Environment Variables

| Flag | Env Variable | Default | Description |
|---|---|---|---|
| `-collectors-path` | `COLLECTORS_PATH` | `collectors.yaml` | Path to the collectors YAML configuration |
| `-uuid-path` | - | `.ingestor.uuid` | Path to persistent Ingestor UUID file |
| - | `LOG_LEVEL` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`) |

---

## 2. Aggregator Configuration (`aggregator.yaml`)

The aggregator reads network listening settings, storage locations, and pre-processing pipeline configurations from `aggregator.yaml`.

```yaml
# gRPC Network Settings
grpc:
  port: 50051                    # Port for the AppendWAL gRPC listener

# Storage Settings
wal:
  dir: "./data/aggregator/wal"   # Directory where received WAL segments are stored

# Pre-Processing Pipeline
processors:
  - name: capitalizor
    script: |
      function process(raw, collector, node_id, timestamp)
          return { level = "info", message = string.upper(raw) }
      end
    collectors:
      - "postgres-db"
      - "server-*-logs"

  - name: json-parser
    script_path: "/etc/logzilla/processors/json.lua"
    collectors: "*.json"

  - name: fallback
    script_path: "/etc/logzilla/processors/fallback.lua"
    collectors: "*"
```

### Processor Collector Matching Rules
- Each incoming log record is routed to at most one processor based on its collector name.
- **Precedence Order**:
  1. Exact match (e.g. `postgres-db`, `journalctl`)
  2. Partial glob match evaluated in configuration order (first match wins). Supported syntax via Go standard [`filepath.Match`](https://pkg.go.dev/path/filepath#Match):
     - `*`: matches any sequence of characters (e.g. `server-*-logs`, `*.json`)
     - `?`: matches any single character (e.g. `node-?`)
     - `[...]`: matches character ranges or sets (e.g. `app-[0-9]`, `service-[a-c]`)
  3. Catch-all wildcard (`*`)
- If a collector matches no configured processor, its records are safely ignored without error.

### Aggregator CLI Flags & Environment Variables

| Flag | Env Variable | Default | Description |
|---|---|---|---|
| `-config` | `AGGREGATOR_CONFIG` | `aggregator.yaml` | Path to the aggregator YAML configuration |
| `-port` | - | `50051` | gRPC server listening port override |
| `-wal-dir` | - | `./data/aggregator/wal` | WAL storage directory path override |
| - | `LOG_LEVEL` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`) |

---

## 3. Hot-Reloading Behavior

- **Ingestor**:
  - Automatically watches `collectors.yaml` using `fsnotify`.
  - Adding, removing, or modifying collectors in `collectors.yaml` updates live collectors without killing the process or missing logs.
  - Updates to `wal` or `push` parameters in `collectors.yaml` take effect dynamically on reload.
- **Aggregator**:
  - Modifying storage directory or listening ports requires restarting the service to rebind network interfaces.
