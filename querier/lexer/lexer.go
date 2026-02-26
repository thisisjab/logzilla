package lexer

import "github.com/thisisjab/logzilla/querier/token"

type Lexer struct {
	input   []rune
	pos     int  // position of the current character in the input string
	readPos int  // position of the next character to be read
	char    rune // current character being processed
}

func New(input string) *Lexer {
	l := &Lexer{[]rune(input), 0, 0, 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.char = 0
	} else {
		l.char = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	switch l.char {
	case '=':
		tok = token.Token{Type: token.EQUAL, Literal: "="}
	case '~':
		tok = token.Token{Type: token.TILDE, Literal: "~"}
	case '!':
		tok = token.Token{Type: token.NOT, Literal: "!"}
	case ',':
		tok = token.Token{Type: token.COMMA, Literal: ","}
	case '(':
		tok = token.Token{Type: token.LPAREN, Literal: "("}
	case ')':
		tok = token.Token{Type: token.RPAREN, Literal: ")"}
	case ':':
		tok = token.Token{Type: token.COLON, Literal: ":"}

	case 0:
		tok = token.Token{Type: token.EOF, Literal: ""}
	}

	l.readChar()
	return tok
}
