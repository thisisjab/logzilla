package token

type Token struct {
	Type    TokenType
	Literal string
}

type TokenType int

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

func (tt TokenType) String() string {
	values := map[TokenType]string{
		AND:          "AND",
		COLON:        "COLON",
		COMMA:        "COMMA",
		DECIMAL:      "DECIMAL",
		EOF:          "EOF",
		EQUAL:        "EQUAL",
		FALSE:        "FALSE",
		GREATER:      "GREATER",
		GREATEREQUAL: "GREATEREQUAL",
		IDENT:        "IDENT",
		ILLEGAL:      "ILLEGAL",
		INT:          "INT",
		KEYWORD:      "KEYWORD",
		LESS:         "LESS",
		LESSEQUAL:    "LESSEQUAL",
		LPAREN:       "LPAREN",
		MINUS:        "MINUS",
		NOT:          "NOT",
		NOTEQUAL:     "NOTEQUAL",
		NULL:         "NULL",
		OR:           "OR",
		RPAREN:       "RPAREN",
		STRING:       "STRING",
		TILDE:        "TILDE",
		TRUE:         "TRUE",
	}

	return values[tt]
}
