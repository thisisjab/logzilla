package main

import (
	"context"
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
	agg, err := aggregator.New(aggregator.Config{
		CollectorsPath: os.Getenv("COLLECTORS_PATH"),
		Logger: shared.NewLogger(readLogLevel()),
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create aggregator: %s\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	exitChan := make(chan os.Signal, 1)

	go func ()  {
		signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)
		<- exitChan
		cancel()
	}()

	err = agg.Ingest(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aggregator stopped with error: %s\n", err)
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
