# Logzilla

A high-performance, distributed log ingestion and storage engine written in Go.

Logzilla reliably collects logs across distributed nodes, persists them in binary Write-Ahead Logs (WALs), ships segments to central aggregators via gRPC, and stores them in an LSM-tree storage architecture (in-memory SkipList MemTable and on-disk SSTables).

---

## Architecture Overview

At a high level, Logzilla splits responsibilities across two node types:

- **Ingestor Node**: Runs at the edge, tails logs from collectors (files, streams), appends records to a binary WAL with periodic `fsync` and size-based rotation, and pushes closed segments to the cluster.
- **Aggregator Node**: Receives WAL segments over gRPC, atomically persists them to disk, unpacks records via a streaming decoder, and routes them through Lua pre-processors into an in-memory MemTable (SkipList) and SSTables.

For the complete architectural design, data pipelines, and distributed consensus details, see [**docs/architecture.md**](docs/architecture.md).

---

## Documentation

Full technical documentation is located in the [`docs/`](docs/) directory:

- [**System Architecture**](docs/architecture.md): Distributed topology, end-to-end data lifecycle, and consensus failover.
- [**Ingestor Subsystem**](docs/ingestor.md): Collector management, dynamic hot-reloading, and background pusher engine.
- [**WAL Binary Format**](docs/wal-format.md): Binary wire layout, rotation, sync loops, and streaming decoder.
- [**Aggregator Service**](docs/aggregator.md): gRPC `AppendWAL` service, atomic `.tmp` persistence, and leader redirection.
- [**Collectors**](docs/collectors.md): Collector interface, file tailing, and guide for adding custom collectors.
- [**Data Structures**](docs/data-structures.md): Concurrent SkipList design and MemTable indexing.
- [**SSTable Format v1**](docs/sstable-format.md): On-disk LSM SSTable layout, indexing, and 1024-byte footer specification.
- [**Configuration Guide**](docs/configuration.md): Complete reference for YAML files, CLI flags, and environment variables.

---

## Setup & Quick Start

### Prerequisites

- **Go**: 1.24 or higher
- **Protocol Buffers & Buf** (optional, only needed when modifying `.proto` files):
  - `buf` CLI

### Running the Services

1. **Start the Aggregator**:
   ```bash
   go run ./cmd/aggregator -config aggregator.yaml
   ```
   Default `aggregator.yaml`:
   ```yaml
   grpc:
     port: 50051

   wal:
     dir: "./data/aggregator/wal"
   ```

2. **Start the Ingestor**:
   ```bash
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

---

## Development Environment Setup

### 1. Clone & Dependencies

```bash
git clone https://github.com/thisisjab/logzilla.git
cd logzilla
go mod download
```

### 2. Verification Commands

Run the standard validation toolchain before submitting any change:

```bash
# Verify formatting
gofmt -l .

# Run static analysis
go vet ./...

# Run automated tests
go test ./...

# Build all binaries
go build ./...
```

---

## Contributing

We welcome contributions! Please adhere to the project standards when submitting pull requests:

### Code Style & Standards
- **Idiomatic Go**: Follow Effective Go and Go Code Review Comments.
- **Clean Toolchain**: Code must be `gofmt`/`goimports` clean and pass `go vet ./...`.
- **Standard Library First**: Stick to the standard library whenever possible. Avoid introducing external dependencies without prior discussion.
- **Error Handling**: Handle every error explicitly. Wrap errors with context using `fmt.Errorf("...: %w", err)`.
- **No Magic Constants**: Use clearly named constants instead of hardcoded numbers or strings.
- **Concurrency Safety**: Goroutines must have a clear owner, clean context cancellation, and must never leak.

### Testing Requirements
- Every new feature or bug fix must include automated unit/integration tests (prefer table-driven tests with subtests `t.Run`).
- Tests must pass before considering any work complete:
  ```bash
  go test -v ./...
  ```

### Git Commit Guidelines
- Use [Conventional Commits](https://www.conventionalcommits.org/):
  - Format: `type(scope): description`
  - Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`, `build`, `ci`, `style`.
  - Mark breaking changes with `!` or a `BREAKING CHANGE:` footer.
- Keep commits small, atomic, and focused on a single logical change.

### Definition of Done
A contribution is ready for merge only when:
1. `gofmt -l .` prints nothing.
2. `go vet ./...` passes cleanly with zero warnings.
3. `go test ./...` passes without errors.
4. Documentation in `docs/` and `README.md` is updated to reflect all affected behaviors, flags, and architecture.

---

## License

MIT
