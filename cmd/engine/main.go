package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thisisjab/logzilla/api"
	"github.com/thisisjab/logzilla/config"
	"github.com/thisisjab/logzilla/engine"
	"gopkg.in/yaml.v3"
)

func main() {
	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	cfgPath := flag.String("config", "./.config.yaml", "path to config file")
	flag.Parse()

	fileContent, err := os.ReadFile(*cfgPath)
	if err != nil {
		panic(fmt.Errorf("cannot read config file content: %w", err))
	}

	var cfg config.ConfigSchema
	if err := yaml.Unmarshal(fileContent, &cfg); err != nil {
		panic(fmt.Errorf("cannot parse config file: %w", err))
	}

	parsedCfg, logger, err := cfg.Parse()
	if err != nil {
		if logger != nil {
			logger.Error("cannot parse config file", "error", err)
			os.Exit(1)
		}
		panic(fmt.Errorf("cannot parse config file: %w", err))
	}

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("engine panic", "error", r)
		}
	}()

	// Open storage
	logger.Info("connecting to storage")
	if err := parsedCfg.Storage.Open(ctx); err != nil {
		logger.Error("cannot open connection to the storage", "error", err)
	}
	// Close storage
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		logger.Info("closing storage connection")
		err := parsedCfg.Storage.Close(ctx)
		if err != nil {
			logger.Error("cannot close storage connection", "error", err)
		}

		cancel()
	}()

	// Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create engine
	engine, err := engine.New(parsedCfg.EngineConfig, logger)
	if err != nil {
		logger.Error("engine error.", "error", err)
		os.Exit(1)
	}

	// Create api server
	apiServer, err := api.NewServer(parsedCfg.APIServerConfig, parsedCfg.Storage, logger)
	if err != nil {
		logger.Error("api server error.", "error", err)
		os.Exit(1)
	}

	// Run engine
	go func() {
		if err := engine.Run(ctx); err != nil {
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

	// Wait for signal
	sig := <-sigChan
	logger.Info("received signal. shutting down.", "signal", sig)
	cancel()
	logger.Info("engine stopped.")
}
