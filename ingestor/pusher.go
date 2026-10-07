package ingestor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	aggregatorv1 "github.com/thisisjab/logzilla/gen/aggregator/v1"
	"github.com/thisisjab/logzilla/wal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Pusher periodically scans closed WAL files and ships them to the Aggregator.
type Pusher struct {
	logger *slog.Logger
	wal    *wal.WAL

	mu       sync.Mutex
	server   string
	interval time.Duration

	conn   *grpc.ClientConn
	client aggregatorv1.AggregatorClient

	updateCh chan struct{}
}

// NewPusher initializes a new Pusher instance.
func NewPusher(server string, interval time.Duration, w *wal.WAL, logger *slog.Logger) *Pusher {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Pusher{
		logger:   logger,
		wal:      w,
		server:   strings.TrimSpace(server),
		interval: interval,
		updateCh: make(chan struct{}, 1),
	}
}

// UpdateConfig dynamically updates the server address and interval.
func (p *Pusher) UpdateConfig(server string, interval time.Duration) {
	p.mu.Lock()
	server = strings.TrimSpace(server)
	serverChanged := server != "" && server != p.server
	if server != "" {
		p.server = server
	}
	if interval > 0 {
		p.interval = interval
	}
	if serverChanged && p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
		p.client = nil
	}
	p.mu.Unlock()

	select {
	case p.updateCh <- struct{}{}:
	default:
	}
}

// Close closes any open gRPC connection.
func (p *Pusher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil
		p.client = nil
		return err
	}
	return nil
}

// Run executes the periodic push cycle until the context is canceled.
func (p *Pusher) Run(ctx context.Context) {
	p.mu.Lock()
	interval := p.interval
	p.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.updateCh:
			p.mu.Lock()
			newInterval := p.interval
			p.mu.Unlock()
			ticker.Reset(newInterval)
		case <-ticker.C:
			p.PushCycle(ctx)
		}
	}
}

type walCandidate struct {
	id   int64
	name string
}

// PushCycle scans for closed WAL files and attempts to send each one in Best-Effort mode.
func (p *Pusher) PushCycle(ctx context.Context) {
	p.mu.Lock()
	server := p.server
	p.mu.Unlock()

	if server == "" || p.wal == nil {
		return
	}

	dir := p.wal.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		p.logger.Error("failed to read WAL dir in pusher", "dir", dir, "error", err)
		return
	}

	activeFile := p.wal.ActiveFileName()
	var candidates []walCandidate

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".wal") || name == activeFile {
			continue
		}

		idStr := strings.TrimSuffix(name, ".wal")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			p.logger.Warn("skipping WAL file with invalid timestamp filename in pusher", "file", name, "error", err)
			continue
		}

		candidates = append(candidates, walCandidate{id: id, name: name})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].id < candidates[j].id
	})

	for _, cand := range candidates {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Double-check active file name
		if cand.name == p.wal.ActiveFileName() {
			continue
		}

		filePath := filepath.Join(dir, cand.name)
		p.sendSegmentWithRetry(ctx, filePath, cand.id)
	}
}

func (p *Pusher) sendSegmentWithRetry(ctx context.Context, filePath string, walID int64) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			p.logger.Error("failed to read WAL file for push", "file", filePath, "error", err)
		}
		return
	}

	const maxRedirects = 3
	redirects := 0

	for {
		client, err := p.getClient()
		if err != nil {
			p.logger.Error("failed to get gRPC client for push", "error", err)
			return
		}

		var trailer metadata.MD
		_, err = client.AppendWAL(ctx, &aggregatorv1.AppendWALRequest{
			WalId: walID,
			Data:  data,
		}, grpc.Trailer(&trailer))

		if err == nil {
			p.logger.Debug("wal file pushed successfully", "file", filePath, "wal_id", walID)
			_ = os.Remove(filePath)
			return
		}

		st, _ := status.FromError(err)
		stCode := st.Code()
		errMsg := strings.ToLower(st.Message())

		// 1. Duplicate: AlreadyExists or "duplicate"
		if stCode == codes.AlreadyExists || strings.Contains(errMsg, "duplicate") || strings.Contains(errMsg, "already exists") {
			p.logger.Warn("duplicate WAL file on aggregator, removing from disk", "file", filePath, "wal_id", walID)
			_ = os.Remove(filePath)
			return
		}

		// 2. Not Leader: FailedPrecondition or "not leader"
		if stCode == codes.FailedPrecondition || strings.Contains(errMsg, "not leader") || strings.Contains(errMsg, "not_leader") {
			if redirects >= maxRedirects {
				p.logger.Warn("max leader redirects reached for WAL push", "file", filePath, "redirects", redirects)
				return
			}

			newLeader := extractLeaderAddr(trailer)
			if newLeader == "" {
				p.logger.Warn("not leader error received but leader_ip trailer is missing", "file", filePath)
				return
			}

			p.logger.Info("received not_leader redirect, updating push server", "old_server", p.getServer(), "new_server", newLeader)
			p.updateServer(newLeader)
			redirects++
			continue
		}

		// 3. Nack / Unavailable: Unavailable or "nack"
		if stCode == codes.Unavailable || strings.Contains(errMsg, "nack") {
			p.logger.Warn("WAL file nacked by aggregator, will retry next interval", "file", filePath, "error", err)
			return
		}

		// 4. Other errors: log and keep file for next cycle
		p.logger.Error("failed to push WAL file to aggregator", "file", filePath, "error", err)
		return
	}
}

func (p *Pusher) getClient() (aggregatorv1.AggregatorClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.server == "" {
		return nil, errors.New("push server address is empty")
	}

	if p.client != nil && p.conn != nil {
		return p.client, nil
	}

	conn, err := grpc.NewClient(p.server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to push server %s: %w", p.server, err)
	}

	p.conn = conn
	p.client = aggregatorv1.NewAggregatorClient(conn)
	return p.client, nil
}

func (p *Pusher) getServer() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.server
}

func (p *Pusher) updateServer(newServer string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newServer = strings.TrimSpace(newServer)
	if newServer == "" || newServer == p.server {
		return
	}

	p.server = newServer
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
		p.client = nil
	}
}

// extractLeaderAddr extracts the leader address from the gRPC trailer "leader_ip".
func extractLeaderAddr(md metadata.MD) string {
	if md != nil {
		if addrs := md.Get("leader_ip"); len(addrs) > 0 {
			return strings.TrimSpace(addrs[0])
		}
	}
	return ""
}
