package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/thisisjab/logzilla/ingestor"
	"github.com/thisisjab/logzilla/shared"
)

func main() {
	port := 9393
	if pStr := os.Getenv("GRPC_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil {
			port = p
		}
	}

	logger := shared.NewLogger(readLogLevel())

	collectorsPath := os.Getenv("COLLECTORS_PATH")
	if collectorsPath == "" {
		collectorsPath = "collectors.yaml"
	}

	ing, err := ingestor.New(ingestor.Config{
		CollectorsPath: collectorsPath,
		Logger:         logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create ingestor: %s\n", err)
		os.Exit(1)
	}

	grpcServer, err := ingestor.NewGRPCServer(ingestor.GRPCServerConfig{
		Port:   port,
		Poller: ing,
		Logger: logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create gRPC server: %s\n", err)
		os.Exit(1)
	}

	// Separate contexts for gRPC server and Ingestor
	grpcCtx, cancelGRPC := context.WithCancel(context.Background())
	defer cancelGRPC()

	ingCtx, cancelIng := context.WithCancel(context.Background())
	defer cancelIng()

	grpcErrCh := make(chan error, 1)
	go func() {
		grpcErrCh <- grpcServer.Run(grpcCtx)
	}()

	ingestErrCh := make(chan error, 1)
	go func() {
		ingestErrCh <- ing.Ingest(ingCtx)
	}()

	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-exitChan:
		logger.Info("shutting down...")
	case err := <-grpcErrCh:
		if err != nil {
			logger.Error("gRPC server fatal error", "error", err)
		}
	case err := <-ingestErrCh:
		if err != nil {
			logger.Error("ingestor fatal error", "error", err)
		}
	}

	// 1. Cancel gRPC context first to drain in-flight streams while WAL is still open.
	cancelGRPC()
	if err := <-grpcErrCh; err != nil {
		logger.Error("gRPC server shutdown error", "error", err)
	}

	// 2. Cancel ingestor context second to stop collectors and close WAL.
	cancelIng()
	if err := <-ingestErrCh; err != nil {
		logger.Error("ingestor shutdown error", "error", err)
	}
}

func readLogLevel() slog.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
