package parser

import (
	"testing"

	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/lexer"
)

func TestParseSingleSortField(t *testing.T) {
	tests := []struct {
		input    string
		expected ast.SortField
	}{
		{
			input:    "sample-field",
			expected: ast.SortField{Name: "sample-field", IsDescending: false},
		},
		{
			input:    "-other-field",
			expected: ast.SortField{Name: "other-field", IsDescending: true},
		},
	}

	for i, tc := range tests {

		l := lexer.New(tc.input)
		p := New(l)

		f, err := p.parseSingleSortField()

		if err != nil {
			t.Fatalf("[%d] couldn't parse single sort field due to error: %s", i, err)
		}

		if f != tc.expected {
			t.Fatalf("[%d] expected test result `%+v` be equal to test case `%+v`", i, f, tc.expected)
		}

		if len(p.errors) != 0 {
			t.Fatalf("[%d] expected no error, but got %d errors: %+v", i, len(p.errors), p.errors)
		}

	}
}
