package lexer

import (
	"testing"

	"github.com/thisisjab/logzilla/querier/token"
)

func TestNextToken(t *testing.T) {
	input := `=~!(),`
	l := New(input)

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.EQUAL, "="},
		{token.TILDE, "~"},
		{token.NOT, "!"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.COMMA, ","},
		{token.EOF, ""},
	}

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("#%d - expected type `%c`, got `%c`", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("#%d - expected literal `%s`, got `%s`", i, tt.expectedLiteral, tok.Literal)
		}
	}
}
