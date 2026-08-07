package parser

import "github.com/thisisjab/logzilla/querier/token"

const (
	LOWEST int = iota
	OR
	AND
)

var precedenceMap = map[token.TokenType]int{
	token.OR:  OR,
	token.AND: AND,
}
