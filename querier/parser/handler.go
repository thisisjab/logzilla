package parser

import (
	"fmt"

	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/token"
)

type (
	nudParseFn func() (ast.Term, error)
	ledParseFn func(left ast.Term, precedence int) (ast.Term, error)
)

// parseIdentifier is a nud function that parses a possible comparison term (i.e. level=info).
func (p *Parser) parseIdentifier() (ast.Term, error) {
	n := ast.ComparisonTerm{
		FieldName: p.curToken.Literal,
	}

	p.nextToken()

	switch p.curToken.Type {
	case token.EQUAL:
		n.Operator = ast.OperatorEq
	case token.NOTEQUAL:
		n.Operator = ast.OperatorNe
	case token.GREATER:
		n.Operator = ast.OperatorGt
	case token.GREATEREQUAL:
		n.Operator = ast.OperatorGte
	case token.LESS:
		n.Operator = ast.OperatorLt
	case token.LESSEQUAL:
		n.Operator = ast.OperatorLte
	case token.TILDE:
		n.Operator = ast.OperatorILike
	default:
		return nil, fmt.Errorf("expected an operator after comparison field name (%s), but got `%s (%s)`", n.FieldName, p.curToken.Literal, p.curToken.Type.String())
	}

	p.nextToken()

	n.Values = p.parseValues()

	return n, nil
}

func (p *Parser) parseAndCondition(left ast.Term, precedence int) (ast.Term, error) {
	t := ast.AndTerm{
		Left: left,
	}

	p.nextToken()

	right, err := p.parseStatement(precedence)
	if err != nil {
		return nil, fmt.Errorf("cannot parse right side of `&` operator: %w", err)
	}

	t.Right = right

	return t, nil
}

func (p *Parser) parseOrCondition(left ast.Term, precedence int) (ast.Term, error) {
	t := ast.OrTerm{
		Left: left,
	}

	p.nextToken()

	right, err := p.parseStatement(precedence)
	if err != nil {
		return nil, fmt.Errorf("cannot parse right side of `|` operator: %w", err)
	}

	t.Right = right

	return t, nil
}

func (p *Parser) parseLParen() (ast.Term, error) {
	p.nextToken()

	exp, err := p.parseStatement(LOWEST)
	if err != nil {
		return nil, fmt.Errorf("cannot parse statement after `(`: %w", err)
	}

	if p.peekToken.Type != token.RPAREN {
		return nil, fmt.Errorf("expected closing `)`, but got %s (%s)", p.peekToken.Literal, p.peekToken.Type)
	}

	p.nextToken()

	return exp, nil
}

func (p *Parser) parseNot() (ast.Term, error) {
	p.nextToken()

	t, err := p.parseStatement(LOWEST)
	if err != nil {
		return nil, fmt.Errorf("cannot parse right side of `!` operator: %w", err)
	}

	return ast.NotNode{
		Term: t,
	}, nil
}
