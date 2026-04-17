package lexer

import (
	"testing"

	"github.com/thisisjab/logzilla/querier/token"
)

func TestNextToken(t *testing.T) {
	input := `=:~!(),&|-
	-a=-19
	null
	true
	false
	source=main-server,docker-compose
	timestamp=2016-12-20,2018-01-01
	sort=-level
	sort=level,id,-source
	cursor= xxxx
	message = "hello this is a message"
	message~="error"
	metadata.time_elapsed <= 1000
	metadata.count >= 2000
	metadata.another-number!=3000
	metadata.other_example= 4000
	metadata.hello.bye <4000
	metadata.random>4000
	metadata.example=a,b
	metadata.example=abc,b23
	metadata.example=0.01,43.555
	metadata.sample_data.ali-express=false
	`
	l := New(input)

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.EQUAL, "="},
		{token.COLON, ":"},
		{token.TILDE, "~"},
		{token.NOT, "!"},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.COMMA, ","},
		{token.AND, "&"},
		{token.OR, "|"},
		{token.MINUS, "-"},
		{token.MINUS, "-"},
		{token.IDENT, "a"},
		{token.EQUAL, "="},
		{token.MINUS, "-"},
		{token.INT, "19"},
		{token.NULL, "null"},
		{token.TRUE, "true"},
		{token.FALSE, "false"},
		{token.IDENT, "source"},
		{token.EQUAL, "="},
		{token.IDENT, "main-server"},
		{token.COMMA, ","},
		{token.IDENT, "docker-compose"},
		{token.IDENT, "timestamp"},
		{token.EQUAL, "="},
		{token.STRING, "2016-12-20"},
		{token.COMMA, ","},
		{token.STRING, "2018-01-01"},
		{token.IDENT, "sort"},
		{token.EQUAL, "="},
		{token.MINUS, "-"},
		{token.IDENT, "level"},
		{token.IDENT, "sort"},
		{token.EQUAL, "="},
		{token.IDENT, "level"},
		{token.COMMA, ","},
		{token.IDENT, "id"},
		{token.COMMA, ","},
		{token.MINUS, "-"},
		{token.IDENT, "source"},
		{token.IDENT, "cursor"},
		{token.EQUAL, "="},
		{token.IDENT, "xxxx"},
		{token.IDENT, "message"},
		{token.EQUAL, "="},
		{token.STRING, "hello this is a message"},
		{token.IDENT, "message"},
		{token.TILDE, "~"},
		{token.EQUAL, "="},
		{token.STRING, "error"},
		{token.IDENT, "metadata.time_elapsed"},
		{token.LESSEQUAL, "<="},
		{token.INT, "1000"},
		{token.IDENT, "metadata.count"},
		{token.GREATEREQUAL, ">="},
		{token.INT, "2000"},
		{token.IDENT, "metadata.another-number"},
		{token.NOTEQUAL, "!="},
		{token.INT, "3000"},
		{token.IDENT, "metadata.other_example"},
		{token.EQUAL, "="},
		{token.INT, "4000"},
		{token.IDENT, "metadata.hello.bye"},
		{token.LESS, "<"},
		{token.INT, "4000"},
		{token.IDENT, "metadata.random"},
		{token.GREATER, ">"},
		{token.INT, "4000"},
		{token.IDENT, "metadata.example"},
		{token.EQUAL, "="},
		{token.IDENT, "a"},
		{token.COMMA, ","},
		{token.IDENT, "b"},
		{token.IDENT, "metadata.example"},
		{token.EQUAL, "="},
		{token.IDENT, "abc"},
		{token.COMMA, ","},
		{token.IDENT, "b23"},
		{token.IDENT, "metadata.example"},
		{token.EQUAL, "="},
		{token.DECIMAL, "0.01"},
		{token.COMMA, ","},
		{token.DECIMAL, "43.555"},
		{token.IDENT, "metadata.sample_data.ali-express"},
		{token.EQUAL, "="},
		{token.FALSE, "false"},
		{token.EOF, ""},
	}

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("#%d - expected type `%d`, got `%d`", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("#%d - expected literal `%s`, got `%s`", i, tt.expectedLiteral, tok.Literal)
		}
	}
}
