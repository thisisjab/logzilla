package aggregator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	aggregatorv1 "github.com/thisisjab/logzilla/gen/aggregator/v1"
	"google.golang.org/grpc"
)

// WALPoller defines the interface required by the gRPC service for polling WAL data.
// grpcServer depends ONLY on this interface, never on *Aggregator directly.
type WALPoller interface {
	PollWAL(ctx context.Context, lastWalID int64) <-chan WALSegment
}

type grpcServer struct {
	aggregatorv1.UnimplementedAggregatorServiceServer
	poller WALPoller
	logger *slog.Logger
}

// PollWAL implements [aggregatorv1.AggregatorServiceServer].
// It receives requests and streams WAL chunks through the channel provided by WALPoller.
func (g *grpcServer) PollWAL(req *aggregatorv1.PollWALRequest, stream grpc.ServerStreamingServer[aggregatorv1.PollWALResponse]) error {
	var lastWalID int64
	if req != nil {
		lastWalID = req.GetLastWalId()
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	for seg := range g.poller.PollWAL(ctx, lastWalID) {
		if seg.Err != nil {
			return seg.Err
		}

		resp := &aggregatorv1.PollWALResponse{
			WalId: seg.ID,
			Data:  seg.Data,
		}
		if err := stream.Send(resp); err != nil {
			return fmt.Errorf("failed to send WAL chunk: %w", err)
		}
	}

	return nil
}

// GRPCServer manages the lifecycle of the Aggregator gRPC service.
type GRPCServer struct {
	server *grpc.Server
	lis    net.Listener
	logger *slog.Logger
}

// GRPCServerConfig defines configuration for creating a GRPCServer.
type GRPCServerConfig struct {
	Port     int
	Listener net.Listener // Optional: if provided, used instead of creating a new listener
	Poller   WALPoller
	Logger   *slog.Logger
}

// NewGRPCServer initializes a new gRPC server wired to the given WALPoller.
func NewGRPCServer(cfg GRPCServerConfig) (*GRPCServer, error) {
	if cfg.Poller == nil {
		return nil, errors.New("poller is nil")
	}
	if cfg.Logger == nil {
		return nil, errors.New("logger is nil")
	}

	lis := cfg.Listener
	if lis == nil {
		if cfg.Port <= 1024 {
			return nil, fmt.Errorf("invalid port (must be > 1024): %d", cfg.Port)
		}
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
		if err != nil {
			return nil, fmt.Errorf("cannot create TCP listener: %w", err)
		}
		lis = l
	}

	reg := grpc.NewServer(
		grpc.MaxSendMsgSize(64*1024*1024),
		grpc.MaxRecvMsgSize(64*1024*1024),
	)

	grpcSvc := &grpcServer{
		poller: cfg.Poller,
		logger: cfg.Logger,
	}
	aggregatorv1.RegisterAggregatorServiceServer(reg, grpcSvc)

	return &GRPCServer{
		server: reg,
		lis:    lis,
		logger: cfg.Logger,
	}, nil
}

// Run starts the gRPC server and serves requests until ctx is cancelled.
// When ctx is cancelled, it gracefully stops the server (draining active connections)
// and closes the listener.
func (s *GRPCServer) Run(ctx context.Context) error {
	serveErrCh := make(chan error, 1)
	go func() {
		if err := s.server.Serve(s.lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) && !errors.Is(err, net.ErrClosed) {
			serveErrCh <- err
		}
		close(serveErrCh)
	}()

	select {
	case err := <-serveErrCh:
		return err
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-time.After(3 * time.Second):
			s.server.Stop()
		}

		if s.lis != nil {
			_ = s.lis.Close()
		}

		return <-serveErrCh
	}
}

// Port returns the TCP port the gRPC server is listening on.
func (s *GRPCServer) Port() int {
	if s.lis == nil {
		return 0
	}
	if tcpAddr, ok := s.lis.Addr().(*net.TCPAddr); ok {
		return tcpAddr.Port
	}
	return 0
}
