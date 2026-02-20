package querier

type CmpOp string

const (
	OpEq  CmpOp = "="
	OpNe  CmpOp = "!="
	OpGt  CmpOp = ">"
	OpGte CmpOp = ">="
	OpLt  CmpOp = "<"
	OpLte CmpOp = "<="
	OpIn  CmpOp = "IN"
)

type Comparison struct {
	Left  ValueExpr
	Op    CmpOp
	Right ValueExpr
}

func (*Comparison) exprNode() {}
