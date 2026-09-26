package ast

// NullLiteral is the null Variant value.
type NullLiteral struct{ Base }

func (*NullLiteral) node()       {}
func (*NullLiteral) expression() {}

// BoolLiteral is a boolean Variant value.
type BoolLiteral struct {
	Base
	Value bool `json:"value"`
}

func (*BoolLiteral) node()       {}
func (*BoolLiteral) expression() {}

// IntegerLiteral is an integer Variant value. Raw is retained so very large
// integers and non-decimal spellings can be parsed without loss.
type IntegerLiteral struct {
	Base
	Raw string `json:"raw"`
}

func (*IntegerLiteral) node()       {}
func (*IntegerLiteral) expression() {}

// FloatLiteral is a floating-point Variant value. Raw is retained to preserve
// values such as nan and inf without loss.
type FloatLiteral struct {
	Base
	Raw string `json:"raw"`
}

func (*FloatLiteral) node()       {}
func (*FloatLiteral) expression() {}

// StringLiteral is a decoded string value. Prefix is empty for String, "&"
// for StringName, or "^" for NodePath syntax.
type StringLiteral struct {
	Base
	Value  string `json:"value"`
	Prefix string `json:"prefix,omitempty"`
}

func (*StringLiteral) node()       {}
func (*StringLiteral) expression() {}

// Identifier is a symbolic Variant value such as an enum constant.
type Identifier struct {
	Base
	Name string `json:"name"`
}

func (*Identifier) node()       {}
func (*Identifier) expression() {}

// UnaryExpression applies a leading plus or minus to a scalar value.
type UnaryExpression struct {
	Base
	Operator string     `json:"operator"`
	Operand  Expression `json:"operand"`
}

func (*UnaryExpression) node()       {}
func (*UnaryExpression) expression() {}

// ArrayElement is one value in an array. The wrapper allows array comments to
// remain interleaved with values in source order.
type ArrayElement struct {
	Base
	Value Expression `json:"value"`
}

func (*ArrayElement) node()      {}
func (*ArrayElement) arrayItem() {}

// ArrayLiteral is an ordered Variant array whose items include comments.
type ArrayLiteral struct {
	Base
	Items []ArrayItem `json:"items"`
}

func (*ArrayLiteral) node()       {}
func (*ArrayLiteral) expression() {}

// DictionaryEntry is one typed dictionary key/value node.
type DictionaryEntry struct {
	Base
	Key           Expression `json:"key"`
	InfixComments []*Comment `json:"infix_comments,omitempty"`
	Value         Expression `json:"value"`
}

func (*DictionaryEntry) node()           {}
func (*DictionaryEntry) dictionaryItem() {}

// DictionaryLiteral is an ordered Variant dictionary whose items include
// comments.
type DictionaryLiteral struct {
	Base
	Items []DictionaryItem `json:"items"`
}

func (*DictionaryLiteral) node()       {}
func (*DictionaryLiteral) expression() {}

// ConstructorArgument is one positional value or key/value argument. Named
// arguments are used by Godot's Object(Type,"property":value,...) encoding.
type ConstructorArgument struct {
	Base
	Key           Expression `json:"key,omitempty"`
	InfixComments []*Comment `json:"infix_comments,omitempty"`
	Value         Expression `json:"value"`
}

func (*ConstructorArgument) node()            {}
func (*ConstructorArgument) constructorItem() {}

// ConstructorCall is a built-in Variant constructor such as Vector2(...) or
// PackedStringArray(...).
type ConstructorCall struct {
	Base
	Name  string            `json:"name"`
	Items []ConstructorItem `json:"items"`
}

func (*ConstructorCall) node()       {}
func (*ConstructorCall) expression() {}

// TypeRef is a recursive Variant container type such as String,
// Array[Vector2], or Dictionary[String, Array[int]].
type TypeRef struct {
	Base
	Name      string     `json:"name"`
	Arguments []*TypeRef `json:"arguments,omitempty"`
}

func (*TypeRef) node() {}

// TypedArrayLiteral is Godot's Array[ElementType]([values]) serialization.
type TypedArrayLiteral struct {
	Base
	ElementType *TypeRef      `json:"element_type"`
	Value       *ArrayLiteral `json:"value"`
}

func (*TypedArrayLiteral) node()       {}
func (*TypedArrayLiteral) expression() {}

// TypedDictionaryLiteral is Godot's
// Dictionary[KeyType, ValueType]({entries}) serialization.
type TypedDictionaryLiteral struct {
	Base
	KeyType   *TypeRef           `json:"key_type"`
	ValueType *TypeRef           `json:"value_type"`
	Value     *DictionaryLiteral `json:"value"`
}

func (*TypedDictionaryLiteral) node()       {}
func (*TypedDictionaryLiteral) expression() {}
