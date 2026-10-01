package ast

import "github.com/cafecito-games/gdparser/token"

// ExpressionStatement evaluates an expression for its side effects.
type ExpressionStatement struct {
	Base
	Trivia
	Expression Expression `json:"expression"`
}

func (*ExpressionStatement) node()      {}
func (*ExpressionStatement) statement() {}

// VariableDeclaration declares var or const.
type VariableDeclaration struct {
	Base
	Trivia
	Annotations  []*Annotation `json:"annotations,omitempty"`
	Name         string        `json:"name"`
	NameSpan     token.Span    `json:"name_span,omitempty"`
	Type         string        `json:"type,omitempty"`
	TypeSpan     token.Span    `json:"type_span,omitempty"`
	Value        Expression    `json:"value,omitempty"`
	OperatorSpan token.Span    `json:"operator_span,omitempty"`
	Constant     bool          `json:"constant,omitempty"`
	Inferred     bool          `json:"inferred,omitempty"`
	Static       bool          `json:"static,omitempty"`
	StaticSpan   token.Span    `json:"static_span,omitempty"`
	KeywordSpan  token.Span    `json:"keyword_span,omitempty"`
	// AccessorComments holds the comments written inside the property accessor
	// block but outside an accessor body, anchored to the number of accessors
	// before them. A comment that ended the block's colon line is anchored to
	// the first accessor and marked trailing.
	AccessorComments  []CollectionComment `json:"accessor_comments,omitempty"`
	Getter            []Statement         `json:"getter,omitempty"`
	GetterSpan        token.Span          `json:"getter_span,omitempty"`
	GetterKeywordSpan token.Span          `json:"getter_keyword_span,omitempty"`
	Setter            *PropertySetter     `json:"setter,omitempty"`
	// GetterName and SetterName hold the method that a shorthand accessor
	// names, as in "var x: get = _get_x, set = _set_x". A property is written
	// either with shorthand accessors or with accessor bodies, never with both,
	// so a declaration carries one form or the other.
	GetterName        string     `json:"getter_name,omitempty"`
	GetterNameSpan    token.Span `json:"getter_name_span,omitempty"`
	SetterName        string     `json:"setter_name,omitempty"`
	SetterNameSpan    token.Span `json:"setter_name_span,omitempty"`
	SetterKeywordSpan token.Span `json:"setter_keyword_span,omitempty"`
	// AccessorBlock reports that the accessors were written in an indented
	// block. An accessor with a body always is, because its body needs one; a
	// shorthand accessor may be written there or on the declaration's own line.
	AccessorBlock bool `json:"accessor_block,omitempty"`
}

func (*VariableDeclaration) node()      {}
func (*VariableDeclaration) statement() {}

// Assignment assigns Value to Target using Operator.
type Assignment struct {
	Base
	Trivia
	Target       Expression `json:"target"`
	Operator     string     `json:"operator"`
	OperatorSpan token.Span `json:"operator_span,omitempty"`
	Value        Expression `json:"value"`
}

func (*Assignment) node()      {}
func (*Assignment) statement() {}

// ReturnStatement optionally returns a value.
type ReturnStatement struct {
	Base
	Trivia
	KeywordSpan token.Span `json:"keyword_span,omitempty"`
	Value       Expression `json:"value,omitempty"`
}

func (*ReturnStatement) node()      {}
func (*ReturnStatement) statement() {}

// Branch is one if/elif condition and body.
type Branch struct {
	Base
	// Comments holds the comments written on their own lines above the branch's
	// keyword. They belong to no body, since the keyword's line belongs to the
	// statement rather than to either block it separates.
	Comments    []*Comment  `json:"comments,omitempty"`
	KeywordSpan token.Span  `json:"keyword_span,omitempty"`
	Condition   Expression  `json:"condition"`
	Body        []Statement `json:"body"`
}

// IfStatement contains its if/elif branches and optional else body.
type IfStatement struct {
	Base
	Trivia
	Branches []Branch    `json:"branches"`
	Else     []Statement `json:"else,omitempty"`
	// ElseComments holds the comments written on their own lines above "else".
	ElseComments    []*Comment `json:"else_comments,omitempty"`
	ElseSpan        token.Span `json:"else_span,omitempty"`
	ElseKeywordSpan token.Span `json:"else_keyword_span,omitempty"`
}

func (*IfStatement) node()      {}
func (*IfStatement) statement() {}

// WhileStatement is a condition-controlled loop.
type WhileStatement struct {
	Base
	Trivia
	KeywordSpan token.Span  `json:"keyword_span,omitempty"`
	Condition   Expression  `json:"condition"`
	Body        []Statement `json:"body"`
}

func (*WhileStatement) node()      {}
func (*WhileStatement) statement() {}

// ForStatement iterates one variable over an expression.
type ForStatement struct {
	Base
	Trivia
	KeywordSpan  token.Span  `json:"keyword_span,omitempty"`
	Variable     string      `json:"variable"`
	VariableSpan token.Span  `json:"variable_span,omitempty"`
	Type         string      `json:"type,omitempty"`
	TypeSpan     token.Span  `json:"type_span,omitempty"`
	InSpan       token.Span  `json:"in_span,omitempty"`
	Iterable     Expression  `json:"iterable"`
	Body         []Statement `json:"body"`
}

func (*ForStatement) node()      {}
func (*ForStatement) statement() {}

// KeywordStatement is pass, break, or continue.
type KeywordStatement struct {
	Base
	Trivia
	Keyword     string     `json:"keyword"`
	KeywordSpan token.Span `json:"keyword_span,omitempty"`
}

func (*KeywordStatement) node()      {}
func (*KeywordStatement) statement() {}

