package sstable

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

// Footer constants defined by LogZilla SSTable Format v1.
const (
	FooterSize = 1024
	Magic      = "LOGZSST1"
	Version    = uint16(1)

	FieldIDUnused    byte = 0
	FieldIDTimestamp byte = 1
	FieldIDLevel     byte = 2
	FieldIDCollector byte = 3
	FieldIDNodeName  byte = 4
	FieldIDNodeID    byte = 5
	FieldIDULID      byte = 6
)

var (
	ErrFooterTooSmall       = errors.New("footer size is less than 1024 bytes")
	ErrFooterBadMagic       = errors.New("invalid sstable magic bytes")
	ErrFooterCorrupt        = errors.New("sstable footer checksum mismatch")
	ErrFooterUnknownVersion = errors.New("unknown sstable version")
	ErrFooterBadIndexRange  = errors.New("invalid index offsets (start > end)")
	ErrFooterBadIndexFields = errors.New("invalid index fields configuration")
	ErrFooterBadReserved    = errors.New("non-zero bytes in reserved footer section")
)

// Footer represents the parsed 1024-byte SSTable file footer.
type Footer struct {
	Version      uint16
	IndexFields  [4]byte
	IndexStart   uint64
	IndexEnd     uint64
	Count        uint64
	MinTimestamp int64
	MaxTimestamp int64
}

// ValidateIndexFields verifies the rules for index field slots:
// 1. At least one slot is used (non-zero).
// 2. Used field IDs must be between 1 and 6.
// 3. All used field IDs must be distinct.
// 4. Zero (unused) slots may only appear after all used slots.
func ValidateIndexFields(fields [4]byte) error {
	seen := make(map[byte]bool, 4)
	usedEnded := false

	for i, f := range fields {
		if f == FieldIDUnused {
			usedEnded = true
			continue
		}

		if usedEnded {
			return fmt.Errorf("%w: unused slot (0) appears before used slot at index %d", ErrFooterBadIndexFields, i)
		}

		if f < FieldIDTimestamp || f > FieldIDULID {
			return fmt.Errorf("%w: invalid field id %d at index %d", ErrFooterBadIndexFields, f, i)
		}

		if seen[f] {
			return fmt.Errorf("%w: duplicate field id %d", ErrFooterBadIndexFields, f)
		}
		seen[f] = true
	}

	if len(seen) == 0 {
		return fmt.Errorf("%w: at least one index field must be specified", ErrFooterBadIndexFields)
	}

	return nil
}

// EncodeFooter serializes a Footer into the fixed 1024-byte on-disk footer representation.
func EncodeFooter(f Footer) ([FooterSize]byte, error) {
	var buf [FooterSize]byte

	if f.Version == 0 {
		f.Version = Version
	} else if f.Version != Version {
		return buf, fmt.Errorf("%w: %d", ErrFooterUnknownVersion, f.Version)
	}

	if err := ValidateIndexFields(f.IndexFields); err != nil {
		return buf, err
	}

	if f.IndexStart > f.IndexEnd {
		return buf, fmt.Errorf("%w: start %d > end %d", ErrFooterBadIndexRange, f.IndexStart, f.IndexEnd)
	}

	binary.BigEndian.PutUint16(buf[0:2], f.Version)
	copy(buf[2:6], f.IndexFields[:])
	binary.BigEndian.PutUint64(buf[6:14], f.IndexStart)
	binary.BigEndian.PutUint64(buf[14:22], f.IndexEnd)
	binary.BigEndian.PutUint64(buf[22:30], f.Count)
	binary.BigEndian.PutUint64(buf[30:38], uint64(f.MinTimestamp))
	binary.BigEndian.PutUint64(buf[38:46], uint64(f.MaxTimestamp))

	// bytes 46..1012 remain zero (reserved)

	crc := crc32.ChecksumIEEE(buf[:1012])
	binary.BigEndian.PutUint32(buf[1012:1016], crc)
	copy(buf[1016:1024], Magic)

	return buf, nil
}

// DecodeFooter decodes and validates a 1024-byte footer.
// If b is larger than 1024 bytes, the trailing 1024 bytes are used.
func DecodeFooter(b []byte) (Footer, error) {
	if len(b) < FooterSize {
		return Footer{}, fmt.Errorf("%w: got %d bytes", ErrFooterTooSmall, len(b))
	}

	footerBytes := b[len(b)-FooterSize:]

	// 1. Magic check (last 8 bytes)
	if string(footerBytes[1016:1024]) != Magic {
		return Footer{}, ErrFooterBadMagic
	}

	// 2. CRC32 check (over bytes 0..1011)
	expectedCRC := binary.BigEndian.Uint32(footerBytes[1012:1016])
	actualCRC := crc32.ChecksumIEEE(footerBytes[:1012])
	if actualCRC != expectedCRC {
		return Footer{}, fmt.Errorf("%w: expected 0x%08x, got 0x%08x", ErrFooterCorrupt, expectedCRC, actualCRC)
	}

	// 3. Version check
	ver := binary.BigEndian.Uint16(footerBytes[0:2])
	if ver != Version {
		return Footer{}, fmt.Errorf("%w: %d", ErrFooterUnknownVersion, ver)
	}

	// 4. Index fields check
	var indexFields [4]byte
	copy(indexFields[:], footerBytes[2:6])
	if err := ValidateIndexFields(indexFields); err != nil {
		return Footer{}, err
	}

	// 5. Index bounds check
	indexStart := binary.BigEndian.Uint64(footerBytes[6:14])
	indexEnd := binary.BigEndian.Uint64(footerBytes[14:22])
	if indexStart > indexEnd {
		return Footer{}, fmt.Errorf("%w: start %d > end %d", ErrFooterBadIndexRange, indexStart, indexEnd)
	}

	// 6. Reserved section check (must be all zero in v1)
	for i := 46; i < 1012; i++ {
		if footerBytes[i] != 0 {
			return Footer{}, fmt.Errorf("%w: byte at offset %d is 0x%02x", ErrFooterBadReserved, i, footerBytes[i])
		}
	}

	count := binary.BigEndian.Uint64(footerBytes[22:30])
	minTS := int64(binary.BigEndian.Uint64(footerBytes[30:38]))
	maxTS := int64(binary.BigEndian.Uint64(footerBytes[38:46]))

	return Footer{
		Version:      ver,
		IndexFields:  indexFields,
		IndexStart:   indexStart,
		IndexEnd:     indexEnd,
		Count:        count,
		MinTimestamp: minTS,
		MaxTimestamp: maxTS,
	}, nil
}
