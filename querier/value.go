package querier

type ValueExpr interface {
	valueNode()
}

type Field struct {
	Name string
}

func (*Field) valueNode() {}

type Literal struct {
	Value any
}

func (*Literal) valueNode() {}
