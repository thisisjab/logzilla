package api

import "net/http"

func (s *server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.writeJson(w, http.StatusOK, apiResponse{ //nolint:errcheck
		Success: true,
		Message: "OK",
	}, nil)
}
