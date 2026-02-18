package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thisisjab/logzilla/api"
)

func main() {
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// FIXME: read this from config
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("server panic", "error", r)
		}
	}()

	// Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the server in a separate goroutine so we can wait for signals
	go func() {
		sig := <-sigChan
		logger.Info("received signal. shutting down.", "signal", sig)
		cancel()
	}()

	// Create server
	server, err := api.NewServer(api.Config{
		Addr: "localhost:8000",
	}, logger)

	if err != nil {
		logger.Error("server error.", "error", err)
		os.Exit(1)
	}

	// Run server
	if err := server.Serve(ctx); err != nil {
		logger.Error("server error.", "error", err)
		cancel()
		os.Exit(1)
	}

	logger.Info("server stopped.")
}
