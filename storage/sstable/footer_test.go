package sstable

import (
	"errors"
	"hash/crc32"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFooterCodec_Roundtrip(t *testing.T) {
	tests := []struct {
		name   string
		footer Footer
	}{
		{
			name: "primary index on timestamp",
			footer: Footer{
				Version:      1,
				IndexFields:  [4]byte{FieldIDTimestamp, 0, 0, 0},
				IndexStart:   10240,
				IndexEnd:     12288,
				Count:        500,
				MinTimestamp: 1700000000000,
				MaxTimestamp: 1700000050000,
			},
		},
		{
			name: "compound index on (level, timestamp)",
			footer: Footer{
				Version:      1,
				IndexFields:  [4]byte{FieldIDLevel, FieldIDTimestamp, 0, 0},
				IndexStart:   512,
				IndexEnd:     2048,
				Count:        100,
				MinTimestamp: 1000,
				MaxTimestamp: 2000,
			},
		},
		{
			name: "four index fields max capacity",
			footer: Footer{
				Version:      1,
				IndexFields:  [4]byte{FieldIDCollector, FieldIDNodeName, FieldIDTimestamp, FieldIDULID},
				IndexStart:   0,
				IndexEnd:     0,
				Count:        0,
				MinTimestamp: 0,
				MaxTimestamp: 0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeFooter(tc.footer)
			require.NoError(t, err)
			require.Len(t, encoded, FooterSize)

			decoded, err := DecodeFooter(encoded[:])
			require.NoError(t, err)

			require.Equal(t, tc.footer.Version, decoded.Version)
			require.Equal(t, tc.footer.IndexFields, decoded.IndexFields)
			require.Equal(t, tc.footer.IndexStart, decoded.IndexStart)
			require.Equal(t, tc.footer.IndexEnd, decoded.IndexEnd)
			require.Equal(t, tc.footer.Count, decoded.Count)
			require.Equal(t, tc.footer.MinTimestamp, decoded.MinTimestamp)
			require.Equal(t, tc.footer.MaxTimestamp, decoded.MaxTimestamp)
		})
	}
}

func TestFooterCodec_TrailingBytes(t *testing.T) {
	footer := Footer{
		Version:      1,
		IndexFields:  [4]byte{FieldIDTimestamp, 0, 0, 0},
		IndexStart:   100,
		IndexEnd:     200,
		Count:        5,
		MinTimestamp: 10,
		MaxTimestamp: 20,
	}

	encoded, err := EncodeFooter(footer)
	require.NoError(t, err)

	// Prepend dummy file data (simulating ReadAt on a large file)
	dummyData := make([]byte, 5000)
	dummyData = append(dummyData, encoded[:]...)

	decoded, err := DecodeFooter(dummyData)
	require.NoError(t, err)
	require.Equal(t, footer.Count, decoded.Count)
	require.Equal(t, footer.MinTimestamp, decoded.MinTimestamp)
}

func TestFooterCodec_CRCCorruption(t *testing.T) {
	footer := Footer{
		Version:      1,
		IndexFields:  [4]byte{FieldIDTimestamp, 0, 0, 0},
		IndexStart:   100,
		IndexEnd:     200,
		Count:        5,
		MinTimestamp: 10,
		MaxTimestamp: 20,
	}

	encoded, err := EncodeFooter(footer)
	require.NoError(t, err)

	corrupted := encoded
	corrupted[10] ^= 0x01

	_, err = DecodeFooter(corrupted[:])
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrFooterCorrupt))
}

func TestFooterCodec_InvalidMagic(t *testing.T) {
	footer := Footer{
		Version:     1,
		IndexFields: [4]byte{FieldIDTimestamp, 0, 0, 0},
	}
	encoded, err := EncodeFooter(footer)
	require.NoError(t, err)

	copy(encoded[1016:1024], "BADMAGIC")

	_, err = DecodeFooter(encoded[:])
	require.ErrorIs(t, err, ErrFooterBadMagic)
}

func TestFooterCodec_TooSmall(t *testing.T) {
	_, err := DecodeFooter(make([]byte, 1023))
	require.ErrorIs(t, err, ErrFooterTooSmall)
}

