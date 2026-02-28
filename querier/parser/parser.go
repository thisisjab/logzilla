package parser

import (
	"fmt"
	"strconv"
	"time"

	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/lexer"
	"github.com/thisisjab/logzilla/querier/token"
)

type Parser struct {
	l         *lexer.Lexer
	curToken  token.Token
	peekToken token.Token
	errors    []error
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l: l,
	}

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseQuery() *ast.Query {
	q := &ast.Query{}

	isParsingFilterSection := false

	for p.curToken.Type != token.EOF {
		if p.curToken.Type == token.COLON {
			isParsingFilterSection = true
		}

		if isParsingFilterSection {
			// TODO
			// p.parseFilterStatement(q)
		} else {
			p.parseControlStatement(q)
		}

		p.nextToken()
	}

	return q
}

func (p *Parser) parseStatement(q *ast.Query) { //nolint:unused
	// TODO
}

func (p *Parser) parseControlStatement(q *ast.Query) {
	switch p.curToken.Literal {
	case "timestamp":
		p.parseTimestamp(q)
	case "limit":
		p.parseLimit(q)
	case "cursor":
		p.parseCursor(q)
	case "sort":
	// TODO
	default:
		panic("unexpected token")
	}
}

func (p *Parser) parseTimestamp(q *ast.Query) {
	if p.peekToken.Type != token.EQUAL {
		p.addPeekError(token.EQUAL)
	}

	p.nextToken()

	if p.peekToken.Type != token.STRING {
		p.addPeekError(token.STRING)
	}

	p.nextToken()

	// Parse start
	start, err := parseDatetime(p.curToken.Literal)
	if err != nil {
		panic(err)
	}

	q.Start = start

	if p.peekToken.Type != token.COMMA {
		return
	}

	p.nextToken()

	if p.peekToken.Type != token.STRING {
		p.addPeekError(token.STRING)
	}

	p.nextToken()

	end, err := parseDatetime(p.curToken.Literal)
	if err != nil {
		panic(err)
	}

	q.End = end

	p.nextToken()
}

func parseDatetime(v string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,          // Handles 2000-10-10T12:20:23Z or with offsets
		"2006-01-02T15:04:05", // 2000-10-10T12:20:23
		"2006-01-02T15:04",    // 2000-10-10T12:20
		"2006-01-02",          // 2000-10-10
	}

	var t time.Time
	var err error

	for _, layout := range layouts {
		t, err = time.Parse(layout, v)
		if err == nil {
			return t, nil
		}
	}

	// If no layouts matched, return the last error or a custom one
	return time.Time{}, fmt.Errorf("failed to parse datetime '%s': %w", v, err)
}

func (p *Parser) parseLimit(q *ast.Query) {
	if p.peekToken.Type != token.EQUAL {
		p.addPeekError(token.EQUAL)
	}

	p.nextToken()

	if p.peekToken.Type != token.INT {
		p.addPeekError(token.INT)
	}

	p.nextToken()

	limit, err := strconv.Atoi(p.curToken.Literal)
	if err != nil {
		p.addError(fmt.Errorf("cannot parse limit value: `%s` is not a valid integer.", p.curToken.Literal))
	}

	q.Limit = limit

	p.nextToken()
}

func (p *Parser) parseCursor(q *ast.Query) {
	if p.peekToken.Type != token.EQUAL {
		p.addPeekError(token.EQUAL)
	}

	p.nextToken()

	if p.peekToken.Type != token.STRING {
		p.addPeekError(token.STRING)
	}

	p.nextToken()

	cursor := p.curToken.Literal

	q.Cursor = cursor

	p.nextToken()
}

func (p *Parser) addError(err error) {
	p.errors = append(p.errors, err)
}

func (p *Parser) addPeekError(expected token.TokenType) {
	p.addError(fmt.Errorf("expected token of type %v, but got %v (literal=`%s`)", expected, p.peekToken.Type, p.peekToken.Literal))
}
