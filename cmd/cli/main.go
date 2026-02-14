package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
	"github.com/thisisjab/logzilla/engine"
	"github.com/thisisjab/logzilla/processor"
	"github.com/thisisjab/logzilla/source"
	"github.com/thisisjab/logzilla/storage"
)

func main() {
	logger := slog.New(
		tint.NewHandler(os.Stdout, &tint.Options{
			// TODO: read these values from config
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	)

	// 1. Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// 2. Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 3. Run the engine in a separate goroutine so we can wait for signals
	go func() {
		sig := <-sigChan
		logger.Info("received signal. shutting down.", "signal", sig)
		cancel()
	}()

	// TODO: read this from config file
	sources := make(map[string]engine.LogSource)
	sources["file"] = source.NewFileLogSource(logger, "file", "/home/jab/Desktop/logs.json", []string{"json"})
	processors := make(map[string]engine.LogProcessor)
	processors["json"] = processor.NewJsonLogProcessor("l", "m", "t")
	storage, err := storage.NewClickhouseStorage(storage.ClickhouseStorageConfig{
		Addr:     []string{"localhost:9000"},
		Database: "logzilla",
		Username: "logzilla",
		Password: "logzilla",
	})
	if err != nil {
		logger.Error("storage error.", "error", err)
		os.Exit(1)
	}
	defer storage.Close()

	engine, err := engine.New(engine.Config{
		Sources:                      sources,
		Processors:                   processors,
		Storage:                      storage,
		RawLogsBufferMaxSize:         100,
		ProcessedLogsInBufferMaxSize: 100,
		ProcessorWorkersCount:        10,
	}, logger)
	if err != nil {
		logger.Error("engine error.", "error", err)
		os.Exit(1)
	}

	// 4. Run the engine.
	if err := engine.Run(ctx); err != nil {
		logger.Error("engine error.", "error", err)
	}

	logger.Info("engine stopped.")
}
