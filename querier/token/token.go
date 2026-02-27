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
	NULL
	TRUE
	FALSE

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
	MINUS
	AND
	OR
	NOT
)

type TokenType int

type Token struct {
	Type    TokenType
	Literal string
}
