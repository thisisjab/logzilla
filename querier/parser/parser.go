package parser

import (
	"fmt"
	"strconv"

	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/lexer"
	"github.com/thisisjab/logzilla/querier/token"
)

type (
	nudParseFn func() ast.Term
	ledParseFn func(ast.Term) ast.Term
)

type Parser struct {
	l         *lexer.Lexer
	curToken  token.Token
	peekToken token.Token
	errors    []error

	nudParseFns map[token.TokenType]nudParseFn
	ledParseFns map[token.TokenType]ledParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:           l,
		errors:      make([]error, 0),
		nudParseFns: make(map[token.TokenType]nudParseFn),
		ledParseFns: make(map[token.TokenType]ledParseFn),
	}

	registerHandlers(p)

	p.nextToken()
	p.nextToken()

	return p
}

func registerHandlers(p *Parser) {
	p.registerNud(token.IDENT, p.parseIdentifier)
}

func (p *Parser) registerNud(tokenType token.TokenType, fn nudParseFn) {
	p.nudParseFns[tokenType] = fn
}

func (p *Parser) registerLed(tokenType token.TokenType, fn ledParseFn) { //nolint:unused
	p.ledParseFns[tokenType] = fn
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
			p.parseFilterStatement(q)
		} else {
			p.parseControlStatement(q)
		}

		p.nextToken()
	}

	return q
}

func (p *Parser) parseFilterStatement(q *ast.Query) {
	q.Root = p.parseStatement(LOWEST)
}

func (p *Parser) parseStatement(precedence int) ast.Term {
	nud, exists := p.nudParseFns[p.curToken.Type]
	if !exists {
		panic(fmt.Errorf("no nud parse function for token type: `%v`", p.curToken.Type))
	}

	leftExp := nud()

	return leftExp
}

func (p *Parser) parseControlStatement(q *ast.Query) {
	// TODO: Handle illegal token

	if p.curToken.Type == token.EOF {
		return
	}

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
	if !p.peekTokenTypeIs(token.EQUAL) {
		return
	}

	p.nextToken()

	if !p.peekTokenTypeIs(token.STRING) {
		return
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

	if !p.peekTokenTypeIs(token.STRING) {
		return
	}

	p.nextToken()

	end, err := parseDatetime(p.curToken.Literal)
	if err != nil {
		panic(err)
	}

	q.End = end

	p.nextToken()
}

func (p *Parser) parseLimit(q *ast.Query) {
	if !p.peekTokenTypeIs(token.EQUAL) {
		return
	}

	p.nextToken()

	if !p.peekTokenTypeIs(token.INT) {
		return
	}

	p.nextToken()

	limit, err := strconv.Atoi(p.curToken.Literal)
	if err != nil {
		p.addError(fmt.Errorf("cannot parse limit value: `%s` is not a valid integer.", p.curToken.Literal))
		return
	}

	q.Limit = limit

	p.nextToken()
}

func (p *Parser) parseCursor(q *ast.Query) {
	if !p.peekTokenTypeIs(token.EQUAL) {
		return
	}

	p.nextToken()

	if !p.peekTokenTypeIs(token.STRING, token.IDENT) {
		return
	}

	p.nextToken()

	cursor := p.curToken.Literal

	q.Cursor = cursor

	p.nextToken()
}

func (p *Parser) parseIdentifier() ast.Term {
	return ast.AndTerm{}
}
