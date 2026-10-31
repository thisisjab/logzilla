package processor

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/wal"
)

// Processor processes an unpacked WAL record into a normalized LogRecord.
type Processor interface {
	Name() string
	Process(record wal.UnpackedRecord) (*entity.LogRecord, error)
}

type globRule struct {
	pattern string
	proc    Processor
}

// Router routes incoming log records to matching processors based on collector name.
// Precedence order:
//  1. Exact match (e.g. "postgres-db")
//  2. Partial glob match (e.g. "server-*-logs", "*.json") evaluated in configuration order; first match wins.
//  3. Catch-all ("*")
type Router struct {
	mu       sync.RWMutex
	exact    map[string]Processor
	globs    []globRule
	catchAll Processor
}

// NewRouter initializes an empty Router.
func NewRouter() *Router {
	return &Router{
		exact: make(map[string]Processor),
	}
}

// Add registers a processor for a collector pattern.
// If an exact pattern or catch-all was already registered, the first registration is retained.
func (r *Router) Add(pattern string, proc Processor) error {
	if proc == nil {
		return errors.New("processor cannot be nil")
	}

	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return errors.New("collector pattern cannot be empty")
	}

	// Validate pattern syntax using filepath.Match
	if _, err := filepath.Match(trimmed, ""); err != nil {
		return fmt.Errorf("invalid collector pattern %q: %w", trimmed, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if trimmed == "*" {
		if r.catchAll == nil {
			r.catchAll = proc
		}
		return nil
	}

	if strings.ContainsAny(trimmed, "*?[]") {
		r.globs = append(r.globs, globRule{
			pattern: trimmed,
			proc:    proc,
		})
		return nil
	}

	if _, exists := r.exact[trimmed]; !exists {
		r.exact[trimmed] = proc
	}

	return nil
}

// Route resolves the matching Processor for a given collector name according to precedence rules.
// Returns nil if no rule matches.
func (r *Router) Route(collectorName string) Processor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Exact match
	if proc, ok := r.exact[collectorName]; ok {
		return proc
	}

	// 2. Partial glob match (first match wins)
	for _, rule := range r.globs {
		if matched, _ := filepath.Match(rule.pattern, collectorName); matched {
			return rule.proc
		}
	}

	// 3. Catch-all wildcard
	if r.catchAll != nil {
		return r.catchAll
	}

	return nil
}

// Process routes an unpacked WAL record by collector name and executes the matched processor.
// If no processor matches or the processor returns nil, (nil, nil) is returned.
func (r *Router) Process(record wal.UnpackedRecord) (*entity.LogRecord, error) {
	proc := r.Route(record.CollectorName)
	if proc == nil {
		return nil, nil
	}

	res, err := proc.Process(record)
	if err != nil {
		return nil, fmt.Errorf("processor %q failed: %w", proc.Name(), err)
	}

	return res, nil
}
