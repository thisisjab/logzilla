package querier

import (
	"context"

	"github.com/thisisjab/logzilla/entity"
	"github.com/thisisjab/logzilla/querier/ast"
)

type QueryRequest struct {
	Query ast.Query
}

type QueryResponse struct {
	Records []entity.LogRecord
	Cursor  string
}

type Querier interface {
	Query(ctx context.Context, req QueryRequest) (QueryResponse, error)
}
