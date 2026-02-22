package api

import (
	"errors"
	"net/http"

	"github.com/thisisjab/logzilla/fault"
)

func (s *server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var f fault.Fault
	if errors.As(err, &f) {
		switch f.Code() {
		case fault.BadInputCode:
			if md, ok := f.Metadata().(fault.FieldErrorsMetadata); ok {
				// This is a 422 error since it's related to specific field
				s.writeError(w, r, http.StatusUnprocessableEntity, apiResponse{
					Success: false,
					Message: f.Message(),
					Metadata: map[string]any{
						"fields": md,
					},
				})
			} else {
				// This is a 400 as it's a bad request with no metadata or unknown metadata
				s.writeError(w, r, http.StatusBadRequest, apiResponse{
					Success:  false,
					Message:  f.Message(),
					Metadata: map[string]any{"context": f.Metadata()},
				})
			}
		case fault.NotFoundCode:
			m := f.Message()
			if m == "" {
				m = "Requested resource not found."
			}

			res := apiResponse{Success: false, Message: m}

			if f.Metadata() != nil {
				res.Metadata = map[string]any{"context": f.Metadata()}
			}

			s.writeError(w, r, http.StatusNotFound, res)

		case fault.PermissionDeniedCode:
			m := f.Message()
			if m == "" {
				m = "Permission denied."
			}
			s.writeError(w, r, http.StatusForbidden, apiResponse{Success: false, Message: m})

		default:
			s.internalServerError(w, r, f)
		}

		return
	}

	s.internalServerError(w, r, err)
}

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
