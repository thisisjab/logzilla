package querier

// QueryNode is the interface that all nodes in the query tree must implement.
// It uses a private marker method to ensure only types defined in this
// package can be used as nodes, creating a controlled "sum type" behavior.
type QueryNode interface {
	queryNode()
}

// AndNode represents a logical conjunction.
// It is satisfied only if all of its Children evaluate to true.
// Drivers should typically join children with a logical "AND".
type AndNode struct {
	Children []QueryNode
}

func (n AndNode) queryNode() {}

// OrNode represents a logical disjunction.
// It is satisfied if at least one of its Children evaluates to true.
// Drivers should typically join children with a logical "OR".
type OrNode struct {
	Children []QueryNode
}

func (n OrNode) queryNode() {}

// NotNode represents a logical negation.
// It inverts the boolean result of its single Child node.
type NotNode struct {
	Child QueryNode
}

func (n NotNode) queryNode() {}

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
	// OperatorIn checks if the field is in the list of values.
	OperatorIn
)

// ComparisonNode is a leaf node in the query tree.
// It represents a concrete filter expression against a specific field.
type ComparisonNode struct {
	// FieldName is the identifier for the log field.
	// This can be a top-level field (e.g., "level") or a
	// path into the metadata JSON (e.g., "metadata.user_id").
	FieldName string

	// Value is the literal data to compare against.
	// Drivers are responsible for handling type casting (e.g., string vs. int).
	Value any

	// Operator defines the relationship between the FieldName and the Value.
	Operator ComparisonOperator
}

func (n ComparisonNode) queryNode() {}
