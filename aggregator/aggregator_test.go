package aggregator

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thisisjab/logzilla/entity"
	aggregatorv1 "github.com/thisisjab/logzilla/gen/aggregator/v1"
	"github.com/thisisjab/logzilla/ingestor"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/wal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func createTestWALData(collectorName, rawData string) []byte {
	nodeID := uuid.New()
	timestamp := uint64(time.Now().UnixMilli())
	logID := ulid.Make()

	colLen := uint32(len(collectorName))
	dataLen := uint32(len(rawData))

	totalLen := 16 + 8 + 16 + 4 + int(colLen) + 4 + int(dataLen)
	rec := make([]byte, totalLen)

	copy(rec[0:16], nodeID[:])
	binary.BigEndian.PutUint64(rec[16:24], timestamp)
	copy(rec[24:40], logID[:])
	binary.BigEndian.PutUint32(rec[40:44], colLen)
	copy(rec[44:44+colLen], collectorName)
	binary.BigEndian.PutUint32(rec[44+colLen:48+colLen], dataLen)
	copy(rec[48+colLen:], rawData)

	return rec
}

type spyProcessor struct {
	name      string
	callCount atomic.Int32
}

func (s *spyProcessor) Name() string { return s.name }
func (s *spyProcessor) Process(_ wal.UnpackedRecord) (*entity.LogRecord, error) {
	s.callCount.Add(1)
	return nil, nil
}

func TestNew_Validation(t *testing.T) {
	t.Run("returns error when logger is nil", func(t *testing.T) {
		agg, err := New(Config{
			Logger: nil,
		})
		assert.Error(t, err)
		assert.Nil(t, agg)
	})

	t.Run("returns error when config path extension is invalid", func(t *testing.T) {
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		agg, err := New(Config{
			Logger:     logger,
			ConfigPath: "config.json",
		})
		assert.Error(t, err)
		assert.Nil(t, agg)
		assert.Contains(t, err.Error(), "config path must end in .yaml or .yml")
	})

	t.Run("defaults port and wal dir", func(t *testing.T) {
		walDir := t.TempDir()
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		v := viper.New()
		v.Set("wal.dir", walDir)

		agg, err := New(Config{
			Logger: logger,
			viper:  v,
		})
		require.NoError(t, err)
		assert.Equal(t, 50051, agg.Port())
		assert.Equal(t, walDir, agg.WALDir())
	})

	t.Run("programmatic overrides take precedence", func(t *testing.T) {
		walDir := t.TempDir()
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

		agg, err := New(Config{
			Logger: logger,
			Port:   9099,
			WALDir: walDir,
		})
		require.NoError(t, err)
		assert.Equal(t, 9099, agg.Port())
		assert.Equal(t, walDir, agg.WALDir())
	})

	t.Run("fails when processor config is invalid", func(t *testing.T) {
		walDir := t.TempDir()
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		v := viper.New()
		v.Set("wal.dir", walDir)
		v.Set("processors", []map[string]any{
			{
				"name": "",
			},
		})

		agg, err := New(Config{
			Logger: logger,
			viper:  v,
		})
		assert.Error(t, err)
		assert.Nil(t, agg)
		assert.Contains(t, err.Error(), "processor name cannot be empty")
	})
}