// Parameter is a function or signal parameter.
type Parameter struct {
	Base
	Name                string     `json:"name"`
	NameSpan            token.Span `json:"name_span,omitempty"`
	Type                string     `json:"type,omitempty"`
	TypeSpan            token.Span `json:"type_span,omitempty"`
	Default             Expression `json:"default,omitempty"`
	DefaultOperatorSpan token.Span `json:"default_operator_span,omitempty"`
	// Inferred reports that the default was written with ":=", which gives the
	// parameter the type of its default value, rather than with "=", which
	// leaves it untyped.
	Inferred bool `json:"inferred,omitempty"`
	// Variadic reports a rest parameter, written "...name", which collects the
	// arguments that follow the parameters before it. It may only be the last
	// parameter of a function or a lambda.
	Variadic     bool       `json:"variadic,omitempty"`
	VariadicSpan token.Span `json:"variadic_span,omitempty"`
}

// FunctionDeclaration declares a function.
type FunctionDeclaration struct {
	Base
	Trivia
	Annotations     []*Annotation `json:"annotations,omitempty"`
	Name            string        `json:"name"`
	NameSpan        token.Span    `json:"name_span,omitempty"`
	Parameters      []Parameter   `json:"parameters"`
	ReturnType      string        `json:"return_type,omitempty"`
	ReturnTypeSpan  token.Span    `json:"return_type_span,omitempty"`
	ReturnArrowSpan token.Span    `json:"return_arrow_span,omitempty"`
	Static          bool          `json:"static,omitempty"`
	StaticSpan      token.Span    `json:"static_span,omitempty"`
	KeywordSpan     token.Span    `json:"keyword_span,omitempty"`
	Abstract        bool          `json:"abstract,omitempty"`
	Body            []Statement   `json:"body"`
	// ParameterComments holds the comments written inside the parameter list.
	ParameterComments []CollectionComment `json:"parameter_comments,omitempty"`
}

func (*FunctionDeclaration) node()      {}
func (*FunctionDeclaration) statement() {}

// ClassDeclaration declares an inner class.
type ClassDeclaration struct {
	Base
	Trivia
	Annotations  []*Annotation `json:"annotations,omitempty"`
	Name         string        `json:"name"`
	NameSpan     token.Span    `json:"name_span,omitempty"`
	Extends      string        `json:"extends,omitempty"`
	BaseTypeSpan token.Span    `json:"base_type_span,omitempty"`
	ExtendsSpan  token.Span    `json:"extends_span,omitempty"`
	KeywordSpan  token.Span    `json:"keyword_span,omitempty"`
	Body         []Statement   `json:"body"`
}

func (*ClassDeclaration) node()      {}
func (*ClassDeclaration) statement() {}

// SignalDeclaration declares a signal.
type SignalDeclaration struct {
	Base
	Trivia
	Annotations []*Annotation `json:"annotations,omitempty"`
	Name        string        `json:"name"`
	NameSpan    token.Span    `json:"name_span,omitempty"`
	KeywordSpan token.Span    `json:"keyword_span,omitempty"`
	Parameters  []Parameter   `json:"parameters,omitempty"`
	// ParameterComments holds the comments written inside the parameter list.
	ParameterComments []CollectionComment `json:"parameter_comments,omitempty"`
}

func (*SignalDeclaration) node()      {}
func (*SignalDeclaration) statement() {}

// EnumMember is one enum value.
type EnumMember struct {
	Base
	Name         string     `json:"name"`
	NameSpan     token.Span `json:"name_span,omitempty"`
	Value        Expression `json:"value,omitempty"`
	OperatorSpan token.Span `json:"operator_span,omitempty"`
}

// PropertySetter stores a property's setter parameter and body.
type PropertySetter struct {
	Base
	KeywordSpan   token.Span  `json:"keyword_span,omitempty"`
	Parameter     string      `json:"parameter"`
	ParameterSpan token.Span  `json:"parameter_span,omitempty"`
	Body          []Statement `json:"body"`
}

// EnumDeclaration declares a named or anonymous enum.
type EnumDeclaration struct {
	Base
	Trivia
	Annotations []*Annotation `json:"annotations,omitempty"`
	Name        string        `json:"name,omitempty"`
	NameSpan    token.Span    `json:"name_span,omitempty"`
	KeywordSpan token.Span    `json:"keyword_span,omitempty"`
	Members     []EnumMember  `json:"members"`
	// Comments holds the comments written inside the enum body.
	Comments []CollectionComment `json:"comments,omitempty"`
}

func (*EnumDeclaration) node()      {}
func (*EnumDeclaration) statement() {}

// MatchCase contains comma-separated patterns and a body. A case holding no
// pattern is the bare "pass" that Godot accepts in a match body, which matches
// nothing and runs nothing.
type MatchCase struct {
	Base
	// Annotations holds the annotations written on their own lines ahead of the
	// branch. Godot allows only @warning_ignore here.
	Annotations []*Annotation `json:"annotations,omitempty"`
	Patterns    []Expression  `json:"patterns"`
	Guard       Expression    `json:"guard,omitempty"`
	WhenSpan    token.Span    `json:"when_span,omitempty"`
	Body        []Statement   `json:"body"`
}

// MatchStatement performs pattern matching.
type MatchStatement struct {
	Base
	Trivia
	KeywordSpan token.Span  `json:"keyword_span,omitempty"`
	Value       Expression  `json:"value"`
	Cases       []MatchCase `json:"cases"`
	// Comments holds the comments written inside the statement but outside any
	// case body, anchored to the case they precede. A comment that ended the
	// "match" line itself is anchored to the first case and marked trailing.
	Comments []CollectionComment `json:"comments,omitempty"`
}

func (*MatchStatement) node()      {}
func (*MatchStatement) statement() {}
