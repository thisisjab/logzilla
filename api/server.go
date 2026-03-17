package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/thisisjab/logzilla/api/ui"
	"github.com/thisisjab/logzilla/querier"
)

type ServerStorage interface {
	querier.QuerierStorage
}

type services struct {
	storage ServerStorage
}

type server struct {
	cfg      Config
	services services
	logger   *slog.Logger
}

func NewServer(cfg Config, queryable ServerStorage, logger *slog.Logger) (*server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &server{
		cfg:      cfg,
		services: services{queryable},
		logger:   logger,
	}, nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	// UI
	mux.HandleFunc("GET /", s.indexPageHandler)
	mux.Handle("GET /static/", http.FileServerFS(ui.Files))

	// API
	mux.HandleFunc("GET /api/healthcheck", s.healthCheckHandler)
	mux.HandleFunc("POST /api/logs/search", s.searchLogsHandler)

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
			s.logger.Error("failed to shutdown server properly", "addr", s.cfg.Addr, "error", err)
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
