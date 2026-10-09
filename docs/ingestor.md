# Ingestor Subsystem

The Ingestor subsystem runs as an agent daemon at the source machine. It is responsible for reading logs from multiple collectors, persisting them locally into Write-Ahead Log (WAL) files, and shipping rotated WAL files to an Aggregator node.

---

## 1. Node Identity (`.ingestor.uuid`)

Every Ingestor has a persistent 16-byte UUID (RFC 4122) generated using the standard library `uuid`.

- On startup, `LoadOrGenerateUUID(path)` checks if the UUID file exists (default: `.ingestor.uuid`).
- If present and valid, it loads the existing UUID.
- If missing, it generates a new UUID v4 and persists it to disk with `0644` permissions.
- This UUID is embedded into every binary WAL record, enabling cluster-wide traceability back to the originating machine.

---

## 2. Configuration & Hot Reloading

The Ingestor uses Viper and `fsnotify` to watch `collectors.yaml` for configuration updates in real-time without interrupting process execution.

### Dynamic Collector Management
- When `collectors.yaml` is modified on disk:
  1. The new collector definitions are compared against the active running collectors.
  2. Any removed collector is canceled via its individual `context.CancelFunc` and allowed to gracefully exit.
  3. Any newly added or updated collector is initialized and spawned in a new goroutine.
  4. Unchanged collectors continue streaming logs without interruption.

```go
type Ingestor struct {
    uuid           uuid.UUID
    v              *viper.Viper
    logger         *slog.Logger
    collectorsPath string
    collectorsMu   sync.RWMutex
    collectors     map[string]*collectorState
    wal            *wal.WAL
    pusher         *Pusher
    ...
}
```

---

## 3. The Pusher Engine

The `Pusher` runs as a background worker responsible for shipping closed WAL files to the central aggregator over gRPC.

### Push Workflow
1. **Periodic Trigger**: Ticks at configured intervals (`push.interval`, default: `30s`).
2. **File Discovery (`PollWAL`)**:
   - Lists files in the WAL directory matching `<timestamp>.wal`.
   - Ignores the currently active WAL file (`wal.ActiveFileName()`).
   - Filters out files already acknowledged (`id <= lastWalID`) and sorts the candidates in ascending chronological order.
3. **gRPC Shipment**:
   - Opens a gRPC connection to `push.server` (e.g. `127.0.0.1:50051`).
   - Calls `AppendWAL(&aggregatorv1.AppendWALRequest{WalId: id, Data: rawBytes})`.
4. **Local Cleanup**:
   - On successful gRPC response, the Pusher records the acknowledged `lastWalID` and removes the local file from disk.

### Dynamic Leader Redirection
When the Aggregator is operating in a multi-node cluster (or behind a non-leader replica), the Aggregator may respond with:
- gRPC status code: `codes.FailedPrecondition`
- Message: `not_leader`
- gRPC Response Trailer Metadata: `leader_ip: <host>:<port>`

When the Pusher detects this trailer:
1. It parses the new leader address from the `leader_ip` header.
2. It closes the current gRPC connection.
3. It reconnects directly to the new leader and immediately retries the write without dropping data.

---

## 4. Lifecycle & Graceful Shutdown

When `SIGINT` or `SIGTERM` is received:
1. Root context is canceled.
2. Every active collector's cancellation function is executed.
3. `ing.collectorWg.Wait()` ensures all reader goroutines complete their in-flight writes to the WAL.
4. The Pusher engine terminates its active loop.
5. `ing.wal.Close()` performs a final `fsync` on the active WAL file and cleanly closes file descriptors.
