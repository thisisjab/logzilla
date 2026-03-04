package parser

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
