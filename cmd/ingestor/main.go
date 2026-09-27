package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/thisisjab/logzilla/ingestor"
	"github.com/thisisjab/logzilla/shared"
)

func main() {
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := ing.Ingest(ctx); err != nil {
		logger.Error("ingestor fatal error", "error", err)
		os.Exit(1)
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
