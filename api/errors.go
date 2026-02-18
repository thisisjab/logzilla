package api

import "net/http"

func (s *server) logError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("internal server error", "method", r.Method, "path", r.RequestURI, "remote-addr", r.RemoteAddr, "error", err)
}

func (s *server) writeError(w http.ResponseWriter, r *http.Request, status int, response apiResponse) {
	s.writeJson(w, status, response, nil) //nolint:errcheck
}

func (s *server) internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	s.logError(w, r, err)
	s.writeError(w, r, http.StatusInternalServerError, apiResponse{Success: false, Message: "Internal server error"})
}
