package aggregator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	aggregatorv1 "github.com/thisisjab/logzilla/gen/aggregator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newTestServer(t *testing.T, walDir string) (*Aggregator, *GRPCServer, context.CancelFunc, context.CancelFunc) {
	t.Helper()
	v := viper.New()
	v.Set("wal.dir", walDir)

	tmpConfigFile, err := os.CreateTemp("", "agg_config_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpConfigFile.Name()) })

	_, err = fmt.Fprintf(tmpConfigFile, "wal:\n  dir: %s\ncollectors: {}\n", walDir)
	require.NoError(t, err)
	tmpConfigFile.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	agg, err := New(Config{
		Logger:         logger,
		CollectorsPath: tmpConfigFile.Name(),
		viper:          v,
	})
	require.NoError(t, err)

	server, err := NewGRPCServer(GRPCServerConfig{
		Port:   getFreePort(t),
		Poller: agg,
		Logger: logger,
	})
	require.NoError(t, err)

	grpcCtx, cancelGRPC := context.WithCancel(context.Background())
	aggCtx, cancelAgg := context.WithCancel(context.Background())

	go func() { _ = server.Run(grpcCtx) }()
	go func() { _ = agg.Ingest(aggCtx) }()

	t.Cleanup(func() {
		cancelGRPC()
		cancelAgg()
	})

	return agg, server, cancelGRPC, cancelAgg
}

func TestNewGRPCServer_Validation(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("returns error when poller is nil", func(t *testing.T) {
		s, err := NewGRPCServer(GRPCServerConfig{
			Port:   9393,
			Poller: nil,
			Logger: logger,
		})
		assert.Nil(t, s)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "poller is nil")
	})

	t.Run("returns error when logger is nil", func(t *testing.T) {
		s, err := NewGRPCServer(GRPCServerConfig{
			Port:   9393,
			Poller: &Aggregator{},
			Logger: nil,
		})
		assert.Nil(t, s)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logger is nil")
	})

	t.Run("returns error with privileged port <= 1024", func(t *testing.T) {
		s, err := NewGRPCServer(GRPCServerConfig{
			Port:   80,
			Poller: &Aggregator{},
			Logger: logger,
		})
		assert.Nil(t, s)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port (must be > 1024): 80")
	})
}

func TestGRPCServer_StartupAndShutdown(t *testing.T) {
	walDir := t.TempDir()
	_, server, cancelGRPC, _ := newTestServer(t, walDir)

	port := server.Port()
	require.Greater(t, port, 1024)

	// Verify server accepts connections
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := aggregatorv1.NewAggregatorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := client.PollWAL(ctx, &aggregatorv1.PollWALRequest{LastWalId: 0})
	require.NoError(t, err)

	_, err = stream.Recv()
	assert.ErrorIs(t, err, io.EOF)

	// Cancel gRPC context and verify listener closes
	cancelGRPC()

	// Wait for listener to fully close
	assert.Eventually(t, func() bool {
		testConn, dialErr := grpc.NewClient(
			fmt.Sprintf("127.0.0.1:%d", port),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if dialErr != nil {
			return true
		}
		defer testConn.Close()
		testClient := aggregatorv1.NewAggregatorServiceClient(testConn)
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer pollCancel()
		testStream, pollErr := testClient.PollWAL(pollCtx, &aggregatorv1.PollWALRequest{LastWalId: 0})
		if pollErr != nil {
			return true
		}
		_, recvErr := testStream.Recv()
		return recvErr != nil && !errors.Is(recvErr, io.EOF)
	}, 3*time.Second, 100*time.Millisecond)
}

func TestGRPCServer_PollWAL_InitialPoll(t *testing.T) {
	walDir := t.TempDir()
	agg, server, _, _ := newTestServer(t, walDir)

	// Write two closed WAL files
	f1Data := []byte("segment 1000 data")
	f2Data := []byte("segment 2000 data")
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "1000.wal"), f1Data, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "2000.wal"), f2Data, 0644))

	activeFileName := agg.wal.ActiveFileName()
	require.NotEmpty(t, activeFileName)

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", server.Port()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := aggregatorv1.NewAggregatorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Initial poll with last_wal_id = 0
	stream, err := client.PollWAL(ctx, &aggregatorv1.PollWALRequest{LastWalId: 0})
	require.NoError(t, err)

	var received []*aggregatorv1.PollWALResponse
	for {
		resp, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		received = append(received, resp)
	}

	// Should receive 1000.wal and 2000.wal in order
	require.Len(t, received, 2)
	assert.Equal(t, int64(1000), received[0].WalId)
	assert.Equal(t, f1Data, received[0].Data)
	assert.Equal(t, int64(2000), received[1].WalId)
	assert.Equal(t, f2Data, received[1].Data)

	// Verify active file was NOT sent
	for _, r := range received {
		assert.NotEqual(t, strings.TrimSuffix(activeFileName, ".wal"), fmt.Sprintf("%d", r.WalId))
	}

	// Verify no files were deleted
	assert.FileExists(t, filepath.Join(walDir, "1000.wal"))
	assert.FileExists(t, filepath.Join(walDir, "2000.wal"))
	assert.FileExists(t, filepath.Join(walDir, activeFileName))
}

