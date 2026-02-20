package querier

type Query struct {
	Expr   Expr
	Limit  int
	Offset int
	Sort   []SortField
}

type SortField struct {
	Name         string
	IsDescending bool
}

// Expr represents a boolean expression (evaluates to true/false)
type Expr interface {
	exprNode()
}
