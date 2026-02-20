package querier

type And struct {
	Exprs []Expr
}

func (*And) exprNode() {}

type Or struct {
	Exprs []Expr
}

func (*Or) exprNode() {}

type Not struct {
	Expr Expr
}

func (*Not) exprNode() {}
