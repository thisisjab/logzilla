package parser

import "github.com/thisisjab/logzilla/querier/token"

const (
	LOWEST int = iota + 1
	OR
	AND
)

var precedenceMap = map[token.TokenType]int{
	token.OR:  OR,
	token.AND: AND,
}