func TestGRPCServer_PollWAL_SubsequentPoll(t *testing.T) {
	walDir := t.TempDir()
	agg, server, _, _ := newTestServer(t, walDir)

	// Write three closed WAL files
	f1Data := []byte("segment 1000 data")
	f2Data := []byte("segment 2000 data")
	f3Data := []byte("segment 3000 data")
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "1000.wal"), f1Data, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "2000.wal"), f2Data, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "3000.wal"), f3Data, 0644))

	activeFileName := agg.wal.ActiveFileName()
	require.NotEmpty(t, activeFileName)

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", server.Port()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := aggregatorv1.NewAggregatorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Subsequent poll with last_wal_id = 2000
	stream, err := client.PollWAL(ctx, &aggregatorv1.PollWALRequest{LastWalId: 2000})
	require.NoError(t, err)

	var received []*aggregatorv1.PollWALResponse
	for {
		resp, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		received = append(received, resp)
	}

	// Should only receive 3000.wal
	require.Len(t, received, 1)
	assert.Equal(t, int64(3000), received[0].WalId)
	assert.Equal(t, f3Data, received[0].Data)

	// Verify segments <= 2000 were deleted
	assert.NoFileExists(t, filepath.Join(walDir, "1000.wal"))
	assert.NoFileExists(t, filepath.Join(walDir, "2000.wal"))

	// Verify segment > 2000 and active WAL still exist
	assert.FileExists(t, filepath.Join(walDir, "3000.wal"))
	assert.FileExists(t, filepath.Join(walDir, activeFileName))
}

func TestGRPCServer_PollWAL_Empty(t *testing.T) {
	walDir := t.TempDir()
	_, server, _, _ := newTestServer(t, walDir)

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", server.Port()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := aggregatorv1.NewAggregatorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// PollWAL when only active segment exists
	stream, err := client.PollWAL(ctx, &aggregatorv1.PollWALRequest{LastWalId: 0})
	require.NoError(t, err)

	resp, err := stream.Recv()
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, io.EOF)
}

func TestGRPCServer_PollWAL_ChronologicalOrder(t *testing.T) {
	walDir := t.TempDir()
	_, server, _, _ := newTestServer(t, walDir)

	// Write segments in unordered fashion
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "5000.wal"), []byte("5000"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "1000.wal"), []byte("1000"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "3000.wal"), []byte("3000"), 0644))

	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", server.Port()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := aggregatorv1.NewAggregatorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stream, err := client.PollWAL(ctx, &aggregatorv1.PollWALRequest{LastWalId: 0})
	require.NoError(t, err)

	var ids []int64
	for {
		resp, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		ids = append(ids, resp.WalId)
	}

	require.Len(t, ids, 3)
	assert.Equal(t, []int64{1000, 3000, 5000}, ids)
}
