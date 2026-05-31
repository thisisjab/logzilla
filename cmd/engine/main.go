package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thisisjab/logzilla/config"
	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/factory"
)

func main() {
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Parse flags
	cfgPath := flag.String("config", "./.config.yaml", "path to config file")
	flag.Parse()

	// Parse configuration
	cfg, err := config.Parse(*cfgPath)
	if err != nil {
		panic(fmt.Errorf("cannot parse config file: %w", err))
	}

	// Create logger
	logger := createLogger(cfg.Logger)

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("engine panic", "error", r)
		}
	}()

	// Build storage
	engineStorage, err := factory.BuildStorage(cfg.Storage)
	if err != nil {
		logger.Error("cannot setup storage", "error", err)
		os.Exit(1)
	}

	// Build sources
	sources, err := factory.BuildSources(cfg.Sources, logger)
	if err != nil {
		logger.Error("cannot build sources", "error", err)
		os.Exit(1)
	}

	// Build processors
	processors, err := factory.BuildProcessors(cfg.Processors)
	if err != nil {
		logger.Error("cannot build processors", "error", err)
		os.Exit(1)
	}

	// Set processors and sources on engine
	cfg.Engine.Processors = processors
	cfg.Engine.Sources = sources
	cfg.Engine.Storage = engineStorage

	// Create engine
	eng, err := engine.New(cfg.Engine, logger)
	if err != nil {
		logger.Error("cannot create engine", "error", err)
		os.Exit(1)
	}

	// Start the actual storage connection
	if err := engineStorage.Open(ctx); err != nil {
		logger.Error("cannot open connection to the storage", "error", err)
		os.Exit(1)
	}

	// Run engine
	go func() {
		if err := eng.Run(ctx); err != nil {
			logger.Error("engine error.", "error", err)
			cancel()
		}
	}()

	// Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	// Wait for signal
	select {
	case sig := <-sigChan:
		const secondsToExit = 30
		logger.Warn("received signal, shutting down", "signal", sig, "wait_seconds", secondsToExit)

		cancel()
		time.Sleep(secondsToExit * time.Second)

	case <-ctx.Done():

	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := engineStorage.Close(shutdownCtx); err != nil {
		logger.Error("error when closing the storage", "error", err)
	}

	logger.Info("bye bye")
}

func createLogger(cfg config.LoggerConfig) *slog.Logger {
	var l slog.Level

	switch cfg.Level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	opts := slog.HandlerOptions{Level: l}

	var handler slog.Handler
	switch cfg.Handler {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, &opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, &opts)
	}

	return slog.New(handler)
}
