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

## 3. Server Architecture & Graceful Shutdown

```go
type Aggregator struct {
    aggregatorv1.UnimplementedAggregatorServer

    v          *viper.Viper
    logger     *slog.Logger
    configPath string
    walDir     string
    port       int

    mu         sync.Mutex
    grpcServer *grpc.Server
    listener   net.Listener
}
```

### Shutdown Handling
- `agg.Serve(ctx)` listens on the configured TCP port (`grpc.port`, default `50051`).
- When the parent `ctx` is canceled (e.g. `SIGINT`/`SIGTERM`), `grpcServer.GracefulStop()` is invoked:
  - Halts acceptance of new incoming connections.
  - Allows in-flight `AppendWAL` RPCs to flush and finish cleanly.
  - Returns `context.Canceled` or clean termination.
