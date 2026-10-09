package entity_test

import (
	"testing"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/thisisjab/logzilla/entity"
)

func TestNewLogRecord(t *testing.T) {
	nodeID := uuid.MustParse("01999999-0000-7000-8000-000000000001")
	recordID := ulid.Make()

	t.Run("explicit timestamp computes UTC datetime method", func(t *testing.T) {
		ts := int64(1760000000123)
		rec := entity.New(
			recordID,
			nodeID,
			"node-1",
			"app-logs",
			ts,
			entity.LevelInfo,
			"user login successful",
			map[string]any{"user_id": 42},
			"raw log line",
		)

		if rec.Timestamp != ts {
			t.Errorf("Timestamp = %d, want %d", rec.Timestamp, ts)
		}
		if rec.DateTime().UnixMilli() != ts {
			t.Errorf("DateTime().UnixMilli() = %d, want %d", rec.DateTime().UnixMilli(), ts)
		}
		if rec.DateTime().Location() != time.UTC {
			t.Errorf("DateTime location = %v, want UTC", rec.DateTime().Location())
		}
		if rec.Level != entity.LevelInfo {
			t.Errorf("Level = %v, want %v", rec.Level, entity.LevelInfo)
		}
		if rec.NodeName != "node-1" {
			t.Errorf("NodeName = %q, want node-1", rec.NodeName)
		}
		if rec.Message != "user login successful" {
			t.Errorf("Message = %q, want user login successful", rec.Message)
		}
		if rec.Metadata["user_id"] != 42 {
			t.Errorf("Metadata[user_id] = %v, want 42", rec.Metadata["user_id"])
		}
	})

	t.Run("nil metadata initializes empty map", func(t *testing.T) {
		rec := entity.New(
			recordID,
			nodeID,
			"node-2",
			"nginx",
			1760000000000,
			entity.LevelWarn,
			"rate limit exceeded",
			nil,
			"raw nginx line",
		)

		if rec.Metadata == nil {
			t.Error("Metadata is nil, want initialized empty map")
		}
		if len(rec.Metadata) != 0 {
			t.Errorf("Metadata len = %d, want 0", len(rec.Metadata))
		}
	})

	t.Run("zero timestamp uses current time", func(t *testing.T) {
		before := time.Now().UnixMilli()
		rec := entity.New(
			recordID,
			nodeID,
			"",
			"default",
			0,
			entity.LevelDebug,
			"ping",
			nil,
			"ping",
		)
		after := time.Now().UnixMilli()

		if rec.Timestamp < before || rec.Timestamp > after {
			t.Errorf("Timestamp = %d, expected between %d and %d", rec.Timestamp, before, after)
		}
		if rec.DateTime().IsZero() {
			t.Error("DateTime() is zero, want non-zero UTC time")
		}
	})
}
