package source

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/thisisjab/logzilla/entity"
)

type ShellLogSourceConfig struct {
	Name           string   `yaml:"name"`
	Command        []string `yaml:"command"`
	ProcessorNames []string `yaml:"processors"`
}

type ShellLogSource struct {
	cfg     ShellLogSourceConfig
	logger  *slog.Logger
	cmdName string
	cmdArgs []string
}

func NewShellLogSource(cfg ShellLogSourceConfig, logger *slog.Logger) (*ShellLogSource, error) {
	if len(cfg.Command) < 1 {
		return nil, fmt.Errorf("command cannot be empty")
	}

	s := ShellLogSource{
		logger: logger,
		cfg:    cfg,
	}

	if len(cfg.Command) > 1 {
		s.cmdName = cfg.Command[0]
		s.cmdArgs = cfg.Command[1:]
	} else if len(cfg.Command) == 1 {
		s.cmdName = cfg.Command[0]
		s.cmdArgs = make([]string, 0)
	} else {
		return nil, fmt.Errorf("cannot run processor with an empty command")
	}

	return &s, nil
}

func (s *ShellLogSource) Name() string {
	return s.cfg.Name
}

func (s *ShellLogSource) ProcessorNames() []string {
	return s.cfg.ProcessorNames
}

func (s *ShellLogSource) Provide(ctx context.Context, logChan chan<- entity.LogRecord) error {
	cmd := exec.CommandContext(ctx, s.cmdName, s.cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot start command `%s` with given args (%s): %w", s.cmdName, strings.Join(s.cmdArgs, ", "), err)
	}

	scannerStdout := bufio.NewScanner(stdout)
	go func() {
		for scannerStdout.Scan() {
			logChan <- entity.LogRecord{
				Source:    s.Name(),
				RawData:   bytes.Clone(scannerStdout.Bytes()),
				Timestamp: time.Now(),
			}
		}
	}()

	scannerStderr := bufio.NewScanner(stderr)
	go func() {
		for scannerStderr.Scan() {
			logChan <- entity.LogRecord{
				Source:    s.Name(),
				RawData:   bytes.Clone(scannerStderr.Bytes()),
				Timestamp: time.Now(),
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			s.logger.Info("context is done, stopping the source", "name", s.Name(), "type", "shell")
			return nil
		} else {
			return fmt.Errorf("command finished with error: %w", err)
		}
	}

	return nil
}
