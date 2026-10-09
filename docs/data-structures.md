# Data Structures: Concurrent SkipList (MemTable Engine)

Logzilla includes a concurrent, in-memory **SkipList** implementation (`dsa/skiplist/skiplist.go`) designed to serve as the core indexing and sorting engine for the MemTable.

---

## 1. Why a SkipList?

A SkipList is a probabilistic alternative to balanced search trees (such as Red-Black Trees or AVL Trees). It provides:
- **$O(\log N)$** expected time complexity for `Search`, `Insert`, and `Delete`.
- **Lock-free / Low-contention concurrency potential**: Linked-list layers allow fine-grained concurrency or read-write locks without expensive tree-rebalancing operations.
- **Natural Range Scans**: Lowest-level nodes form a sorted singly-linked list, ideal for sequentially iterating log records over a time interval.

---

## 2. Architecture & Probabilistic Towering

```
Level 3:  [Head] ------------------------------> [Key: 50] ------------------------------> NIL
            |                                       |
Level 2:  [Head] --------------> [Key: 25] ------> [Key: 50] --------------> [Key: 80] --> NIL
            |                       |               |                       |
Level 1:  [Head] --> [Key: 10] -> [Key: 25] ------> [Key: 50] -> [Key: 60] -> [Key: 80] --> NIL
            |           |           |               |           |           |
Level 0:  [Head] -> 10 -> 15 -> 20 -> 25 -> 30 -> 40 -> 50 -> 55 -> 60 -> 70 -> 80 -> 90 -> NIL
```

### Configurable Parameters
- `MaxLevel`: Default `16` (supports millions of items with optimal height).
- `Probability (P)`: Default `0.5` (each node has a 50% chance of ascending to the next higher layer).

```go
type SkipList struct {
    head     *Node
    maxLevel int
    level    int
    p        float64
    size     int
    mu       sync.RWMutex
}
```

---

## 3. Core API

```go
// New creates a new SkipList instance with custom max level and probability.
func New(maxLevel int, p float64) *SkipList

// Insert stores a key-value pair. If key already exists, updates value.
func (sl *SkipList) Insert(key string, value any)

// Search retrieves value for a key in O(log N) time.
func (sl *SkipList) Search(key string) (any, bool)

// Delete removes a key and returns true if deleted.
func (sl *SkipList) Delete(key string) bool

// Size returns total count of elements in O(1) time.
func (sl *SkipList) Size() int
```

---

## 4. Role in Logzilla MemTable

During Phase 3, incoming unpacked and pre-processed `LogRecord` instances are inserted into the SkipList using a composite key:
`key := fmt.Sprintf("%020d_%s", record.Timestamp, record.ID.String())`

This guarantees:
1. All records in memory are automatically sorted chronologically.
2. Ties in timestamps are broken deterministically by ULID.
3. Fast range scanning during queries and SSTable disk flushes.
