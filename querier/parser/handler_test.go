package parser

import (
	"testing"

	"github.com/thisisjab/logzilla/querier/lexer"
	"github.com/thisisjab/logzilla/querier/token"
)

func TestParseConditionTerm(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    ":level=10",
			expected: "(level = 10)",
		},
		{
			input:    ":level <= 10",
			expected: "(level <= 10)",
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for i, tc := range tests {
		l = lexer.New(tc.input)
		p = New(l)

		parseQueryResult, err := p.ParseQuery()
		if err != nil {
			t.Fatalf("[%d] failed because: `%s`", i, err)
		}

		result := parseQueryResult.Root.String()

		if result != tc.expected {
			t.Fatalf("[%d] expected `%s` after parsing, but got `%s`", i, tc.expected, result)
		}

		if p.peekToken.Type != token.EOF {
			t.Fatalf("[%d] expected EOF, but got `%s (%s)`", i, p.curToken.Literal, p.curToken.Type.String())
		}

		if len(p.errors) != 0 {
			t.Fatalf("[%d] expected no error, but got %d errors: %+v", i, len(p.errors), p.errors)
		}
	}
}

func TestParseAndTerm(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    ": level=10 & message~hello",
			expected: "((level = 10) & (message ~ \"hello\"))",
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for i, tc := range tests {
		l = lexer.New(tc.input)
		p = New(l)

		parseQueryResult, err := p.ParseQuery()
		if err != nil {
			t.Fatalf("[%d] failed because: `%s`", i, err)
		}

		result := parseQueryResult.Root.String()

		if result != tc.expected {
			t.Fatalf("[%d] expected `%s` after parsing, but got `%s`", i, tc.expected, result)
		}

		if p.peekToken.Type != token.EOF {
			t.Fatalf("[%d] expected EOF, but got `%s (%s)`", i, p.curToken.Literal, p.curToken.Type.String())
		}

		if len(p.errors) != 0 {
			t.Fatalf("[%d] expected no error, but got %d errors: %+v", i, len(p.errors), p.errors)
		}
	}
}

func TestParseOrTerm(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    ": level=10 | message~hello",
			expected: "((level = 10) | (message ~ \"hello\"))",
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for i, tc := range tests {
		l = lexer.New(tc.input)
		p = New(l)

		parseQueryResult, err := p.ParseQuery()
		if err != nil {
			t.Fatalf("[%d] failed because: `%s`", i, err)
		}

		result := parseQueryResult.Root.String()

		if result != tc.expected {
			t.Fatalf("[%d] expected `%s` after parsing, but got `%s`", i, tc.expected, result)
		}

		if p.peekToken.Type != token.EOF {
			t.Fatalf("[%d] expected EOF, but got `%s (%s)`", i, p.curToken.Literal, p.curToken.Type.String())
		}

		if len(p.errors) != 0 {
			t.Fatalf("[%d] expected no error, but got %d errors: %+v", i, len(p.errors), p.errors)
		}
	}
}

func TestParseLParen(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    ":(level=10)",
			expected: "(level = 10)",
		},
		{
			input:    ":((level=10))",
			expected: "(level = 10)",
		},
		{
			input:    ":(((level=10)))",
			expected: "(level = 10)",
		},
		{
			input:    ":((level=10) & (metadata.time < 10))",
			expected: "((level = 10) & (metadata.time < 10))",
		},
		{
			input:    ":((level=10) & (metadata.time < 10 | metadata.x >= 6))",
			expected: "((level = 10) & ((metadata.time < 10) | (metadata.x >= 6)))",
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for i, tc := range tests {
		l = lexer.New(tc.input)
		p = New(l)

		parseQueryResult, err := p.ParseQuery()
		if err != nil {
			t.Fatalf("[%d] failed because: `%s`", i, err)
		}

		result := parseQueryResult.Root.String()

		if result != tc.expected {
			t.Fatalf("[%d] expected `%s` after parsing, but got `%s`", i, tc.expected, result)
		}

		if p.peekToken.Type != token.EOF {
			t.Fatalf("[%d] expected EOF, but got `%s (%s)`", i, p.curToken.Literal, p.curToken.Type.String())
		}

		if len(p.errors) != 0 {
			t.Fatalf("[%d] expected no error, but got %d errors: %+v", i, len(p.errors), p.errors)
		}
	}
}

func TestParseNot(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    ":!(level=10)",
			expected: "!((level = 10))",
		},
		{
			input:    ":(!!(level=10))",
			expected: "!(!((level = 10)))",
		},
		{
			input:    ":(!level=10) & id!=10",
			expected: "(!((level = 10)) & (id != 10))",
		},
	}

	var l *lexer.Lexer
	var p *Parser
	for i, tc := range tests {
		l = lexer.New(tc.input)
		p = New(l)

		parseQueryResult, err := p.ParseQuery()
		if err != nil {
			t.Fatalf("[%d] failed because: `%s`", i, err)
		}

		result := parseQueryResult.Root.String()

		if result != tc.expected {
			t.Fatalf("[%d] expected `%s` after parsing, but got `%s`", i, tc.expected, result)
		}

		if p.peekToken.Type != token.EOF {
			t.Fatalf("[%d] expected EOF, but got `%s (%s)`", i, p.curToken.Literal, p.curToken.Type.String())
		}

		if len(p.errors) != 0 {
			t.Fatalf("[%d] expected no error, but got %d errors: %+v", i, len(p.errors), p.errors)
		}
	}
}