func TestFooterCodec_InvalidIndexFields(t *testing.T) {
	t.Run("all zero", func(t *testing.T) {
		err := ValidateIndexFields([4]byte{0, 0, 0, 0})
		require.ErrorIs(t, err, ErrFooterBadIndexFields)
	})

	t.Run("zero before used", func(t *testing.T) {
		err := ValidateIndexFields([4]byte{0, 1, 0, 0})
		require.ErrorIs(t, err, ErrFooterBadIndexFields)
	})

	t.Run("invalid field id", func(t *testing.T) {
		err := ValidateIndexFields([4]byte{7, 0, 0, 0})
		require.ErrorIs(t, err, ErrFooterBadIndexFields)
	})

	t.Run("duplicate field id", func(t *testing.T) {
		err := ValidateIndexFields([4]byte{1, 1, 0, 0})
		require.ErrorIs(t, err, ErrFooterBadIndexFields)
	})
}

func TestFooterCodec_InvalidVersion(t *testing.T) {
	t.Run("encode invalid version", func(t *testing.T) {
		footer := Footer{
			Version:     2,
			IndexFields: [4]byte{FieldIDTimestamp, 0, 0, 0},
		}
		_, err := EncodeFooter(footer)
		require.ErrorIs(t, err, ErrFooterUnknownVersion)
	})

	t.Run("decode invalid version", func(t *testing.T) {
		footer := Footer{
			Version:     1,
			IndexFields: [4]byte{FieldIDTimestamp, 0, 0, 0},
		}
		encoded, err := EncodeFooter(footer)
		require.NoError(t, err)

		encoded[0] = 0
		encoded[1] = 99 // version 99
		crc := crc32Checksum(encoded[:1012])
		encoded[1012] = byte(crc >> 24)
		encoded[1013] = byte(crc >> 16)
		encoded[1014] = byte(crc >> 8)
		encoded[1015] = byte(crc)

		_, err = DecodeFooter(encoded[:])
		require.ErrorIs(t, err, ErrFooterUnknownVersion)
	})
}

func TestFooterCodec_InvalidIndexRange(t *testing.T) {
	t.Run("encode start > end", func(t *testing.T) {
		footer := Footer{
			Version:     1,
			IndexFields: [4]byte{FieldIDTimestamp, 0, 0, 0},
			IndexStart:  500,
			IndexEnd:    400, // start > end
		}
		_, err := EncodeFooter(footer)
		require.ErrorIs(t, err, ErrFooterBadIndexRange)
	})

	t.Run("decode start > end", func(t *testing.T) {
		footer := Footer{
			Version:     1,
			IndexFields: [4]byte{FieldIDTimestamp, 0, 0, 0},
			IndexStart:  100,
			IndexEnd:    200,
		}
		encoded, err := EncodeFooter(footer)
		require.NoError(t, err)

		// Overwrite start to 300 > end (200)
		encoded[6] = 0
		encoded[7] = 0
		encoded[8] = 0
		encoded[9] = 0
		encoded[10] = 0
		encoded[11] = 0
		encoded[12] = 1
		encoded[13] = 44 // 300
		crc := crc32Checksum(encoded[:1012])
		encoded[1012] = byte(crc >> 24)
		encoded[1013] = byte(crc >> 16)
		encoded[1014] = byte(crc >> 8)
		encoded[1015] = byte(crc)

		_, err = DecodeFooter(encoded[:])
		require.ErrorIs(t, err, ErrFooterBadIndexRange)
	})
}

func TestFooterCodec_NonZeroReserved(t *testing.T) {
	footer := Footer{
		Version:     1,
		IndexFields: [4]byte{FieldIDTimestamp, 0, 0, 0},
		IndexStart:  100,
		IndexEnd:    200,
	}
	encoded, err := EncodeFooter(footer)
	require.NoError(t, err)

	// Write to reserved offset 100, and recompute CRC so CRC passes but reserved check fails
	encoded[100] = 0xAA
	crc := crc32Checksum(encoded[:1012])
	encoded[1012] = byte(crc >> 24)
	encoded[1013] = byte(crc >> 16)
	encoded[1014] = byte(crc >> 8)
	encoded[1015] = byte(crc)

	_, err = DecodeFooter(encoded[:])
	require.ErrorIs(t, err, ErrFooterBadReserved)
}

func crc32Checksum(b []byte) uint32 {
	return crc32.ChecksumIEEE(b)
}
