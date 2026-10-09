# WAL Binary Format & Decoder

Logzilla uses a custom, compact binary wire format for Write-Ahead Log (WAL) files. This format ensures high write throughput, predictable memory alignment, and safe zero-allocation deserialization.

---

## 1. Binary Record Layout

Every log record inside a `.wal` file consists of a fixed-size header followed by two variable-length byte payloads:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+                                                               +
|                                                               |
+                   Ingestor UUID (16 bytes)                    +
|                                                               |
+                                                               +
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+               Unix Timestamp ms (8 bytes uint64)              +
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+                                                               +
|                                                               |
+                   Record ULID (16 bytes)                      +
|                                                               |
+                                                               +
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|             Collector Name Length (4 bytes uint32)            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|             Collector Name Payload (N bytes UTF-8)            |
|                              ...                              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                Log Data Length (4 bytes uint32)               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  Raw Log Payload (M bytes)                    |
|                              ...                              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### Segment Breakdown

| Field | Size | Encoding | Description |
|---|---|---|---|
| `NodeID` | 16 Bytes | Raw UUID bytes | Standard 128-bit UUID of the ingestor node |
| `Timestamp` | 8 Bytes | BigEndian uint64 | Event arrival Unix timestamp in milliseconds |
| `ID` | 16 Bytes | Raw ULID bytes | Universally Unique Lexicographically Sortable Identifier |
| `ColLen` | 4 Bytes | BigEndian uint32 | Byte length $N$ of the collector name |
| `CollectorName` | $N$ Bytes | UTF-8 String | Name of the originating collector (e.g. `server-a`) |
| `DataLen` | 4 Bytes | BigEndian uint32 | Byte length $M$ of the raw log data |
| `LogData` | $M$ Bytes | Raw Bytes / String | Raw log line or message |

- **Minimum Record Header Size**: $16 + 8 + 16 + 4 + 4 = 48 \text{ bytes}$.

---

## 2. WAL File Lifecycle

### File Naming
WAL files are saved as `<UnixMilli>.wal` (e.g., `1728471234000.wal`). This guarantees natural chronological ordering when listing directories.

### Size-Based Rotation
- When `wal.Append()` is invoked, the WAL writes the encoded byte buffer to the active file.
- The active file size is tracked in memory (`w.size`).
- If `w.size >= wal.max_bytes` during a sync check, `w.Rotate()` is called:
  1. `file.Sync()` flushes dirty OS buffers to disk.
  2. The current file is closed.
  3. A new file `<CurrentUnixMilli>.wal` is created with permissions `0644`.

### Background Sync
- A background ticker (`time.NewTicker(syncInterval)`) wakes up every `wal.sync_interval` (default: `100ms`).
- It calls `file.Sync()` on the active file and triggers rotation if the size limit was exceeded.

---

## 3. Streaming WAL Decoder

The `wal` package provides `RecordIterator` for sequential, memory-efficient decoding:

```go
type RecordIterator struct {
    data []byte
    pos  int
    curr UnpackedRecord
    err  error
}

func NewRecordIterator(data []byte) *RecordIterator
func (it *RecordIterator) Next() bool
func (it *RecordIterator) Record() UnpackedRecord
func (it *RecordIterator) Err() error
```

### Bounds & Corruption Checking
`RecordIterator.Next()` performs checks on every record:
1. **Header Bounds**: Verifies `len(data) - it.pos >= 48` bytes before reading fixed fields.
2. **Collector Length**: Ensures `colLen >= 0` and `it.pos + colLen <= len(data)`.
3. **Data Length**: Ensures `dataLen >= 0` and does not extend beyond remaining bytes.
4. Returns `ErrCorruptRecord` or `ErrUnexpectedEOF` if an invalid length or unexpected truncation is encountered.
