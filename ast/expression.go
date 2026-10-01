package ast

import "github.com/cafecito-games/gdparser/token"

// LiteralKind distinguishes the lexical form of a literal.
type LiteralKind string

const (
	IntegerLiteral    LiteralKind = "integer"
	FloatLiteral      LiteralKind = "float"
	StringLiteral     LiteralKind = "string"
	StringNameLiteral LiteralKind = "string_name"
	NodePathLiteral   LiteralKind = "node_path"
	BoolLiteral       LiteralKind = "bool"
	NullLiteral       LiteralKind = "null"
)

// Literal is a scalar literal. Raw contains its exact GDScript spelling.
type Literal struct {
	Base
	Kind LiteralKind `json:"literal_kind"`
	Raw  string      `json:"raw"`
	// Quote is the quote character of a string, string name, or node path
	// literal, and zero for every other kind.
	Quote byte `json:"quote,omitempty"`
	// Triple reports a triple-quoted string literal.
	Triple bool `json:"triple,omitempty"`
	// RawPrefix reports an r-prefixed string literal, in which a backslash has
	// no escaping meaning.
	RawPrefix bool `json:"raw_prefix,omitempty"`
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
	Operator     string     `json:"operator"`
	OperatorSpan token.Span `json:"operator_span,omitempty"`
	Operand      Expression `json:"operand"`
}

func (*UnaryExpression) node()       {}
func (*UnaryExpression) expression() {}

// BinaryExpression applies an infix operator.
type BinaryExpression struct {
	Base
	Left         Expression `json:"left"`
	Operator     string     `json:"operator"`
	OperatorSpan token.Span `json:"operator_span,omitempty"`
	Right        Expression `json:"right"`
}

func (*BinaryExpression) node()       {}
func (*BinaryExpression) expression() {}

// TernaryExpression is GDScript's value if condition else alternative form.
type TernaryExpression struct {
	Base
	Value       Expression `json:"value"`
	IfSpan      token.Span `json:"if_span,omitempty"`
	Condition   Expression `json:"condition"`
	ElseSpan    token.Span `json:"else_span,omitempty"`
	Alternative Expression `json:"alternative"`
}

func (*TernaryExpression) node()       {}
func (*TernaryExpression) expression() {}

// CallExpression calls a function or callable expression.
type CallExpression struct {
	Base
	Callee    Expression   `json:"callee"`
	Arguments []Expression `json:"arguments"`
	// Comments holds the comments written inside the argument list.
	Comments []CollectionComment `json:"comments,omitempty"`
}

func (*CallExpression) node()       {}
func (*CallExpression) expression() {}

// MemberExpression accesses Object.Member.
type MemberExpression struct {
	Base
	Object       Expression `json:"object"`
	Property     string     `json:"property"`
	PropertySpan token.Span `json:"property_span,omitempty"`
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
	// Comments holds the comments written inside the brackets.
	Comments []CollectionComment `json:"comments,omitempty"`
}

func (*ArrayLiteral) node()       {}
func (*ArrayLiteral) expression() {}

// DictionaryEntry is one key/value pair.
type DictionaryEntry struct {
	Key   Expression `json:"key"`
	Value Expression `json:"value"`
	// SeparatorSpan covers the ":" or "=" between the key and the value.
	SeparatorSpan token.Span `json:"separator_span,omitempty"`
}

// DictionaryLiteral is a dictionary expression.
type DictionaryLiteral struct {
	Base
	Entries []DictionaryEntry `json:"entries"`
	// LuaStyle reports the "{key = value}" spelling, where an identifier key
	// stands for the string of the same name. A literal is written in one style
	// throughout, so the field describes all of its entries.
	LuaStyle bool `json:"lua_style,omitempty"`
	// Comments holds the comments written inside the braces.
	Comments []CollectionComment `json:"comments,omitempty"`
}

func (*DictionaryLiteral) node()       {}
func (*DictionaryLiteral) expression() {}

// NodePathExpression represents the $Node/Child and %UniqueNode shorthand.
type NodePathExpression struct {
	Base
	Path       string     `json:"path"`
	PathSpan   token.Span `json:"path_span,omitempty"`
	Unique     bool       `json:"unique,omitempty"`
	PrefixSpan token.Span `json:"prefix_span,omitempty"`
}

func (*NodePathExpression) node()       {}
func (*NodePathExpression) expression() {}

// TypeExpression is a type name used by operators such as as and is.
type TypeExpression struct {
	Base
	Name string `json:"name"`
}

func (*TypeExpression) node()       {}
func (*TypeExpression) expression() {}

// BindingPattern is the "var name" pattern of a match branch, which binds the
// matched value to a new variable. It appears only where a pattern may, which
// is a branch's pattern list and the elements of an array or dictionary
// pattern.
type BindingPattern struct {
	Base
	Name        string     `json:"name"`
	NameSpan    token.Span `json:"name_span,omitempty"`
	KeywordSpan token.Span `json:"keyword_span,omitempty"`
}

func (*BindingPattern) node()       {}
func (*BindingPattern) expression() {}

// WildcardPattern is the "_" pattern of a match branch, which matches any value
// without binding it. Godot gives the lone underscore its own token, so the
// wildcard is a pattern of its own rather than a name.
type WildcardPattern struct {
	Base
}

func (*WildcardPattern) node()       {}
func (*WildcardPattern) expression() {}

// RestPattern is the ".." pattern that lets an array or dictionary pattern
// match a value holding more elements than the pattern lists. It must be the
// last element of the pattern that holds it, and it may not stand on its own as
// a branch's pattern.
type RestPattern struct {
	Base
}

func (*RestPattern) node()       {}
func (*RestPattern) expression() {}

// LambdaExpression is an anonymous function expression.
type LambdaExpression struct {
	Base
	// Name is the optional name of the lambda, which Godot reports in a stack
	// trace. It is empty for an anonymous lambda.
	Name            string      `json:"name,omitempty"`
	NameSpan        token.Span  `json:"name_span,omitempty"`
	Parameters      []Parameter `json:"parameters"`
	ReturnType      string      `json:"return_type,omitempty"`
	ReturnTypeSpan  token.Span  `json:"return_type_span,omitempty"`
	ReturnArrowSpan token.Span  `json:"return_arrow_span,omitempty"`
	KeywordSpan     token.Span  `json:"keyword_span,omitempty"`
	Body            []Statement `json:"body"`
	Inline          bool        `json:"inline,omitempty"`
	// ParameterComments holds the comments written inside the parameter list.
	ParameterComments []CollectionComment `json:"parameter_comments,omitempty"`
}

func (*LambdaExpression) node()       {}
func (*LambdaExpression) expression() {}
