package token

const (
	ILLEGAL TokenType = iota
	EOF

	// Identifiers + literals
	IDENT
	INT
	DECIMAL
	STRING
	KEYWORD

	// Delimiters
	COMMA
	COLON
	LPAREN
	RPAREN

	EQUAL
	NOTEQUAL
	LESS
	LESSEQUAL
	GREATER
	GREATEREQUAL
	TILDE
	AND
	OR
	NOT
)

type TokenType int

type Token struct {
	Type    TokenType
	Literal string
}
