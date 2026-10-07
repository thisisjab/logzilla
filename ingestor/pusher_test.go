package ingestor

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	aggregatorv1 "github.com/thisisjab/logzilla/gen/aggregator/v1"
	"github.com/thisisjab/logzilla/wal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockAggregatorServer struct {
	aggregatorv1.UnimplementedAggregatorServer

	mu       sync.Mutex
	received []*aggregatorv1.AppendWALRequest

	// handler func to customize responses
	handler func(ctx context.Context, req *aggregatorv1.AppendWALRequest) (*aggregatorv1.AppendWALResponse, error)
}

func (m *mockAggregatorServer) AppendWAL(ctx context.Context, req *aggregatorv1.AppendWALRequest) (*aggregatorv1.AppendWALResponse, error) {
	m.mu.Lock()
	m.received = append(m.received, req)
	h := m.handler
	m.mu.Unlock()

	if h != nil {
		return h(ctx, req)
	}
	return &aggregatorv1.AppendWALResponse{}, nil
}

func startMockAggregator(t *testing.T, srv *mockAggregatorServer) (string, func()) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := grpc.NewServer()
	aggregatorv1.RegisterAggregatorServer(s, srv)

	go func() {
		_ = s.Serve(lis)
	}()

	stop := func() {
		s.Stop()
		_ = lis.Close()
	}

	return lis.Addr().String(), stop
}

func TestPusher_IdleWhenServerEmpty(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	walDir := t.TempDir()
	w, err := wal.New(walDir, 1024*1024, 100*time.Millisecond, logger)
	require.NoError(t, err)
	defer w.Close()

	// Write dummy closed file
	fPath := filepath.Join(walDir, "1000.wal")
	require.NoError(t, os.WriteFile(fPath, []byte("data"), 0644))

	pusher := NewPusher("", 30*time.Second, w, logger)
	defer pusher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	pusher.PushCycle(ctx)

	// File should still exist because server is empty
	assert.FileExists(t, fPath)
}

func TestPusher_SuccessDeletesFile(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	walDir := t.TempDir()
	w, err := wal.New(walDir, 1024*1024, 100*time.Millisecond, logger)
	require.NoError(t, err)
	defer w.Close()

	mockSrv := &mockAggregatorServer{}
	addr, stop := startMockAggregator(t, mockSrv)
	defer stop()

	fPath := filepath.Join(walDir, "1000.wal")
	require.NoError(t, os.WriteFile(fPath, []byte("wal-content-1"), 0644))

	pusher := NewPusher(addr, 30*time.Second, w, logger)
	defer pusher.Close()

	ctx := context.Background()
	pusher.PushCycle(ctx)

	// File should be deleted after successful push
	assert.NoFileExists(t, fPath)

	mockSrv.mu.Lock()
	defer mockSrv.mu.Unlock()
	require.Len(t, mockSrv.received, 1)
	assert.Equal(t, int64(1000), mockSrv.received[0].WalId)
	assert.Equal(t, []byte("wal-content-1"), mockSrv.received[0].Data)
}

func TestPusher_DuplicateWarningAndDeletesFile(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	walDir := t.TempDir()
	w, err := wal.New(walDir, 1024*1024, 100*time.Millisecond, logger)
	require.NoError(t, err)
	defer w.Close()

	mockSrv := &mockAggregatorServer{
		handler: func(ctx context.Context, req *aggregatorv1.AppendWALRequest) (*aggregatorv1.AppendWALResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "duplicate wal file")
		},
	}
	addr, stop := startMockAggregator(t, mockSrv)
	defer stop()

	fPath := filepath.Join(walDir, "1000.wal")
	require.NoError(t, os.WriteFile(fPath, []byte("duplicate-data"), 0644))

	pusher := NewPusher(addr, 30*time.Second, w, logger)
	defer pusher.Close()

	pusher.PushCycle(context.Background())

	// Duplicate file should also be removed from disk
	assert.NoFileExists(t, fPath)
}

