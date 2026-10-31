package processor_test

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/wal"
)

type dummyProcessor struct {
	name   string
	called bool
}

func (d *dummyProcessor) Name() string {
	return d.name
}

func (d *dummyProcessor) Process(record wal.UnpackedRecord) (*entity.LogRecord, error) {
	d.called = true
	rec := entity.New(
		record.ID,
		record.NodeID,
		"",
		record.CollectorName,
		record.Timestamp,
		entity.LevelInfo,
		record.Raw,
		nil,
		record.Raw,
	)
	return &rec, nil
}

func TestRouterPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		collectorName string
		expectedName  string
	}{
		{
			name:          "exact match takes precedence over glob and catch-all",
			collectorName: "postgres-db",
			expectedName:  "exact-postgres",
		},
		{
			name:          "first glob takes precedence over later glob and catch-all",
			collectorName: "server-app-logs",
			expectedName:  "glob-server-prefix",
		},
		{
			name:          "second glob matches when first does not",
			collectorName: "worker-special-logs",
			expectedName:  "glob-any-logs",
		},
		{
			name:          "single character wildcard ? matches single character",
			collectorName: "node-1",
			expectedName:  "glob-single-char",
		},
		{
			name:          "character range [0-9] matches digits",
			collectorName: "app-3",
			expectedName:  "glob-range",
		},
		{
			name:          "catch-all matches when neither exact nor glob matches",
			collectorName: "nginx-access",
			expectedName:  "catch-all",
		},
	}

	r := processor.NewRouter()

	exactProc := &dummyProcessor{name: "exact-postgres"}
	firstGlobProc := &dummyProcessor{name: "glob-server-prefix"}
	secondGlobProc := &dummyProcessor{name: "glob-any-logs"}
	singleCharProc := &dummyProcessor{name: "glob-single-char"}
	rangeProc := &dummyProcessor{name: "glob-range"}
	catchAllProc := &dummyProcessor{name: "catch-all"}

	if err := r.Add("*", catchAllProc); err != nil {
		t.Fatalf("failed to add catch-all: %v", err)
	}
	if err := r.Add("node-?", singleCharProc); err != nil {
		t.Fatalf("failed to add single-char glob: %v", err)
	}
	if err := r.Add("app-[0-9]", rangeProc); err != nil {
		t.Fatalf("failed to add range glob: %v", err)
	}
	if err := r.Add("server-*-logs", firstGlobProc); err != nil {
		t.Fatalf("failed to add first glob: %v", err)
	}
	if err := r.Add("*-logs", secondGlobProc); err != nil {
		t.Fatalf("failed to add second glob: %v", err)
	}
	if err := r.Add("postgres-db", exactProc); err != nil {
		t.Fatalf("failed to add exact rule: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matched := r.Route(tc.collectorName)
			if matched == nil {
				t.Fatalf("expected processor %q, got nil", tc.expectedName)
			}
			if matched.Name() != tc.expectedName {
				t.Fatalf("expected processor %q, got %q", tc.expectedName, matched.Name())
			}
		})
	}
}

func TestRouterNoMatch(t *testing.T) {
	r := processor.NewRouter()
	proc := &dummyProcessor{name: "json-proc"}
	if err := r.Add("*.json", proc); err != nil {
		t.Fatalf("failed to add glob: %v", err)
	}

	matched := r.Route("syslog")
	if matched != nil {
		t.Fatalf("expected nil for non-matching collector, got %q", matched.Name())
	}

	record := wal.UnpackedRecord{
		NodeID:        uuid.New(),
		Timestamp:     time.Now().UnixMilli(),
		ID:            ulid.Make(),
		CollectorName: "syslog",
		Raw:           "test message",
	}

	res, err := r.Process(record)
	if err != nil {
		t.Fatalf("unexpected error processing unmatched record: %v", err)
	}
	if res != nil {
		t.Fatalf("expected nil result for unmatched record, got %+v", res)
	}
}

func TestRouterAddValidation(t *testing.T) {
	r := processor.NewRouter()

	t.Run("nil processor returns error", func(t *testing.T) {
		err := r.Add("test", nil)
		if err == nil {
			t.Fatal("expected error for nil processor, got nil")
		}
	})

	t.Run("empty pattern returns error", func(t *testing.T) {
		proc := &dummyProcessor{name: "test"}
		err := r.Add("   ", proc)
		if err == nil {
			t.Fatal("expected error for empty pattern, got nil")
		}
	})

	t.Run("invalid glob pattern returns error", func(t *testing.T) {
		proc := &dummyProcessor{name: "test"}
		err := r.Add("[-abc", proc)
		if err == nil {
			t.Fatal("expected error for invalid glob pattern, got nil")
		}
	})
}

func TestRouterProcessWithLua(t *testing.T) {
	script := `
function process(raw, collector, node_id, timestamp)
    if raw == "DROP" then
        return nil
    end
    return {
        level = "warn",
        message = "processed: " .. raw,
        timestamp = timestamp,
    }
end
`
	luaProc, err := processor.NewLuaProcessor("test-lua", script)
	if err != nil {
		t.Fatalf("failed to create lua processor: %v", err)
	}

	r := processor.NewRouter()
	if err := r.Add("app-*", luaProc); err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	t.Run("successful processing", func(t *testing.T) {
		now := time.Now().UnixMilli()
		rec := wal.UnpackedRecord{
			NodeID:        uuid.New(),
			Timestamp:     now,
			ID:            ulid.Make(),
			CollectorName: "app-backend",
			Raw:           "hello world",
		}

		res, err := r.Process(rec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("expected processed record, got nil")
		}
		if res.Message != "processed: hello world" {
			t.Fatalf("unexpected message: %q", res.Message)
		}
		if res.Level != entity.LevelWarn {
			t.Fatalf("unexpected level: %v", res.Level)
		}
	})

	t.Run("dropped record returns nil", func(t *testing.T) {
		rec := wal.UnpackedRecord{
			NodeID:        uuid.New(),
			Timestamp:     time.Now().UnixMilli(),
			ID:            ulid.Make(),
			CollectorName: "app-backend",
			Raw:           "DROP",
		}

		res, err := r.Process(rec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != nil {
			t.Fatalf("expected nil result for dropped record, got %+v", res)
		}
	})
}

type failingProcessor struct{}

func (f *failingProcessor) Name() string { return "failing" }
func (f *failingProcessor) Process(_ wal.UnpackedRecord) (*entity.LogRecord, error) {
	return nil, errors.New("boom")
}

func TestRouterProcessError(t *testing.T) {
	r := processor.NewRouter()
	_ = r.Add("error-collector", &failingProcessor{})

	rec := wal.UnpackedRecord{
		CollectorName: "error-collector",
	}

	_, err := r.Process(rec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
