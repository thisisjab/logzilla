# Aggregator Service

The Aggregator service receives binary Write-Ahead Log (WAL) segments from Ingestors across the network, persists them to disk atomically, and prepares them for pipeline processing (Lua unpacker, MemTable, SSTables).

---

## 1. gRPC Service Contract

The Aggregator service is defined in `proto/aggregator/v1/aggregator.proto`:

```protobuf
syntax = "proto3";

package aggregator.v1;

option go_package = "github.com/thisisjab/logzilla/gen/aggregator/v1;aggregatorv1";

service Aggregator {
  rpc AppendWAL(AppendWALRequest) returns (AppendWALResponse);
}

message AppendWALRequest {
  int64 wal_id = 1;
  bytes data = 2;
}

message AppendWALResponse {}
```

---

## 2. Atomic Disk Persistence

To ensure zero corrupted or half-written WAL segments in the event of an abrupt process crash or power cut, the Aggregator uses an atomic file swap pattern:

```
                      +-----------------------------+
                      | Incoming AppendWAL Request  |
                      +--------------+--------------+
                                     |
                                     v
                 +---------------------------------------+
                 | Write to <wal_id>.wal.tmp-<pid> file  |
                 +-------------------+-------------------+
                                     |
                                     v
                 +---------------------------------------+
                 | Atomic os.Rename(tmp, <wal_id>.wal)   |
                 +-------------------+-------------------+
                                     |
                                     v
                 +---------------------------------------+
                 | Return AppendWALResponse (Success)    |
                 +---------------------------------------+
```

### Idempotency & Deduplication
- If a client retries a push due to network latency, the Aggregator checks if `<wal_id>.wal` already exists with the same file size.
- If it exists, the request returns success immediately without re-writing to disk.
- If a write error occurs before `os.Rename`, the temporary `.tmp` file is safely unlinked.

---

## 3. Background Pipeline Worker & Processing

The Aggregator runs an asynchronous background worker that discovers closed WAL segments on disk and streams them through sandboxed pre-processors:

```
                      +-----------------------------+
                      | Incoming AppendWAL Request  |
                      +--------------+--------------+
                                     |
                                     v
                  +---------------------------------------+
                  | Write to <wal_id>.wal.tmp-<pid> file  |
                  +-------------------+-------------------+
                                     |
                                     v
                  +---------------------------------------+
                  | Atomic os.Rename(tmp, <wal_id>.wal)   |
                  +-------------------+-------------------+
                                     |
                                     v
                  +---------------------------------------+
                  | Return AppendWALResponse (Success)    |
                  +---------------------------------------+


   [Background Worker: Periodic Ticker (aggregator/worker.go)]
                                 |
                                 v
              +-------------------------------------+
              | Scan walDir for id > lastWalID      |
              +------------------+------------------+
                                 |
                                 v
              +-------------------------------------+
              | Stream records via Iterator         |
              +------------------+------------------+
                                 |
                                 v
              +-------------------------------------+
              | Route to LuaProcessor               |
              | (exact > partial glob > *)          |
              +-------------------------------------+
```

### Worker Behavior
1. **Periodic Discovery**: The worker periodically scans `walDir` (every 500ms), discovering closed files matching `<id>.wal` where `id > lastProcessedWalID`.
2. **Sequential Ordering**: Candidates are sorted ascending by timestamp ID to preserve strict chronological replay.
3. **Collector Routing**: Uses `processor.Router` to resolve the matching processor for `record.CollectorName`:
   - Exact match (e.g. `postgres-db`)
   - Partial glob match evaluated in configuration order (first match wins) supporting Go `filepath.Match` wildcards (`*`, `?`, `[...]`)
   - Catch-all (`*`)
4. **Cursor Advancement**: On completing a segment, the in-memory cursor `lastProcessedWalID` (`atomic.Int64`) is updated to prevent duplicate execution during the service lifecycle.

---

## 4. Server Architecture & Graceful Shutdown

```go
type Aggregator struct {
    aggregatorv1.UnimplementedAggregatorServer

    v          *viper.Viper
    logger     *slog.Logger
    configPath string
    walDir     string
    port       int
    router     *processor.Router

    mu                 sync.Mutex
    grpcServer         *grpc.Server
    listener           net.Listener
    lastProcessedWalID atomic.Int64
    workerWg           sync.WaitGroup
}
```

### Shutdown Handling
- `agg.Serve(ctx)` listens on the configured TCP port (`grpc.port`, default `50051`) and launches the background worker goroutine.
- When the parent `ctx` is canceled (e.g. `SIGINT`/`SIGTERM`):
  - `grpcServer.GracefulStop()` stops accepting new RPCs and lets in-flight writes finish.
  - Background worker context is canceled and `a.workerWg.Wait()` ensures in-flight segment decoding finishes cleanly.
  - Returns `context.Canceled` or clean termination.
