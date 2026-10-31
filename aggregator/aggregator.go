package aggregator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/viper"
	aggregatorv1 "github.com/thisisjab/logzilla/gen/aggregator/v1"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Config struct {
	ConfigPath string
	Logger     *slog.Logger
	Port       int
	WALDir     string
	Router     *processor.Router
	Storage    *storage.Storage
	viper      *viper.Viper
}

// Aggregator collects WAL files from ingestors, stores them on disk,
// and decodes them through pre-processors in a background worker.
type Aggregator struct {
	aggregatorv1.UnimplementedAggregatorServer

	v          *viper.Viper
	logger     *slog.Logger
	configPath string
	walDir     string
	port       int
	router     *processor.Router
	storage    *storage.Storage

	mu                 sync.Mutex
	grpcServer         *grpc.Server
	listener           net.Listener
	lastProcessedWalID atomic.Int64
	workerWg           sync.WaitGroup
}

func New(cfg Config) (*Aggregator, error) {
	if cfg.Logger == nil {
		return nil, errors.New("logger is nil")
	}

	v := cfg.viper
	if v == nil {
		v = viper.New()
	}

	v.SetDefault("grpc.port", 50051)
	v.SetDefault("wal.dir", "./data/aggregator/wal")

	if cfg.ConfigPath != "" {
		if !strings.HasSuffix(cfg.ConfigPath, ".yml") && !strings.HasSuffix(cfg.ConfigPath, ".yaml") {
			return nil, errors.New("config path must end in .yaml or .yml")
		}
		v.SetConfigFile(cfg.ConfigPath)
		_ = v.ReadInConfig()
	}

	port := v.GetInt("grpc.port")
	if cfg.Port > 0 {
		port = cfg.Port
	}

	walDir := v.GetString("wal.dir")
	if cfg.WALDir != "" {
		walDir = cfg.WALDir
	}

	if err := os.MkdirAll(walDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create WAL directory: %w", err)
	}

	router := cfg.Router
	if router == nil {
		var err error
		router, err = buildRouterFromViper(v)
		if err != nil {
			return nil, fmt.Errorf("cannot configure processors: %w", err)
		}
	}

	storageEngine := cfg.Storage
	if storageEngine == nil {
		storageEngine = storage.NewStorage(storage.DefaultMaxMemTableBytes)
	}

	return &Aggregator{
		v:          v,
		logger:     cfg.Logger,
		configPath: cfg.ConfigPath,
		walDir:     walDir,
		port:       port,
		router:     router,
		storage:    storageEngine,
	}, nil
}

// Port returns the configured or assigned listening port.
func (a *Aggregator) Port() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listener != nil {
		if addr, ok := a.listener.Addr().(*net.TCPAddr); ok {
			return addr.Port
		}
	}
	return a.port
}

// WALDir returns the directory path where received WAL segments are stored.
func (a *Aggregator) WALDir() string {
	return a.walDir
}

// Addr returns the network address string of the active listener.
func (a *Aggregator) Addr() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listener != nil {
		return a.listener.Addr().String()
	}
	return fmt.Sprintf(":%d", a.port)
}

// Router returns the active processor router.
func (a *Aggregator) Router() *processor.Router {
	return a.router
}

// Storage returns the storage engine instance.
func (a *Aggregator) Storage() *storage.Storage {
	return a.storage
}

// LastProcessedWalID returns the highest WAL ID processed by the background worker.
func (a *Aggregator) LastProcessedWalID() int64 {
	return a.lastProcessedWalID.Load()
}

// AppendWAL handles incoming WAL segments from ingestors.
func (a *Aggregator) AppendWAL(ctx context.Context, req *aggregatorv1.AppendWALRequest) (*aggregatorv1.AppendWALResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	if req.WalId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "wal_id must be greater than 0")
	}

	if len(req.Data) == 0 {
		return nil, status.Error(codes.InvalidArgument, "data cannot be empty")
	}

	targetPath := filepath.Join(a.walDir, fmt.Sprintf("%d.wal", req.WalId))

	// Check if already exists (idempotent write)
	if existing, err := os.ReadFile(targetPath); err == nil {
		if len(existing) == len(req.Data) {
			a.logger.Debug("wal segment already stored", "wal_id", req.WalId)
			return &aggregatorv1.AppendWALResponse{}, nil
		}
	}

	// Write atomically using temporary file
	tmpPath := filepath.Join(a.walDir, fmt.Sprintf("%d.wal.tmp-%d", req.WalId, os.Getpid()))
	if err := os.WriteFile(tmpPath, req.Data, 0644); err != nil {
		a.logger.Error("failed to write temporary WAL file", "error", err, "path", tmpPath)
		return nil, status.Errorf(codes.Internal, "cannot write wal file: %s", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		a.logger.Error("failed to rename temporary WAL file", "error", err, "path", targetPath)
		return nil, status.Errorf(codes.Internal, "cannot persist wal file: %s", err)
	}

	a.logger.Info("wal segment persisted", "wal_id", req.WalId, "bytes", len(req.Data))
	return &aggregatorv1.AppendWALResponse{}, nil
}

// Serve starts the gRPC server and background processing worker, blocking until ctx is canceled or an error occurs.
func (a *Aggregator) Serve(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", a.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	aggregatorv1.RegisterAggregatorServer(grpcServer, a)

	a.mu.Lock()
	a.listener = lis
	a.grpcServer = grpcServer
	a.mu.Unlock()

	a.logger.Info("aggregator gRPC server listening", "addr", lis.Addr().String(), "wal_dir", a.walDir)

	// Launch background WAL pipeline worker
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	a.workerWg.Add(1)
	go func() {
		defer a.workerWg.Done()
		a.runWorker(workerCtx)
	}()

	errCh := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		a.logger.Info("stopping aggregator gRPC server")
		grpcServer.GracefulStop()
		workerCancel()
		a.workerWg.Wait()
		return ctx.Err()
	case err := <-errCh:
		grpcServer.GracefulStop()
		workerCancel()
		a.workerWg.Wait()
		return err
	}
}

// Close gracefully stops the server if running and closes the storage engine.
func (a *Aggregator) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
		a.grpcServer = nil
	}
	if a.storage != nil {
		_ = a.storage.Close()
	}
}
