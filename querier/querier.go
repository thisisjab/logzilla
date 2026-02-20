package querier

import (
	"context"

	"github.com/thisisjab/logzilla/entity"
)

type QueryDirection string

const (
	QueryDirectionForward  QueryDirection = "forward"
	QueryDirectionBackward QueryDirection = "backward"
)

type QueryRequest struct {
	Query Query
}

type QueryResponse struct {
	Records []entity.LogRecord
	Cursor  string
}

type Querier interface {
	Query(ctx context.Context, req QueryRequest) (QueryResponse, error)
}
