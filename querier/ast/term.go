package ast

import (
	"fmt"
	"reflect"
	"strings"
)

// Term is the interface that all nodes in the query tree must implement.
// It uses a private marker method to ensure only types defined in this
// package can be used as nodes, creating a controlled "sum type" behavior.
type Term interface {
	term()
	String() string
}

// AndTerm represents a logical conjunction.
// It is satisfied only if all of its Children evaluate to true.
// Drivers should typically join children with a logical "AND".
type AndTerm struct {
	Left  Term
	Right Term
}

func (n AndTerm) term() {}

func (n AndTerm) String() string {
	return fmt.Sprintf("(%s & %s)", n.Left, n.Right)
}

// OrTerm represents a logical disjunction.
// It is satisfied if at least one of its Children evaluates to true.
// Drivers should typically join children with a logical "OR".
type OrTerm struct {
	Left  Term
	Right Term
}

func (n OrTerm) term() {}

func (n OrTerm) String() string {
	return fmt.Sprintf("(%s | %s)", n.Left, n.Right)
}

// NotNode represents a logical negation.
// It inverts the boolean result of its single Child node.
type NotNode struct {
	Term Term
}

func (n NotNode) term() {}

func (n NotNode) String() string {
	return fmt.Sprintf("!(%s)", n.Term)
}

// ComparisonOperator defines the type of comparison to be performed
// in an expression (e.g., equality, greater than).
type ComparisonOperator uint8

const (
	// OperatorEq checks if the field is equal to the value.
	OperatorEq ComparisonOperator = iota
	// OperatorNe checks if the field is not equal to the value.
	OperatorNe
	// OperatorGt checks if the field is strictly greater than the value.
	OperatorGt
	// OperatorLt checks if the field is strictly less than the value.
	OperatorLt
	// OperatorGte checks if the field is greater than or equal to the value.
	OperatorGte
	// OperatorLte checks if the field is less than or equal to the value.
	OperatorLte
	// OperatorLike checks if the field is like the value.
	OperatorLike
	// OperatorILike checks if the field is like the value, ignoring case.
	OperatorILike
)

func (o ComparisonOperator) String() string {
	return map[ComparisonOperator]string{
		OperatorEq:    "=",
		OperatorNe:    "!=",
		OperatorGt:    ">",
		OperatorLt:    "<",
		OperatorGte:   ">=",
		OperatorLte:   "<=",
		OperatorILike: "~",
	}[o]
}

// ComparisonTerm is a leaf node in the query tree.
// It represents a concrete filter expression against a specific field.
type ComparisonTerm struct {
	// FieldName is the identifier for the log field.
	// This can be a top-level field (e.g., "level") or a
	// path into the metadata JSON (e.g., "metadata.user_id").
	FieldName string

	// Values is the data (string, int, float, boolean, or list of these primitive types) to compare against.
	Values []any

	// Operator defines the relationship between the FieldName and the Value.
	Operator ComparisonOperator
}

func (n ComparisonTerm) term() {}

func (n ComparisonTerm) String() string {
	values := make([]string, len(n.Values))
	for i := range n.Values {
		if _, ok := n.Values[i].(string); ok {
			values[i] = fmt.Sprintf("\"%s\"", n.Values[i])
			continue
		}

		values[i] = fmt.Sprintf("%v", n.Values[i])
	}

	return fmt.Sprintf("(%s %s %s)", n.FieldName, n.Operator.String(), strings.Join(values, ", "))
}

func (q *Query) Equal(other *Query) bool {
	if q == nil || other == nil {
		return q == other
	}

	// 1. Basic Metadata Comparison
	if q.Limit != other.Limit ||
		q.Cursor != other.Cursor ||
		!q.Start.Equal(other.Start) ||
		!q.End.Equal(other.End) {
		return false
	}

	// 2. Sort Fields Comparison
	if len(q.Sort) != len(other.Sort) {
		return false
	}
	for i := range q.Sort {
		if q.Sort[i] != other.Sort[i] {
			return false
		}
	}

	// 3. The Query Tree (Deep Interface Comparison)
	return nodesEqual(q.Root, other.Root)
}

func nodesEqual(a, b Term) bool {
	if a == nil || b == nil {
		return a == b
	}

	switch nodeA := a.(type) {
	case *ComparisonTerm:
		nodeB, ok := b.(*ComparisonTerm)
		return ok &&
			nodeA.FieldName == nodeB.FieldName &&
			nodeA.Operator == nodeB.Operator &&
			reflect.DeepEqual(nodeA.Values, nodeB.Values)

	case *AndTerm:
		nodeB, ok := b.(*AndTerm)
		return ok && nodesEqual(nodeA.Left, nodeB.Left) && nodesEqual(nodeA.Right, nodeB.Right)

	case *OrTerm:
		nodeB, ok := b.(*OrTerm)
		return ok && nodesEqual(nodeA.Left, nodeB.Left) && nodesEqual(nodeA.Right, nodeB.Right)

	case *NotNode:
		nodeB, ok := b.(*NotNode)
		return ok && nodesEqual(nodeA.Term, nodeB.Term)

	default:
		return reflect.DeepEqual(a, b)
	}
}
