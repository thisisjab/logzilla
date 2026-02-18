package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thisisjab/logzilla/api"
)

// main starts the API server, sets up a JSON slog logger, installs panic recovery,
// and listens for SIGINT/SIGTERM to trigger a graceful shutdown.
//
// It initializes a cancellable context, creates the server bound to localhost:8000,
// runs the server until the context is cancelled or an error occurs, and exits with
// status 1 if server creation fails.
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
	}

	logger.Info("server stopped.")
}