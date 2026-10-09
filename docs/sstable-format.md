# LogZilla SSTable Format v1

Final design decisions for the on-disk SSTable used by LogZilla's LSM storage.

- All integers are **big-endian** (same as the WAL).
- Files are **immutable** once written. Name: `<id>.sst`, where `<id>` is a monotonic number.
- Sizes below are in bytes.

---

## 1. File layout

```
 offset
   0   ┌──────────────────────────────────────┐
       │ DATA RECORD 1                        │ ◄──┐
       ├──────────────────────────────────────┤    │
       │ DATA RECORD 2                        │    │ index entries
       ├──────────────────────────────────────┤    │ point at the
       │ ...                                  │    │ start of a
       ├──────────────────────────────────────┤    │ record
       │ DATA RECORD n                        │ ◄──┘
       ╞══════════════════════════════════════╡ ◄── index_start
       │ INDEX ENTRY 1   key | offset         │
       │ INDEX ENTRY 2   key | offset         │
       │ ...                                  │
       │ INDEX ENTRY m   key | offset         │
       ╞══════════════════════════════════════╡ ◄── index_end = size - 1024
       │ FOOTER (1024 bytes, fixed)           │
       └──────────────────────────────────────┘ ◄── size
```

- No header: data starts at offset 0.
- Records are sorted by the index key (see section 4), with `(ts, ulid)` as the final tiebreaker.
- Minimum valid file size is 1024 bytes (an empty file with only a footer).

---

## 2. Field ids

Used by `index_fields` in the footer.

```
┌────┬─────────────────┬───────────┐
│ id │ field           │ indexable │
├────┼─────────────────┼───────────┤
│ 1  │ timestamp       │ yes       │
│ 2  │ level           │ yes       │
│ 3  │ collector name  │ yes       │
│ 4  │ node name       │ yes       │
│ 5  │ node id         │ yes       │
│ 6  │ ulid            │ yes       │
│ 7  │ message         │ no        │
│ 8  │ metadata keys   │ no        │
│ 9  │ raw text        │ no        │
└────┴─────────────────┴───────────┘
```

`0` means "unused slot".

---

## 3. Data record

Each record is self-delimiting, so a file can be scanned from offset 0 to `index_start` without the index (this is what compaction does).

```
┌────────────────────────────────────────────────┐
│ rec_len     u32   bytes that follow (incl crc) │
├────────────────────────────────────────────────┤
│ ts          u64   unix ms                      │
│ ulid        16                                 │
│ node_id     16                                 │
│ level       u8                                 │
│ collector   u16 len | bytes                    │
│ node_name   u16 len | bytes                    │
│ message     u32 len | bytes                    │
│ metadata    u16 n | n x (u16 klen | key |      │
│                          u32 vlen | value)     │
│ raw         u32 len | bytes                    │
├────────────────────────────────────────────────┤
│ crc32       u32   over everything after        │
│                   rec_len, up to the crc       │
└────────────────────────────────────────────────┘
```

- The ULID is always stored, even if it is not an index field. Compaction needs it for total ordering and for dropping duplicates.
- The shape of `metadata` can change without affecting the rest of the format.

---

## 4. Index

### Key

The index key is the encoded values of the fields listed in `index_fields`, joined in slot order. Data is sorted by that key, then by `(ts, ulid)`.

```
 field        encoding
 ts           8 bytes
 level        1 byte
 node id      16 bytes
 ulid         16 bytes
 string       u16 len | bytes   (collector name, node name)
```

### Entry

```
 entry = key | offset (u64)
```

- If every key field is fixed-width, entries are fixed-size and can be binary-searched directly on disk (`ReadAt` / mmap).
- If any key field is a string, entries are variable-length, and the whole index is loaded into memory when the file is opened.

### When to write an entry

```
 - at the first record
 - whenever the leading field's value changes
 - after every N records (default N = 64; N = 1 gives a dense index)
```

### Example: file indexed on `ts` (fixed 16-byte entries)

```
┌─────────────────┬──────────────┐
│ key = ts (8)    │ offset (8)   │
├─────────────────┼──────────────┤
│ 1760000400120   │ 0            │   first record
│ 1760000402404   │ 6144         │
│ 1760000405560   │ 12288        │
└─────────────────┴──────────────┘
```

