package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/source"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})) // TODO: read this from config

	// 1. Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// 2. Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 3. Run the engine in a separate goroutine so we can wait for signals
	go func() {
		sig := <-sigChan
		logger.Info("Received signal. Shutting down.", "signal", sig)
		cancel()
	}()

	// TODO: read this from config file
	sources := make(map[string]engine.LogSource)
	sources["file"] = source.NewFileLogSource(logger, "file", "/home/jab/Desktop/logs.json", []string{"json"})
	processors := make(map[string]engine.LogProcessor)
	processors["json"] = processor.NewJsonLogProcessor("l", "m", "t")

	engine := engine.New(engine.Config{
		Sources:    sources,
		Processors: processors,
	}, logger)

	// 4. Run the engine.
	err := engine.Run(ctx)

	if err != nil {
		logger.Error("Engine error.", "error", err)
	}

	logger.Info("Engine stopped.")
}
