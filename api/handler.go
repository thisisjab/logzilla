package api

import (
	"net/http"

	"github.com/thisisjab/logzilla/querier"
	"github.com/thisisjab/logzilla/querier/ast"
)

func (s *server) searchLogsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: add documentation

	// Reading query object from request
	var logQuery ast.Query
	if s.returnOnError(w, r, s.readJson(w, r, &logQuery)) {
		return
	}

	// Preparing request
	req := querier.QueryRequest{Query: logQuery}

	// Getting response
	resp, err := s.services.storage.Query(r.Context(), req)
	if s.returnOnError(w, r, err) {
		return
	}

	// Return JSON response
	s.writeJson( // nolint:errcheck
		w,
		http.StatusOK,
		apiResponse{
			Success: true,
			Data:    resp.Records,
			Metadata: map[string]any{"pagination": map[string]any{
				"cursor": resp.Cursor,
			}},
		},
		nil,
	)

}
