# Logzilla

A high-performance, distributed log ingestion and storage engine written in Go.

Logzilla is designed to reliably collect logs across nodes, persist them in binary Write-Ahead Logs (WALs), ship segments to cluster aggregators via gRPC, and process them through a pipeline using an in-memory MemTable (SkipList) and SSTables.

---

## Architecture Overview

```
+-------------------------------------------------------------------------+
|                              INGESTOR NODE                              |
|                                                                         |
|  +--------------------+    +------------------+    +-----------------+  |
|  | File Collector(s)  | -> | Binary WAL Engine| -> |  Pusher Engine  |  |
|  | (live log tailing) |    | (.wal segments)  |    | (gRPC shipping) |  |
|  +--------------------+    +------------------+    +--------+--------+  |
+-------------------------------------------------------------|-----------+
                                                              |
                                                    gRPC AppendWAL Stream
                                                              |
                                                              v
+-------------------------------------------------------------------------+
|                            AGGREGATOR NODE                              |
|                                                                         |
|  +--------------------+    +------------------+    +-----------------+  |
|  |   gRPC Handler     | -> | Atomic WAL Store | -> |  WAL Unpacker / |  |
|  | (leader redirect)  |    | (.tmp -> rename) |    | Streaming Dec.  |  |
|  +--------------------+    +------------------+    +--------+--------+  |
|                                                             |           |
|                                                             v           |
|                                                    +-----------------+  |
|                                                    |  MemTable (DSA) |  |
|                                                    |   (SkipList)    |  |
|                                                    +-----------------+  |
+-------------------------------------------------------------------------+
```

---

## Core Features

- **Ingestor Node**:
  - Hot-reloading YAML configuration via `fsnotify` (add/remove log collectors without restarts).
  - High-throughput file tailing with byte offset tracking.
  - Persistent Node UUID identity (`.ingestor.uuid`).
  - Background WAL pusher with dynamic gRPC leader redirection (`leader_ip` trailer).
- **Binary Write-Ahead Log (WAL)**:
  - Custom binary wire format packing Ingestor UUID, Unix timestamp, ULID, collector name, and raw payload.
  - Periodic background sync (`wal.sync_interval`) and size-based rotation (`wal.max_bytes`).
  - Zero-allocation streaming binary unpacker with comprehensive bounds-checking.
- **Aggregator Service**:
  - gRPC `AppendWAL` service with atomic file persistence (`.tmp` write followed by atomic rename).
  - Idempotent deduplication for safe retries and Best-Effort delivery.
- **In-Memory MemTable**:
  - Concurrent, probabilistic SkipList index for ordered in-memory log ingestion.

---

## Quick Start

### Prerequisites

- Go 1.24+ (uses Go standard library `uuid`)
- Protocol Buffers compiler / `buf` (optional, for regenerating proto files)

### 1. Build and Run the Aggregator

The aggregator receives WAL files from ingestors over gRPC.

```bash
# Start Aggregator on port 50051
go run ./cmd/aggregator -config aggregator.yaml
```

Default `aggregator.yaml`:
```yaml
grpc:
  port: 50051

wal:
  dir: "./data/aggregator/wal"
```

### 2. Build and Run the Ingestor

The ingestor tails configured logs and ships rotated WAL segments to the aggregator.

```bash
# Start Ingestor
go run ./cmd/ingestor -collectors-path collectors.yaml
```

Default `collectors.yaml`:
```yaml
collectors:
  app-logs:
    type: file
    args:
      path: /var/log/app.log

wal:
  dir: "./data/wal"
  max_bytes: 10485760      # 10 MB
  sync_interval: 100ms

push:
  server: "127.0.0.1:50051"
  interval: 10s
```

### 3. Run Tests

```bash
go test -v ./...
```

---

## Documentation

Comprehensive documentation for all subsystems is available in the [`docs/`](docs/) directory:

- [**System Architecture**](docs/architecture.md): Distributed data flow, ingestion & aggregation pipeline, and consensus roadmap.
- [**Ingestor Subsystem**](docs/ingestor.md): Collector management, hot reloading, persistent identity, and the Pusher engine.
- [**WAL Binary Format**](docs/wal-format.md): Binary layout, rotation, sync loops, and the streaming unpacker.
- [**Aggregator Service**](docs/aggregator.md): gRPC endpoint, atomic `.tmp` persistence, and leader redirection.
- [**Collectors**](docs/collectors.md): Collector interface, file tailing, and extensibility guide.
- [**Data Structures**](docs/data-structures.md): Concurrent SkipList design and MemTable foundation.
- [**SSTable Format v1**](docs/sstable-format.md): On-disk LSM SSTable layout, data records, indexing, and 1024-byte footer specification.
- [**Configuration Guide**](docs/configuration.md): Complete reference for YAML configurations and CLI flags.

---

## License

MIT
