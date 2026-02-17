package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	var cfg config.Config
	if err := yaml.Unmarshal(fileContent, &cfg); err != nil {
		panic(fmt.Errorf("cannot parse config file: %w", err))
	}

	engineCfg, logger, err := cfg.Parse()
	if err != nil {
		if logger != nil {
			logger.Error("cannot parse config file", "error", err)
			os.Exit(1)
		}
		panic(fmt.Errorf("cannot parse config file: %w", err))
	}

	// Setup signal handling to catch Ctrl+C (SIGINT) or Terminate (SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the engine in a separate goroutine so we can wait for signals
	go func() {
		sig := <-sigChan
		logger.Info("received signal. shutting down.", "signal", sig)
		cancel()
	}()

	// Create engine
	engine, err := engine.New(*engineCfg, logger)
	if err != nil {
		logger.Error("engine error.", "error", err)
		os.Exit(1)
	}

	// Run engine
	if err := engine.Run(ctx); err != nil {
		logger.Error("engine error.", "error", err)
		cancel()
	}

	logger.Info("engine stopped.")
}
