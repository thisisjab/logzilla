package lexer

import "github.com/thisisjab/logzilla/querier/token"

type Lexer struct {
	input   []rune
	pos     int  // position of the current character in the input string
	readPos int  // position of the next character to be read
	char    rune // current character being processed
}

var keywords = map[string]token.TokenType{
	"null":  token.NULL,
	"true":  token.TRUE,
	"false": token.FALSE,
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

func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPos]
	}
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()

	switch l.char {
	case '=':
		tok = token.Token{Type: token.EQUAL, Literal: "="}
	case '~':
		tok = token.Token{Type: token.TILDE, Literal: "~"}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.LESSEQUAL, Literal: "<="}
		} else {
			tok = token.Token{Type: token.LESS, Literal: "<"}
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.GREATEREQUAL, Literal: ">="}
		} else {
			tok = token.Token{Type: token.GREATER, Literal: ">"}
		}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok = token.Token{Type: token.NOTEQUAL, Literal: "!="}
		} else {
			tok = token.Token{Type: token.NOT, Literal: "!"}
		}
	case ',':
		tok = token.Token{Type: token.COMMA, Literal: ","}
	case '(':
		tok = token.Token{Type: token.LPAREN, Literal: "("}
	case ')':
		tok = token.Token{Type: token.RPAREN, Literal: ")"}
	case ':':
		tok = token.Token{Type: token.COLON, Literal: ":"}
	case '&':
		tok = token.Token{Type: token.AND, Literal: "&"}
	case '|':
		tok = token.Token{Type: token.OR, Literal: "|"}
	case '-':
		tok = token.Token{Type: token.MINUS, Literal: "-"}
	case 0:
		tok = token.Token{Type: token.EOF, Literal: ""}
	case '"':
		tok = token.Token{Type: token.STRING, Literal: l.readQuotedString()}
	default:
		if isLetter(l.char) {
			return l.readIdentifier()
		} else if isDigit(l.char) {
			return l.readPossibleNumber()
		} else {
			tok = token.Token{Type: token.ILLEGAL, Literal: string(l.char)}
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) readIdentifier() token.Token {
	pos := l.pos

	for {
		// Stop if we hit a boundary: space, comma, EOF, or an operator (=, &, |, etc.)
		if l.char == 0 || isWhitespace(l.char) || l.char == ',' || isOperator(l.char) {
			break
		}
		l.readChar()
	}

	literal := string(l.input[pos:l.pos])

	return token.Token{Type: l.lookupIdent(literal), Literal: literal}
}

func (l *Lexer) lookupIdent(ident string) token.TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return token.IDENT
}

func isLetter(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r == '_' || r == '.' || r == '-'
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func (l *Lexer) skipWhitespace() {
	for isWhitespace(l.char) {
		l.readChar()
	}
}

func (l *Lexer) readPossibleNumber() token.Token {
	pos := l.pos
	hasDot := false
	isPureNumber := true

	for {
		if isDigit(l.char) {
			l.readChar()
		} else if l.char == '.' {
			if hasDot { // Second dot? It's definitely not a valid float, treat as string/ident
				isPureNumber = false
			}
			hasDot = true
			l.readChar()
		} else if l.char == ',' || l.char == ' ' || l.char == '\t' || l.char == '\n' || l.char == '\r' || l.char == 0 || isOperator(l.char) {
			// These are the boundaries where we MUST stop
			break
		} else {
			// We hit a dash '-', a letter, or something else.
			// It's still a valid "literal" for a log value, but it's not a "Number" type.
			isPureNumber = false
			l.readChar()
		}
	}

	literal := string(l.input[pos:l.pos])

	if !isPureNumber {
		return token.Token{Type: token.STRING, Literal: literal}
	}
	if hasDot {
		return token.Token{Type: token.DECIMAL, Literal: literal}
	}
	return token.Token{Type: token.INT, Literal: literal}
}

func (l *Lexer) readQuotedString() string {
	// We skip the opening quote by starting at current position + 1
	pos := l.pos + 1

	// TODO: handle the cases with scape quotation marks like \"
	for {
		l.readChar()
		if l.char == '"' || l.char == 0 {
			break
		}
	}

	return string(l.input[pos:l.pos])
}

func isOperator(r rune) bool {
	return r == '=' || r == '~' || r == '!' || r == '&' || r == '|' || r == '(' || r == ')' || r == '<' || r == '>'
}
