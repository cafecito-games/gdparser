package ast

// LiteralKind distinguishes lexical literal forms.
type LiteralKind string

const (
	NumberLiteral LiteralKind = "number"
	StringLiteral LiteralKind = "string"
	BoolLiteral   LiteralKind = "bool"
)

// Literal is a scalar value. Raw retains its exact spelling.
type Literal struct {
	Base
	Kind LiteralKind `json:"literal_kind"`
	Raw  string      `json:"raw"`
}

func (*Literal) node()            {}
func (*Literal) expression()      {}
func (*Literal) initializerItem() {}

// Identifier references a variable, function, type constructor, or enum member.
type Identifier struct {
	Base
	Name string `json:"name"`
}

func (*Identifier) node()            {}
func (*Identifier) expression()      {}
func (*Identifier) initializerItem() {}

// UnaryExpression applies a prefix operator.
type UnaryExpression struct {
	Base
	Operator string     `json:"operator"`
	Operand  Expression `json:"operand"`
}

func (*UnaryExpression) node()            {}
func (*UnaryExpression) expression()      {}
func (*UnaryExpression) initializerItem() {}

// PostfixExpression applies ++ or -- after an operand.
type PostfixExpression struct {
	Base
	Operand  Expression `json:"operand"`
	Operator string     `json:"operator"`
}

func (*PostfixExpression) node()            {}
func (*PostfixExpression) expression()      {}
func (*PostfixExpression) initializerItem() {}

// BinaryExpression applies an infix operator.
type BinaryExpression struct {
	Base
	Left     Expression `json:"left"`
	Operator string     `json:"operator"`
	Right    Expression `json:"right"`
}

func (*BinaryExpression) node()            {}
func (*BinaryExpression) expression()      {}
func (*BinaryExpression) initializerItem() {}

// ConditionalExpression is condition ? then : else.
type ConditionalExpression struct {
	Base
	Condition Expression `json:"condition"`
	Then      Expression `json:"then"`
	Else      Expression `json:"else"`
}

func (*ConditionalExpression) node()            {}
func (*ConditionalExpression) expression()      {}
func (*ConditionalExpression) initializerItem() {}

// AssignmentExpression assigns a value and is right-associative.
type AssignmentExpression struct {
	Base
	Target   Expression `json:"target"`
	Operator string     `json:"operator"`
	Value    Expression `json:"value"`
}

func (*AssignmentExpression) node()            {}
func (*AssignmentExpression) expression()      {}
func (*AssignmentExpression) initializerItem() {}

// CallExpression invokes a function or constructor.
type CallExpression struct {
	Base
	Callee    Expression   `json:"callee"`
	Arguments []Expression `json:"arguments"`
}

func (*CallExpression) node()            {}
func (*CallExpression) expression()      {}
func (*CallExpression) initializerItem() {}

// MemberExpression accesses a field or vector swizzle.
type MemberExpression struct {
	Base
	Object   Expression `json:"object"`
	Property string     `json:"property"`
}

func (*MemberExpression) node()            {}
func (*MemberExpression) expression()      {}
func (*MemberExpression) initializerItem() {}

// IndexExpression indexes a vector, matrix, or array.
type IndexExpression struct {
	Base
	Object Expression `json:"object"`
	Index  Expression `json:"index,omitempty"`
}

func (*IndexExpression) node()            {}
func (*IndexExpression) expression()      {}
func (*IndexExpression) initializerItem() {}

// InitializerList is a brace-delimited aggregate initializer.
type InitializerList struct {
	Base
	Items []InitializerItem `json:"items"`
}

func (*InitializerList) node()            {}
func (*InitializerList) expression()      {}
func (*InitializerList) initializerItem() {}
