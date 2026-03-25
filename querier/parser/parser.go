package parser

import (
	"fmt"
	"strconv"

	"github.com/thisisjab/logzilla/pkg/fault"
	"github.com/thisisjab/logzilla/pkg/helper"
	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/lexer"
	"github.com/thisisjab/logzilla/querier/token"
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
	p.registerNud(token.LPAREN, p.parseLParen)
	p.registerNud(token.NOT, p.parseNot)
	p.registerNud(token.EOF, p.parseEOF)

	p.registerLed(token.AND, p.parseAndCondition)
	p.registerLed(token.OR, p.parseOrCondition)
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

func (p *Parser) ParseQuery() (*ast.Query, error) {
	q := &ast.Query{}

	isParsingFilterSection := false

	for p.curToken.Type != token.EOF {
		if p.curToken.Type == token.ILLEGAL {
			return nil, fault.New(fault.BadInputCode, "Illegal token.").WithMetadata(fault.FieldErrorsMetadata{"query": []string{fmt.Sprintf("illegal token: %s", p.curToken.Literal)}})
		}

		if p.curToken.Type == token.COLON {
			isParsingFilterSection = true
			p.nextToken()
		}

		if isParsingFilterSection {
			err := p.parseFilterStatement(q)
			if err != nil {
				return nil, fault.New(fault.BadInputCode, "").WithMetadata(fault.FieldErrorsMetadata{"query": []string{err.Error()}})
			}
		} else {
			err := p.parseControlStatement(q)
			if err != nil {
				return nil, fault.New(fault.BadInputCode, "").WithMetadata(fault.FieldErrorsMetadata{"query": []string{err.Error()}})
			}
		}

		p.nextToken()
	}

	return q, nil
}

func (p *Parser) parseFilterStatement(q *ast.Query) error {
	root, err := p.parseStatement(LOWEST)
	if err != nil {
		return err
	}

	q.Root = root

	return nil
}

func (p *Parser) parseStatement(precedence int) (ast.Term, error) {
	nud, exists := p.nudParseFns[p.curToken.Type]
	if !exists {
		panic(fmt.Errorf("no nud parse function for token type: `%v`", p.curToken.Type))
	}

	leftExp, err := nud()
	if err != nil {
		return nil, fmt.Errorf("cannot parse token: %w", err)
	}

	for precedenceMap[p.peekToken.Type] > precedence {
		p.nextToken()
		led, exists := p.ledParseFns[p.curToken.Type]
		if !exists {
			panic(fmt.Errorf("no led parse function for token type: `%v`", p.curToken.Type))
		}

		leftExp, err = led(leftExp, precedence)
		if err != nil {
			return nil, fmt.Errorf("cannot parse token: %w", err)
		}
	}

	return leftExp, nil
}

func (p *Parser) parseControlStatement(q *ast.Query) error {
	if p.curToken.Type == token.EOF {
		return nil
	}

	switch p.curToken.Literal {
	case "timestamp":
		return p.parseTimestamp(q)
	case "limit":
		return p.parseLimit(q)
	case "cursor":
		return p.parseCursor(q)
	case "sort":
		return p.parseSort(q)
	default:
		return fmt.Errorf("unexpected token of type `%s (value=%s)`", p.curToken.Type.String(), p.curToken.Literal)
	}
}

func (p *Parser) parseTimestamp(q *ast.Query) error {
	if !p.peekTokenTypeIs(token.EQUAL) {
		return fmt.Errorf("error when parsing `timestamp`: %w", p.createPeekError(token.EQUAL))
	}

	p.nextToken()

	if !p.peekTokenTypeIs(token.STRING) {
		return fmt.Errorf("error when parsing `timestamp`: %w", p.createPeekError(token.STRING))
	}

	p.nextToken()

	// Parse start
	start, err := helper.ParseDatetime(p.curToken.Literal)
	if err != nil {
		return fmt.Errorf("cannot parse `start` for timestamp: %w", err)
	}

	q.Start = start

	if p.peekToken.Type != token.COMMA {
		// There's no value for `end`, so let's return
		return nil
	}

	p.nextToken()

	if !p.peekTokenTypeIs(token.STRING) {
		return fmt.Errorf("error when parsing end of `timestamp`: %w", p.createPeekError(token.STRING))
	}

	p.nextToken()

	end, err := helper.ParseDatetime(p.curToken.Literal)
	if err != nil {
		return fmt.Errorf("cannot parse `end` for timestamp: %w", err)
	}

	q.End = end

	return nil
}

func (p *Parser) parseLimit(q *ast.Query) error {
	if !p.peekTokenTypeIs(token.EQUAL) {
		return fmt.Errorf("error when parsing `limit`: %w", p.createPeekError(token.EQUAL))
	}

	p.nextToken()

	if !p.peekTokenTypeIs(token.INT) {
		return fmt.Errorf("error when parsing `limit`: %w", p.createPeekError(token.INT))
	}

	p.nextToken()

	limit, err := strconv.Atoi(p.curToken.Literal)
	if err != nil {
		return fmt.Errorf("cannot parse limit value: `%s` is not a valid integer.", p.curToken.Literal)
	}

	q.Limit = limit

	return nil
}

func (p *Parser) parseCursor(q *ast.Query) error {
	if !p.peekTokenTypeIs(token.EQUAL) {
		return fmt.Errorf("error when parsing `cursor`: %w", p.createPeekError(token.EQUAL))
	}

	p.nextToken()

	if !p.peekTokenTypeIs(token.STRING, token.IDENT) {
		return fmt.Errorf("error when parsing `cursor`: %w", p.createPeekError(token.STRING, token.IDENT))
	}

	p.nextToken()

	q.Cursor = p.curToken.Literal

	return nil
}

func (p *Parser) parseSort(q *ast.Query) error {
	if !p.peekTokenTypeIs(token.EQUAL) {
		return fmt.Errorf("error when parsing `sort`: %w", p.createPeekError(token.EQUAL))
	}

	p.nextToken()

	if q.Sort == nil {
		q.Sort = make([]ast.SortField, 0)
	}

	for p.peekTokenTypeIs(token.MINUS, token.IDENT, token.STRING) {
		p.nextToken()

		f, err := p.parseSingleSortField()
		if err != nil {
			return fmt.Errorf("error when parsing `sort`: %w", err)
		}

		q.Sort = append(q.Sort, f)

		if p.peekToken.Type != token.COMMA {
			return nil
		}

		p.nextToken()
	}

	return nil
}