func TestPusher_LeaderRedirect(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	walDir := t.TempDir()
	w, err := wal.New(walDir, 1024*1024, 100*time.Millisecond, logger)
	require.NoError(t, err)
	defer w.Close()

	// Server 2: True Leader
	mockLeader := &mockAggregatorServer{}
	leaderAddr, stopLeader := startMockAggregator(t, mockLeader)
	defer stopLeader()

	// Server 1: Follower (redirects to Server 2)
	mockFollower := &mockAggregatorServer{
		handler: func(ctx context.Context, req *aggregatorv1.AppendWALRequest) (*aggregatorv1.AppendWALResponse, error) {
			_ = grpc.SetTrailer(ctx, metadata.Pairs("leader_ip", leaderAddr))
			return nil, status.Error(codes.FailedPrecondition, "not leader")
		},
	}
	followerAddr, stopFollower := startMockAggregator(t, mockFollower)
	defer stopFollower()

	fPath := filepath.Join(walDir, "2000.wal")
	require.NoError(t, os.WriteFile(fPath, []byte("redirect-data"), 0644))

	pusher := NewPusher(followerAddr, 30*time.Second, w, logger)
	defer pusher.Close()

	pusher.PushCycle(context.Background())

	// File should be delivered to leader and removed
	assert.NoFileExists(t, fPath)

	mockLeader.mu.Lock()
	defer mockLeader.mu.Unlock()
	require.Len(t, mockLeader.received, 1)
	assert.Equal(t, int64(2000), mockLeader.received[0].WalId)

	// Server address should now be the leader's address
	assert.Equal(t, leaderAddr, pusher.getServer())
}

func TestPusher_BestEffortDelivery(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	walDir := t.TempDir()
	w, err := wal.New(walDir, 1024*1024, 100*time.Millisecond, logger)
	require.NoError(t, err)
	defer w.Close()

	// Fails file 1000 with NACK (Unavailable), succeeds file 2000
	mockSrv := &mockAggregatorServer{
		handler: func(ctx context.Context, req *aggregatorv1.AppendWALRequest) (*aggregatorv1.AppendWALResponse, error) {
			if req.WalId == 1000 {
				return nil, status.Error(codes.Unavailable, "nack quorum failed")
			}
			return &aggregatorv1.AppendWALResponse{}, nil
		},
	}
	addr, stop := startMockAggregator(t, mockSrv)
	defer stop()

	f1 := filepath.Join(walDir, "1000.wal")
	f2 := filepath.Join(walDir, "2000.wal")
	require.NoError(t, os.WriteFile(f1, []byte("f1"), 0644))
	require.NoError(t, os.WriteFile(f2, []byte("f2"), 0644))

	pusher := NewPusher(addr, 30*time.Second, w, logger)
	defer pusher.Close()

	pusher.PushCycle(context.Background())

	// f1 failed, so it must be kept
	assert.FileExists(t, f1)
	// f2 succeeded, so it must be deleted (Best-Effort)
	assert.NoFileExists(t, f2)
}

func TestPusher_ActiveFileExclusion(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	walDir := t.TempDir()
	w, err := wal.New(walDir, 1024*1024, 100*time.Millisecond, logger)
	require.NoError(t, err)
	defer w.Close()

	mockSrv := &mockAggregatorServer{}
	addr, stop := startMockAggregator(t, mockSrv)
	defer stop()

	// Active WAL file exists
	activeName := w.ActiveFileName()
	require.NotEmpty(t, activeName)
	activePath := filepath.Join(walDir, activeName)

	pusher := NewPusher(addr, 30*time.Second, w, logger)
	defer pusher.Close()

	pusher.PushCycle(context.Background())

	// Active file must never be pushed or deleted
	assert.FileExists(t, activePath)
	mockSrv.mu.Lock()
	defer mockSrv.mu.Unlock()
	assert.Empty(t, mockSrv.received)
}

func TestPusher_UpdateConfig(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	walDir := t.TempDir()
	w, err := wal.New(walDir, 1024*1024, 100*time.Millisecond, logger)
	require.NoError(t, err)
	defer w.Close()

	pusher := NewPusher("", 30*time.Second, w, logger)
	defer pusher.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pusher.Run(ctx)

	mockSrv := &mockAggregatorServer{}
	addr, stop := startMockAggregator(t, mockSrv)
	defer stop()

	fPath := filepath.Join(walDir, "3000.wal")
	require.NoError(t, os.WriteFile(fPath, []byte("data"), 0644))

	// Update config to point to mock server with 50ms interval
	pusher.UpdateConfig(addr, 50*time.Millisecond)

	assert.Eventually(t, func() bool {
		_, err := os.Stat(fPath)
		return os.IsNotExist(err)
	}, 3*time.Second, 50*time.Millisecond)
}
