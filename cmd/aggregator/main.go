package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/thisisjab/logzilla/aggregator"
	"github.com/thisisjab/logzilla/shared"
)

func main() {
	configPathFlag := flag.String("config", "", "path to aggregator yaml configuration file (overrides AGGREGATOR_CONFIG env)")
	portFlag := flag.Int("port", 0, "gRPC port override")
	walDirFlag := flag.String("wal-dir", "", "WAL storage directory override")
	flag.Parse()

	logger := shared.NewLogger(readLogLevel())

	configPath := *configPathFlag
	if configPath == "" {
		configPath = os.Getenv("AGGREGATOR_CONFIG")
	}
	if configPath == "" {
		configPath = "aggregator.yaml"
	}

	agg, err := aggregator.New(aggregator.Config{
		ConfigPath: configPath,
		Logger:     logger,
		Port:       *portFlag,
		WALDir:     *walDirFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create aggregator: %s\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := agg.Serve(ctx); err != nil && err != context.Canceled {
		logger.Error("aggregator server error", "error", err)
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
