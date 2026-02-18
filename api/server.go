package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

type CORSConfig struct {
	TrustedOrigins []string `yaml:"trusted_origins"`
}

type Config struct {
	Addr     string     `yaml:"addr"`
	CertFile string     `yaml:"cert_file"`
	KeyFile  string     `yaml:"key_file"`
	CORS     CORSConfig `yaml:"cors"`
}

type server struct {
	cfg    Config
	logger *slog.Logger
}

// NewServer creates a new server configured with cfg and instrumented by logger.
// It validates that cfg.Addr is non-empty and returns an error if the address is not provided.
func NewServer(cfg Config, logger *slog.Logger) (*server, error) {
	if cfg.Addr == "" {
		return nil, errors.New("addr is required, but not provided")
	}

	return &server{
		cfg:    cfg,
		logger: logger,
	}, nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthcheck", s.healthCheckHandler)

	return s.recoverPanicMiddleware(s.requestLoggerMiddleware(s.corsMiddleware(mux)))
}

func (s *server) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.routes(),
	}

	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down server", "addr", s.cfg.Addr)
		if err := srv.Shutdown(ctx); err != nil {
			s.logger.Error("failed to shutdown server", "addr", s.cfg.Addr, "error", err)
		}
	}()

	var serverErr error
	if s.cfg.CertFile != "" && s.cfg.KeyFile != "" {
		s.logger.Info("starting server with TLS", "addr", s.cfg.Addr)
		serverErr = srv.ListenAndServeTLS(s.cfg.CertFile, s.cfg.KeyFile)
	} else {
		s.logger.Info("starting server without TLS", "addr", s.cfg.Addr)
		serverErr = srv.ListenAndServe()
	}

	if serverErr != nil && serverErr != http.ErrServerClosed {
		return serverErr
	}

	return nil
}