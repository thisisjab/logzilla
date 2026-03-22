package server

import (
	"net/http"
	"time"
)

func (s *server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.writeJson(w, http.StatusOK, apiResponse{ //nolint:errcheck
		Success: true,
		Message: "OK",
		Metadata: map[string]any{
			"Uptime": time.Since(s.startTime).String(),
		},
	}, nil)
}
