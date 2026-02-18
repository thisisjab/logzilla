package api

import (
	"context"
	"log/slog"
	"net/http"
)

type server struct {
	cfg    Config
	logger *slog.Logger
}

func NewServer(cfg Config, logger *slog.Logger) (*server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
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
