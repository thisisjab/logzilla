package api

import (
	"net/http"

	"github.com/thisisjab/logzilla/querier"
	"github.com/thisisjab/logzilla/querier/lexer"
	"github.com/thisisjab/logzilla/querier/parser"
)

func (s *server) searchLogsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: add documentation
	var reqBody struct {
		Query string `json:"query"`
	}

	// Reading query object from request
	if s.returnOnError(w, r, s.readJson(w, r, &reqBody)) {
		return
	}

	// Process user given string using lexer and parser
	// WARN: this is the worst place to do this
	// TODO: get rid of this garbage right away
	p := parser.New(lexer.New(reqBody.Query)).ParseQuery()

	// Preparing request
	req := querier.QueryRequest{Query: *p}

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
			Metadata: map[string]any{
				"cursor": resp.Cursor,
			},
		},
		nil,
	)

}
