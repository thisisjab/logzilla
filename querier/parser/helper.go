package parser

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/thisisjab/logzilla/querier/ast"
	"github.com/thisisjab/logzilla/querier/token"
)

func (p *Parser) addError(err error) {
	p.errors = append(p.errors, err)
}

func (p *Parser) addPeekError(expectedTokens ...token.TokenType) {
	values := make([]string, len(expectedTokens))

	for i := range expectedTokens {
		values[i] = fmt.Sprintf("`%s`", expectedTokens[i])
	}

	p.addError(fmt.Errorf("expected token of type %v, but got %v (literal=`%s`)", strings.Join(values, ", "), p.peekToken.Type, p.peekToken.Literal))
}

func (p *Parser) peekTokenTypeIs(expected ...token.TokenType) bool {
	if slices.Contains(expected, p.peekToken.Type) {
		return true
	}

	p.addPeekError(expected...)
	return false
}

func (p *Parser) currentTokenTypeIs(expected ...token.TokenType) bool { //nolint:unused
	return slices.Contains(expected, p.curToken.Type)
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

func (p *Parser) parseSingleSortField() (ast.SortField, error) {
	s := ast.SortField{}

	switch p.curToken.Type {
	case token.IDENT, token.STRING:
		s.Name = p.curToken.Literal
		s.IsDescending = false
	case token.MINUS:
		if !p.peekTokenTypeIs(token.IDENT, token.STRING) {
			return s, fmt.Errorf("expected a literal after a descending sort field, got `%s`", p.peekToken.Type.String())
		}

		p.nextToken()

		s.Name = p.curToken.Literal
		s.IsDescending = true
	default:
		return s, fmt.Errorf("unexpected token of type `%s`", p.peekToken.Type.String())
	}

	return s, nil
}

func (p *Parser) parseValues() []any {
	values := make([]any, 0)

	for p.currentTokenTypeIs(token.STRING, token.IDENT, token.INT, token.DECIMAL, token.NULL, token.TRUE, token.FALSE) {
		switch p.curToken.Type {
		case token.STRING, token.IDENT:
			values = append(values, p.curToken.Literal)

		case token.INT:
			num, err := strconv.Atoi(p.curToken.Literal)
			if err != nil {
				panic(fmt.Errorf("cannot parse %s to a valid int", p.curToken.Literal))
			}

			values = append(values, num)

		case token.DECIMAL:
			num, err := strconv.ParseFloat(p.curToken.Literal, 32)
			if err != nil {
				panic(fmt.Errorf("cannot parse %s to a valid decimal", p.curToken.Literal))
			}

			values = append(values, num)

		case token.TRUE, token.FALSE:
			values = append(values, p.curToken.Type == token.TRUE)

		case token.NULL:
			values = append(values, nil)

		default:
			p.addPeekError(token.STRING, token.IDENT, token.INT, token.DECIMAL, token.NULL, token.TRUE, token.FALSE)
		}

		if p.peekToken.Type != token.COMMA {
			return values
		}

		p.nextToken() // Read comma
		p.nextToken() // Read next value
	}

	return values
}
