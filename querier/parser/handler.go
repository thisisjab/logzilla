package parser

import (
	"fmt"

	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/token"
)

type (
	nudParseFn func() ast.Term
	ledParseFn func(left ast.Term, precedence int) ast.Term
)

// parseIdentifier is a nud function that parses a possible comparison term (i.e. level=info).
func (p *Parser) parseIdentifier() ast.Term {
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
		// TODO: add better error handling
		panic(fmt.Errorf("expected an operator after comparison field name, but got `%s (%s)`", p.curToken.Literal, p.curToken.Type.String()))
	}

	p.nextToken()

	n.Values = p.parseValues()

	return n
}

func (p *Parser) parseAndCondition(left ast.Term, precedence int) ast.Term {
	t := ast.AndTerm{
		Left: left,
	}

	p.nextToken()

	t.Right = p.parseStatement(precedence)

	return t
}

func (p *Parser) parseOrCondition(left ast.Term, precedence int) ast.Term {
	t := ast.OrTerm{
		Left: left,
	}

	p.nextToken()

	t.Right = p.parseStatement(precedence)

	return t
}

func (p *Parser) parseLParen() ast.Term {
	p.nextToken()

	exp := p.parseStatement(LOWEST)

	if p.peekToken.Type != token.RPAREN {
		// TODO: add error handling
		panic(fmt.Errorf("expected RPAREN, but got %s (%s)", p.peekToken.Literal, p.peekToken.Type))
	}

	p.nextToken()

	return exp
}
