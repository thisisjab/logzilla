package ast

import (
	"testing"
	"time"
)

// TestTermEquality tests if equality check function works as expected for terms.
func TestTermsEquality(t *testing.T) {
	comp1 := ComparisonTerm{FieldName: "a", Operator: OperatorEq, Values: []any{"a"}}
	comp2 := ComparisonTerm{FieldName: "b", Operator: OperatorEq, Values: []any{"a"}}
	comp3 := ComparisonTerm{FieldName: "a", Operator: OperatorLike, Values: []any{}}

	tests := []struct {
		term1    Term
		term2    Term
		expected bool
	}{
		// Empty terms
		{AndTerm{}, AndTerm{}, true},
		{OrTerm{}, OrTerm{}, true},
		{ComparisonTerm{}, ComparisonTerm{}, true},
		// Distinctive terms
		{AndTerm{}, OrTerm{}, false},
		{AndTerm{}, ComparisonTerm{}, false},
		{OrTerm{}, ComparisonTerm{}, false},
		// Misc
		{comp1, comp1, true},
		{comp2, comp2, true},
		{comp3, comp3, true},
		{comp1, comp2, false},
		{comp1, comp3, false},
		{comp2, comp3, false},
	}

	for _, tc := range tests {
		if nodesEqual(tc.term1, tc.term2) != tc.expected {
			t.Fatalf("expected `%v` when comparing %+v with %+v", tc.expected, tc.term1, tc.term2)
		}
	}
}

// TestQueryEquality checks if query equality check works as expected.
func TestQueryEquality(t *testing.T) {
	sortFields1 := []SortField{
		{Name: "a", IsDescending: true},
		{Name: "b", IsDescending: false},
	}

	sortFields2 := []SortField{
		{Name: "a", IsDescending: true},
		{Name: "b", IsDescending: false},
	}

	sortFields3 := []SortField{
		{Name: "b", IsDescending: true},
		{Name: "a", IsDescending: false},
	}
	sortFields4 := []SortField{}

	tests := []struct {
		q1       Query
		q2       Query
		expected bool
	}{
		// Sort fields
		{
			// Same sort fields
			Query{Sort: sortFields1},
			Query{Sort: sortFields1},
			true,
		},
		{
			// Same sort fields, but in different addresses
			Query{Sort: sortFields1},
			Query{Sort: sortFields2},
			true,
		},
		{
			// Different sort fields
			Query{Sort: sortFields1},
			Query{Sort: sortFields3},
			false,
		},
		{
			// Empty sort fields
			Query{Sort: sortFields4},
			Query{Sort: sortFields4},
			true,
		},
		// Limit
		{
			// Same values
			Query{Limit: 10},
			Query{Limit: 10},
			true,
		},
		{
			// Different values
			Query{Limit: 10},
			Query{Limit: 5},
			false,
		},
		// Cursor
		{
			// Same values
			Query{Cursor: "x"},
			Query{Cursor: "x"},
			true,
		},
		{
			// Different values
			Query{Cursor: "x"},
			Query{Cursor: "y"},
			false,
		},
		// Start and end
		{
			// Same start, no end
			Query{Start: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)},
			Query{Start: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)},
			true,
		},
		{
			// Different start, no end
			Query{Start: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)},
			Query{Start: time.Date(1999, 0, 0, 0, 0, 0, 0, time.UTC)},
			false,
		},
		{
			// No start, same end
			Query{End: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)},
			Query{End: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)},
			true,
		},
		{
			// No start, different end
			Query{End: time.Date(2000, 0, 0, 0, 0, 0, 0, time.UTC)},
			Query{End: time.Date(1999, 0, 0, 0, 0, 0, 0, time.UTC)},
			false,
		},
		// Misc
		{
			// Empty queries
			Query{},
			Query{},
			true,
		},
	}

	for _, tc := range tests {
		if tc.q1.Equal(&tc.q2) != tc.expected {
			t.Fatalf("expected `%v` when comparing %+v with %+v", tc.expected, tc.q1, tc.q2)
		}
	}
}
