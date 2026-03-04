package parser

import (
	"testing"

	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/lexer"
	"github.com/thisisjab/logzilla/querier/token"
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

func TestParseValues(t *testing.T) {
	tests := []struct {
		input    string
		expected []any
	}{
		{
			input:    "a,b,c",
			expected: []any{"a", "b", "c"},
		},
		{
			// NOTE: using values like 1.5 is intentional for floats. Since parsing something like 1.3 will give 1.299999999.
			input:    "a,1,1.5",
			expected: []any{"a", 1, 1.5},
		},
		{
			input:    "null,true,false,1,1.5,1,a,\"hello\"",
			expected: []any{nil, true, false, 1, 1.5, 1, "a", "hello"},
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for i, tc := range tests {
		l = lexer.New(tc.input)
		p = New(l)
		result := p.parseValues()

		if len(result) != len(tc.expected) {
			t.Fatalf("[%d] expected result and expected have different lengths: %d != %d\nresult:%+vexpected:%+v", i, len(result), len(tc.expected), result, tc.expected)
		}

		for j := range result {
			if result[j] != tc.expected[j] {
				t.Fatalf("[%d] expected result[%d] be `%+v`, but it's `%v`", i, j, result[j], tc.expected[j])
			}
		}

		if p.peekToken.Type != token.EOF {
			t.Fatalf("[%d] expected EOF, but got `%s (%s)`", i, p.curToken.Literal, p.curToken.Type.String())
		}

		if len(p.errors) != 0 {
			t.Fatalf("[%d] expected no error, but got %d errors: %+v", i, len(p.errors), p.errors)
		}
	}
}
