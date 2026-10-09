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

The aggregator reads network listening settings and storage locations from `aggregator.yaml`.

```yaml
# gRPC Network Settings
grpc:
  port: 50051                    # Port for the AppendWAL gRPC listener

# Storage Settings
wal:
  dir: "./data/aggregator/wal"   # Directory where received WAL segments are stored
```

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