### Example: file indexed on `(level, ts)`

```
 leading field = level, so one entry is written every time the level changes,
 plus one every N records inside a level run.

┌──────────────────────────┬──────────────┐
│ key = level(1) | ts(8)   │ offset (8)   │
├──────────────────────────┼──────────────┤
│ ERROR | 1760000402404    │ 0            │
│ ERROR | 1760000409871    │ 4096         │
│ INFO  | 1760000400120    │ 8192         │
│ WARN  | 1760000401310    │ 20480        │
└──────────────────────────┴──────────────┘
```

---

## 5. Footer (fixed, 1024 bytes)

The footer is always the **last 1024 bytes** of the file.

```
┌────────┬──────┬────────────────┬──────────────────────────────────────┐
│ offset │ size │ field          │ notes                                │
├────────┼──────┼────────────────┼──────────────────────────────────────┤
│   0    │  2   │ version        │ 1                                    │
│   2    │  4   │ index_fields   │ 4 slots x 1 B (field ids, key order) │
│   6    │  8   │ index_start    │ offset of the first index entry      │
│  14    │  8   │ index_end      │ offset just past the last entry      │
│  22    │  8   │ count          │ number of data records               │
│  30    │  8   │ min_ts         │ smallest timestamp in the file       │
│  38    │  8   │ max_ts         │ largest timestamp in the file        │
│  46    │ 966  │ reserved       │ all zero in v1                       │
│ 1012   │  4   │ crc32          │ over bytes 0..1011                   │
│ 1016   │  8   │ magic          │ "LOGZSST1"  (last 8 bytes of file)   │
└────────┴──────┴────────────────┴──────────────────────────────────────┘
                                                        = 1024 bytes
```

### `index_fields` examples

```
 [01][00][00][00]   index on (ts)
 [01][05][00][00]   index on (ts, node_id)
 [02][01][00][00]   index on (level, ts)
 [03][04][01][00]   index on (collector, node name, ts)
```

### Rules

- Slots hold distinct, **indexable** field ids (1 to 6). Zero slots only appear after the used slots, and at least one slot is used.
- The **CRC covers the reserved bytes too**, so a flipped bit in unused space is still detected.
- The **magic is the very last 8 bytes**, so the first check on open is one small read.
- **Reserved bytes are zero in v1.** A writer that stores anything there must bump `version`. A reader **rejects any version it does not know** instead of guessing. New fields go at the next free offset (46 onward), so existing offsets never move.
- `FooterSize = 1024` is a single constant in code. Do not hardcode the number elsewhere.

### Validation when opening a file

```
 1. size >= 1024
 2. magic matches
 3. crc32 matches
 4. version is known
 5. index_end == size - 1024
 6. index_start <= index_end
 7. index_fields are valid (see rules)
 8. reserved bytes are zero
```

---

## 6. Read path

```
 open file
    │
    ▼
 ReadAt(size - 1024) ──► bad magic / crc / sizes? ──► reject file
    │
    ▼
 [min_ts, max_ts] overlaps the query? ──── no ──► skip file
    │ yes
    ▼
 last index entry with key <= lower bound     (none: start at offset 0)
    │
    ▼
 seek to its offset, scan records, check each record crc
    │
    ▼
 stop when key > upper bound
```

---

## 7. Write path (memtable flush or compaction)

```
 sorted records ──► <id>.sst.tmp
                       │
                       ├─ append each record, track min_ts / max_ts / count
                       ├─ build index entries in memory
                       ├─ append index
                       ├─ append footer (1024 bytes)
                       │
                       ▼
                    fsync file
                       │
                       ▼
                    rename to <id>.sst
                       │
                       ▼
                    fsync directory
                       │
                       ▼
                    update manifest (add new file; for compaction, remove inputs)
```

---

## 8. Compaction and retention

- **Compaction:** k-way heap merge over the inputs' record iterators, in key order. Records with the same ULID are duplicates, so only one is kept. Start with size-tiered: merge N small files into one.
- **Retention:** delete any file whose `max_ts` is older than the cutoff. No rewrite is needed.
- **Secondary indexes** (level, collector, node): the same file format, with a different `index_fields`. Their data should hold only the primary key `(ts, ulid)`, not the full log line, so logs are not stored several times.
