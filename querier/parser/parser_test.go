package parser

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/lexer"
	"github.com/thisisjab/logzilla/querier/token"
)

func TestParseControlSectionTimestamp(t *testing.T) {
	tests := map[string]ast.Query{
		"timestamp=2021-04-17": {
			Start: time.Date(2021, 4, 17, 0, 0, 0, 0, time.UTC),
		},
		"timestamp=2021-04-17,2022-03-10": {
			Start: time.Date(2021, 4, 17, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2022, 3, 10, 0, 0, 0, 0, time.UTC),
		},
		"timestamp=2022-02-12T12:00:00": {
			Start: time.Date(2022, 2, 12, 12, 0, 0, 0, time.UTC),
		},
		"timestamp=2022-02-12T12:00:00,2022-02-12T10:10:10": {
			Start: time.Date(2022, 2, 12, 12, 0, 0, 0, time.UTC),
			End:   time.Date(2022, 2, 12, 10, 10, 10, 0, time.UTC),
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for input, expected := range tests {
		l = lexer.New(input)
		p = New(l)

		actual := p.ParseQuery()
		if !actual.Equal(&expected) {
			t.Fatalf("ParseQuery(%q)\n%+v,\nwant %+v", input, actual, expected)
		}

		if p.curToken.Type != token.EOF {
			t.Fatalf("Expected EOF token, got %v", p.curToken)
		}
	}
}

func TestParseControlSectionLimit(t *testing.T) {
	tests := map[string]ast.Query{
		"limit=100": {
			Limit: 100,
		},
		"limit=1000": {
			Limit: 1000,
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for input, expected := range tests {
		l = lexer.New(input)
		p = New(l)

		actual := p.ParseQuery()
		if !actual.Equal(&expected) {
			t.Fatalf("ParseQuery(%q)\n%+v,\nwant %+v", input, actual, expected)
		}

		if p.curToken.Type != token.EOF {
			t.Fatalf("Expected EOF token, got %v", p.curToken)
		}
	}
}

func TestParseControlSectionOffset(t *testing.T) {
	testUUID := uuid.New()

	tests := map[string]ast.Query{
		"cursor=\"1234567890\"": {
			Cursor: "1234567890",
		},
		fmt.Sprintf("cursor=%v", testUUID): {
			Cursor: testUUID.String(),
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for input, expected := range tests {
		l = lexer.New(input)
		p = New(l)

		actual := p.ParseQuery()
		if !actual.Equal(&expected) {
			t.Fatalf("ParseQuery(%q)\n%+v,\nwant %+v", input, actual, expected)
		}

		if p.curToken.Type != token.EOF {
			t.Fatalf("Expected EOF token, got %v", p.curToken)
		}
	}
}
