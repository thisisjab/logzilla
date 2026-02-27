package ast

import (
	"fmt"
	"time"

	"github.com/thisisjab/logzilla/fault"
)

type QueryDirection string

const (
	QueryDirectionForward  QueryDirection = "forward"
	QueryDirectionBackward QueryDirection = "backward"
)

// Query defines the parameters for searching and filtering logs.
// It supports time-based pagination and flexible sorting.
type Query struct {
	Node QueryNode

	// Sort defines the order of the results. If multiple fields are provided,
	// they are applied in the order they appear in the slice.
	Sort []SortField `json:"sort_fields"`

	// Start defines the beginning of the time range (inclusive).
	// This field is required for all queries.
	Start time.Time `json:"start"`

	// End defines the end of the time range (exclusive).
	// If End is before Start, the query is executed in backward chronological order.
	End time.Time `json:"end"`

	// Limit specifies the maximum number of records to return.
	// Must be between 1 and 1000.
	Limit int `json:"limit"`

	// Cursor is an opaque string used to resume a search from a specific point.
	// When provided, it overrides the starting point of the search.
	Cursor string `json:"cursor,omitempty"`
}

// SortField defines a single sorting criterion.
type SortField struct {
	// Name is the field to sort by (e.g., "timestamp", "severity").
	Name string `json:"name"`
	// IsDescending specifies if the sort should be in reverse order.
	IsDescending bool `json:"is_descending"`
}

// GetQueryDirection determines the temporal direction of the search.
// It returns QueryDirectionBackward if the End timestamp is earlier than the Start,
// indicating the user is searching "into the past."
func (r Query) GetQueryDirection() QueryDirection {
	if !r.End.IsZero() && r.End.Before(r.Start) {
		return QueryDirectionBackward
	}
	return QueryDirectionForward
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
