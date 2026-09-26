package ast

// LiteralKind distinguishes the lexical form of a literal.
type LiteralKind string

const (
	IntegerLiteral LiteralKind = "integer"
	FloatLiteral   LiteralKind = "float"
	StringLiteral  LiteralKind = "string"
	BoolLiteral    LiteralKind = "bool"
	NullLiteral    LiteralKind = "null"
)

// Literal is a scalar literal. Raw contains its exact GDScript spelling.
type Literal struct {
	Base
	Kind LiteralKind `json:"literal_kind"`
	Raw  string      `json:"raw"`
}

func (*Literal) node()       {}
func (*Literal) expression() {}

// Identifier references a local, member, type, or global name.
type Identifier struct {
	Base
	Name string `json:"name"`
}

func (*Identifier) node()       {}
func (*Identifier) expression() {}

// UnaryExpression applies a prefix operator.
type UnaryExpression struct {
	Base
	Operator string     `json:"operator"`
	Operand  Expression `json:"operand"`
}

func (*UnaryExpression) node()       {}
func (*UnaryExpression) expression() {}

// BinaryExpression applies an infix operator.
type BinaryExpression struct {
	Base
	Left     Expression `json:"left"`
	Operator string     `json:"operator"`
	Right    Expression `json:"right"`
}

func (*BinaryExpression) node()       {}
func (*BinaryExpression) expression() {}

// TernaryExpression is GDScript's value if condition else alternative form.
type TernaryExpression struct {
	Base
	Value       Expression `json:"value"`
	Condition   Expression `json:"condition"`
	Alternative Expression `json:"alternative"`
}

func (*TernaryExpression) node()       {}
func (*TernaryExpression) expression() {}

// CallExpression calls a function or callable expression.
type CallExpression struct {
	Base
	Callee    Expression   `json:"callee"`
	Arguments []Expression `json:"arguments"`
}

func (*CallExpression) node()       {}
func (*CallExpression) expression() {}

// MemberExpression accesses Object.Member.
type MemberExpression struct {
	Base
	Object   Expression `json:"object"`
	Property string     `json:"property"`
}

func (*MemberExpression) node()       {}
func (*MemberExpression) expression() {}

// SubscriptExpression accesses Object[Index].
type SubscriptExpression struct {
	Base
	Object Expression `json:"object"`
	Index  Expression `json:"index"`
}

func (*SubscriptExpression) node()       {}
func (*SubscriptExpression) expression() {}

// ArrayLiteral is an array expression.
type ArrayLiteral struct {
	Base
	Elements []Expression `json:"elements"`
}

func (*ArrayLiteral) node()       {}
func (*ArrayLiteral) expression() {}

// DictionaryEntry is one key/value pair.
type DictionaryEntry struct {
	Key   Expression `json:"key"`
	Value Expression `json:"value"`
}

// DictionaryLiteral is a dictionary expression.
type DictionaryLiteral struct {
	Base
	Entries []DictionaryEntry `json:"entries"`
}

func (*DictionaryLiteral) node()       {}
func (*DictionaryLiteral) expression() {}

// NodePathExpression represents the $Node/Child and %UniqueNode shorthand.
type NodePathExpression struct {
	Base
	Path   string `json:"path"`
	Unique bool   `json:"unique,omitempty"`
}

func (*NodePathExpression) node()       {}
func (*NodePathExpression) expression() {}
