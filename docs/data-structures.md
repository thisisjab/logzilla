# Data Structures: MemTable and SkipList Engine

Logzilla includes an in-memory **MemTable** backed by a private, generic **SkipList** implementation (`storage/skiplist.go`) designed to serve as the core indexing and sorting engine for ingestion and flushing.

---

## 1. Why a SkipList for MemTable?

A SkipList is a probabilistic alternative to balanced search trees (such as Red-Black Trees or AVL Trees). It provides:
- **$O(\log N)$** expected time complexity for `Search`, `Add`, and `Remove`.
- **Low-Contention Concurrency**: Read/Write operations can be synchronized cleanly with mutexes without tree rebalancing overheads.
- **Natural Range Scans via Level 0 Iterator**: Lowest-level nodes form a sorted singly-linked list, ideal for sequentially iterating log records when flushing to disk (SSTables) or querying time ranges.

---

## 2. Architecture & Probabilistic Towering

```
Level 3:  [Head] ------------------------------> [Record C] ------------------------------> NIL
            |                                       |
Level 2:  [Head] --------------> [Record B] ------> [Record C] --------------> [Record E] --> NIL
            |                       |               |                       |
Level 1:  [Head] --> [Record A] -> [Record B] ------> [Record C] -> [Record D] -> [Record E] --> NIL
            |           |           |               |           |           |
Level 0:  [Head] -> Rec A -> Rec B -> Rec C -> Rec D -> Rec E -> Rec F -> Rec G -> NIL
```

### Configurable Parameters
- `maxLevels`: `16` (supports up to $2^{16}$ elements with optimal height).
- `Probability (P)`: `0.5` (each node has a 50% chance of ascending to the next higher layer).

---

## 3. Storage Layer Architecture

The `storage` package manages the lifecycle of in-memory log ingestion:

```go
// DefaultMaxMemTableBytes is 64MB.
const DefaultMaxMemTableBytes int64 = 64 * 1024 * 1024

// MemTable encapsulates a skipList[entity.LogRecord] sorted by Timestamp, then ULID.
type MemTable struct { ... }

// Storage coordinates active and immutable MemTables, swap handoffs, and write stalls.
type Storage struct { ... }
```

### Ordering Guarantee
`MemTable` orders incoming `entity.LogRecord` instances chronologically:
1. `Timestamp` ascending (millisecond Unix timestamp).
2. `ID.Compare(other.ID)` as the deterministic tie-breaker.

### Memory Accounting
Threshold capacity calculation uses `wal.RecordHeaderSize` (48 bytes wire header) plus lengths of variable fields (`CollectorName`, `Raw`, `Message`). When threshold is reached, `Storage` performs a MemTable swap. If an immutable MemTable is currently flushing, write stall backpressure is applied via condition variable until the flush completes.
