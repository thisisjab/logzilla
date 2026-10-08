package aggregator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	aggregatorv1 "github.com/thisisjab/logzilla/gen/aggregator/v1"
	"github.com/thisisjab/logzilla/ingestor"
	"github.com/thisisjab/logzilla/wal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

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

func TestAggregator_EndToEndIntegration(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	aggWALDir := t.TempDir()
	ingWALDir := t.TempDir()

	// 1. Start Aggregator on random available port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := lis.Addr().(*net.TCPAddr).Port
	_ = lis.Close() // Release so aggregator can bind it

	agg, err := New(Config{
		Logger: logger,
		Port:   port,
		WALDir: aggWALDir,
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

	// Write dummy closed segment to Ingestor
	fPath := filepath.Join(ingWALDir, "5000.wal")
	expectedContent := []byte("ingested-wal-log-segment-5000")
	require.NoError(t, os.WriteFile(fPath, expectedContent, 0644))

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
	assert.Equal(t, expectedContent, storedBytes)

	// 4. Shutdown aggregator
	cancel()
	select {
	case err := <-aggErr:
		assert.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		t.Fatal("aggregator did not shutdown in time")
	}
}
