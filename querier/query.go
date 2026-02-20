package querier

import (
	"fmt"
	"time"

	"github.com/thisisjab/logzilla/fault"
)

// Expr represents a boolean expression (evaluates to true/false)
type Expr interface {
	exprNode()
}

type Query struct {
	Expr Expr
	Sort []SortField
	// Below fields are used for time-based pagination.
	Start  time.Time
	End    time.Time
	Limit  int
	Cursor string
}

type SortField struct {
	Name         string
	IsDescending bool
}

func (r Query) Validate() error {
	// MAYBE: In future we may want to read these from configs.
	const LimitMin = 1
	const LimitMax = 1000

	if r.Limit > LimitMax {
		return fault.New(fault.BadInputCode, "").WithMetadata(fault.FieldErrorsMetadata{"limit": []string{fmt.Sprintf("Values larger than %d are not supported.", LimitMax)}})
	}

	if r.Limit < LimitMin {
		return fault.New(fault.BadInputCode, "").WithMetadata(fault.FieldErrorsMetadata{"limit": []string{fmt.Sprintf("Values smaller than %d are not supported.", LimitMin)}})
	}

	if r.Start.IsZero() {
		return fault.New(fault.BadInputCode, "").WithMetadata(fault.FieldErrorsMetadata{"start": []string{"Field is required."}})
	}

	return nil
}

// GetQueryDirection helps storage implementations to determine if query must be in an ascending order or descending.
func (r Query) GetQueryDirection() QueryDirection {
	// If End is before Start, user wants to go backwards in time
	if !r.End.IsZero() && r.End.Before(r.Start) {
		return QueryDirectionBackward
	}
	// Default to Forward
	return QueryDirectionForward
}
