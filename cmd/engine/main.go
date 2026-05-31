package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thisisjab/logzilla/config"
	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/factory"
	"github.com/thisisjab/logzilla/server"
)

func main() {
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Parse configuration
	cfgPath := flag.String("config", "./.config.yaml", "path to config file")
	flag.Parse()

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
	logger.Info("connecting to storage")
	engineStorage, err := factory.BuildStorage(cfg.Storage)
	if err != nil {
		logger.Error("cannot create storage", "error", err)
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

	if err := engineStorage.Open(ctx); err != nil {
		logger.Error("cannot open connection to the storage", "error", err)
		os.Exit(1)
	}

	// Create api server
	apiServer, err := server.New(cfg.Server, engineStorage, logger)
	if err != nil {
		logger.Error("api server error.", "error", err)
		os.Exit(1)
	}

	// Run engine
	go func() {
		if err := eng.Run(ctx); err != nil {
			logger.Error("engine error.", "error", err)
			cancel()
		}
	}()

	// Run api server
	go func() {
		if err := apiServer.Serve(ctx); err != nil {
			logger.Error("api server error.", "error", err)
			cancel()
		}
	}()

	// Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	select {
	case sig := <-sigChan:
		cancel()
		logger.Info("received signal. shutting down.", "signal", sig)
		if err := engineStorage.Close(ctx); err != nil {
			logger.Error("error when closing the storage", "error", err)
		}
		logger.Info("storage stopped")
	case <-ctx.Done():
		logger.Info("running cancelled")
		logger.Info("engine stopped")
	}

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
