# System Architecture

Logzilla is a distributed, append-only log ingestion and storage engine written in Go. Its primary design goals are high throughput, zero log loss at the edge, simple operational maintenance, and modular processing.

---

## 1. High-Level Distributed Topology

Logzilla divides log processing into two primary node types: **Ingestors** (running at the edge / source) and **Aggregators** (running in the central cluster).

```
                      +-------------------+
                      |   Source Files    |
                      | (/var/log/*.log)  |
                      +---------+---------+
                                |
                                v
                +-------------------------------+
                |         INGESTOR NODE         |
                |                               |
                |  +-------------------------+  |
                |  |  Collector Subsystem    |  |
                |  |  (file tailing)         |  |
                |  +------------+------------+  |
                |               |               |
                |               v               |
                |  +-------------------------+  |
                |  |   Local Binary WAL      |  |
                |  |   (fsync & rotation)    |  |
                |  +------------+------------+  |
                |               |               |
                |               v               |
                |  +-------------------------+  |
                |  |      Pusher Engine      |  |
                |  |  (best-effort gRPC push)|  |
                |  +------------+------------+  |
                +---------------+---------------+
                                |
                                | gRPC AppendWAL(wal_id, data)
                                v
                +-------------------------------+
                |        AGGREGATOR NODE        |
                |                               |
                |  +-------------------------+  |
                |  |   gRPC AppendWAL API    |  |
                |  |   (atomic persistence)  |  |
                |  +------------+------------+  |
                |               |               |
                |               v               |
                |  +-------------------------+  |
                |  |   WAL Storage (.wal)    |  |
                |  +------------+------------+  |
                |               |               |
                |               v               |
                |  +-------------------------+  |
                |  | Streaming WAL Unpacker  |  |
                |  +------------+------------+  |
                |               |               |
                |               v               |
                |  +-------------------------+  |
                |  | Lua Pre-Processor       |  |
                |  | (filter, parse, extract)|  |
                |  +------------+------------+  |
                |               |               |
                |               v               |
                |  +-------------------------+  |
                |  |  MemTable (SkipList)    |  |
                |  +------------+------------+  |
                |               |               |
                |               v               |
                |  +-------------------------+  |
                |  |  SSTable Flush & Index  |  |
                |  +-------------------------+  |
                +-------------------------------+
```

---

## 2. Ingestion Pipeline (Edge)

1. **Log Collection**:
   - The Ingestor initializes independent collector routines for each configured source in `collectors.yaml`.
   - The File Collector continuously tails target files from their current byte offsets and chunks incoming lines.
2. **Binary WAL Appending**:
   - Each collected log line is packed with the Ingestor's persistent 16-byte Node UUID, a millisecond Unix timestamp, and a 16-byte ULID identifier into a binary record.
   - Records are appended to the active WAL file (`<UnixMilli>.wal`).
   - The WAL background loop flushes (`fsync`) dirty buffers at configured intervals (`wal.sync_interval`) and rotates the active file once it reaches `wal.max_bytes`.
3. **Pusher Engine**:
   - The Pusher periodically discovers closed (rotated) WAL files in the local WAL directory.
   - It streams closed segments sequentially to the Aggregator via gRPC (`AppendWAL`).
   - If an aggregator node returns `codes.FailedPrecondition` with a `leader_ip` trailer, the Pusher updates its target address and reconnects dynamically.
   - Upon successful acknowledgement, the Ingestor deletes the local WAL file to reclaim disk space.

---

## 3. Aggregation & Storage Pipeline (Cluster)

1. **Atomic Ingestion**:
   - The Aggregator receives raw WAL binary segments via gRPC `AppendWAL`.
   - Segments are written to temporary files (`<wal_id>.wal.tmp-<pid>`) and atomically renamed to `<wal_id>.wal`.
   - If a WAL segment with identical byte length already exists, the write is treated as an idempotent success.
2. **Streaming Unpacking & Decoding**:
   - The binary WAL decoder (`wal.RecordIterator`) iterates through the WAL binary payload in pure Go without memory reallocations, unpacking:
     - `NodeID` (UUID)
     - `Timestamp` (int64 milliseconds)
     - `ID` (ULID)
     - `CollectorName` (string)
     - `Raw` (string)
3. **Lua Processing Engine (Phase 2)**:
   - Unpacked records are passed into sandboxed Lua pre-processing scripts.
   - User-defined Lua scripts parse unstructured lines (JSON, Nginx, Syslog, RegEx), extract key-value fields, and normalize severity levels into unified `LogRecord` structs.
4. **MemTable & Storage (Phase 3 & 4)**:
   - Processed records are inserted into a concurrent in-memory `SkipList` (MemTable) ordered by timestamp and ULID.
   - When the MemTable capacity threshold is reached, it is frozen and flushed as an immutable SSTable on disk following the [SSTable Format v1](sstable-format.md) specification (`<id>.sst`).

---

## 4. Resilience & Error Handling

- **Crash Consistency**: Both Ingestor and Aggregator write to disk before reporting success. Ingestor WAL files ensure that no logs are lost if the host reboots or the network drops.
- **Idempotency**: All WAL files are keyed by monotonic timestamp IDs (`wal_id`). Duplicate pushes from network retries are safely ignored by the Aggregator.
- **Dynamic Leader Redirection**: When the Aggregator cluster forms a consensus group (Raft), write requests sent to follower nodes are automatically redirected to the active leader via gRPC response metadata.