func TestAppendWAL_Validation(t *testing.T) {
	walDir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	agg, err := New(Config{
		Logger: logger,
		WALDir: walDir,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("nil request returns InvalidArgument", func(t *testing.T) {
		resp, err := agg.AppendWAL(ctx, nil)
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid wal_id returns InvalidArgument", func(t *testing.T) {
		resp, err := agg.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{
			WalId: 0,
			Data:  []byte("data"),
		})
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("empty data returns InvalidArgument", func(t *testing.T) {
		resp, err := agg.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{
			WalId: 1000,
			Data:  nil,
		})
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestAppendWAL_PersistenceAndIdempotency(t *testing.T) {
	walDir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	agg, err := New(Config{
		Logger: logger,
		WALDir: walDir,
	})
	require.NoError(t, err)

	ctx := context.Background()
	payload := []byte("wal segment binary content")

	// First write
	resp, err := agg.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{
		WalId: 1000,
		Data:  payload,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	targetFile := filepath.Join(walDir, "1000.wal")
	assert.FileExists(t, targetFile)
	data, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	assert.Equal(t, payload, data)

	// Idempotent re-send
	resp2, err := agg.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{
		WalId: 1000,
		Data:  payload,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp2)
}

func TestAggregator_BackgroundPipelineWorker(t *testing.T) {
	walDir := t.TempDir()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	router := processor.NewRouter()
	spy := &spyProcessor{name: "spy"}
	require.NoError(t, router.Add("app-*", spy))

	agg, err := New(Config{
		Logger: logger,
		WALDir: walDir,
		Port:   0, // Not starting gRPC listener
		Router: router,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Launch worker in goroutine directly
	go agg.runWorker(ctx)

	// Build WAL data with 2 records matching "app-*" and 1 non-matching "syslog"
	data1 := createTestWALData("app-api", "log line 1")
	data2 := createTestWALData("app-db", "log line 2")
	data3 := createTestWALData("syslog", "log line 3")

	combined := append(data1, data2...)
	combined = append(combined, data3...)

	// Send WAL segment 100 via AppendWAL
	resp, err := agg.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{
		WalId: 100,
		Data:  combined,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Wait for pipeline worker to pick up and process file 100
	require.Eventually(t, func() bool {
		return agg.LastProcessedWalID() == 100 && spy.callCount.Load() == 2
	}, 3*time.Second, 20*time.Millisecond)

	// Send WAL segment 200 via AppendWAL
	data4 := createTestWALData("app-auth", "log line 4")
	resp, err = agg.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{
		WalId: 200,
		Data:  data4,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Wait for pipeline worker to advance to 200 and increment spy call count
	require.Eventually(t, func() bool {
		return agg.LastProcessedWalID() == 200 && spy.callCount.Load() == 3
	}, 3*time.Second, 20*time.Millisecond)

	// Cancel context and verify clean exit
	cancel()
}

func TestAggregator_EndToEndIntegration(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	aggWALDir := t.TempDir()
	ingWALDir := t.TempDir()

	// 1. Start Aggregator on random available port with an inline Lua processor
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	_ = lis.Close() // Release so aggregator can bind it

	v := viper.New()
	v.Set("wal.dir", aggWALDir)
	v.Set("grpc.port", port)
	v.Set("processors", []map[string]any{
		{
			"name": "e2e-proc",
			"script": `
function process(raw, collector, node_id, timestamp)
    return { level = "info", message = raw }
end`,
			"collectors": "*",
		},
	})

	agg, err := New(Config{
		Logger: logger,
		Port:   port,
		WALDir: aggWALDir,
		viper:  v,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	aggErr := make(chan error, 1)
	go func() {
		aggErr <- agg.Serve(ctx)
	}()

	// Wait for aggregator to start accepting connections
	aggAddr := fmt.Sprintf("127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		conn, err := grpc.NewClient(aggAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return false
		}
		defer conn.Close()
		client := aggregatorv1.NewAggregatorClient(conn)
		_, err = client.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{WalId: 1, Data: []byte("ping")})
		return err == nil
	}, 3*time.Second, 50*time.Millisecond)

	// Clean ping test WAL
	_ = os.Remove(filepath.Join(aggWALDir, "1.wal"))

	// 2. Setup Ingestor WAL and Pusher
	ingWAL, err := wal.New(ingWALDir, 1024*1024, 50*time.Millisecond, logger)
	require.NoError(t, err)
	defer ingWAL.Close()

	// Write valid closed segment with encoded records to Ingestor
	fPath := filepath.Join(ingWALDir, "5000.wal")
	validSegmentContent := createTestWALData("test-collector", "hello from ingestor")
	require.NoError(t, os.WriteFile(fPath, validSegmentContent, 0644))

	pusher := ingestor.NewPusher(aggAddr, 50*time.Millisecond, ingWAL, logger)
	defer pusher.Close()

	// Trigger push cycle
	pusher.PushCycle(ctx)

	// 3. Verify:
	// File should be deleted on Ingestor side
	assert.NoFileExists(t, fPath)

	// File should be safely stored on Aggregator side
	aggStoredFile := filepath.Join(aggWALDir, "5000.wal")
	assert.FileExists(t, aggStoredFile)
	storedBytes, err := os.ReadFile(aggStoredFile)
	require.NoError(t, err)
	assert.Equal(t, validSegmentContent, storedBytes)

	// Background worker should have processed file 5000
	require.Eventually(t, func() bool {
		return agg.LastProcessedWalID() == 5000
	}, 3*time.Second, 50*time.Millisecond)

	// 4. Shutdown aggregator
	cancel()
	select {
	case err := <-aggErr:
		assert.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		t.Fatal("aggregator did not shutdown in time")
	}
}
