// Package ast defines the typed abstract syntax tree produced by gdparser.
package ast

import "github.com/cafecito-games/gdparser/token"

// Node is implemented by every AST node.
type Node interface {
	Span() token.Span
	node()
}

// Statement is a node that can appear in a statement list.
type Statement interface {
	Node
	statement()
}

// Expression is a node that produces a value.
type Expression interface {
	Node
	expression()
}

// Base stores source location data embedded by concrete nodes and source
// components. Spans describe the original parsed source. Mutating AST values
// does not update them; programmatically constructed trees may leave them zero.
type Base struct {
	SourceSpan token.Span `json:"span"`
}

func (b Base) Span() token.Span { return b.SourceSpan }

// File is one parsed GDScript source file.
type File struct {
	Base
	Name       string      `json:"name,omitempty"`
	Statements []Statement `json:"statements"`
}

func (*File) node() {}

// Annotation represents @name or @name(arguments). It is retained as a
// statement so annotation order can be inspected and transformed.
type Annotation struct {
	Base
	Name      string       `json:"name"`
	NameSpan  token.Span   `json:"name_span,omitempty"`
	Arguments []Expression `json:"arguments,omitempty"`
}

func (*Annotation) node()      {}
func (*Annotation) statement() {}

// Comment preserves a regular (#) or documentation (##) comment.
type Comment struct {
	Base
	Text          string `json:"text"`
	Documentation bool   `json:"documentation,omitempty"`
}

func (*Comment) node()      {}
func (*Comment) statement() {}

// Directive represents file/class directives such as extends and class_name.
type Directive struct {
	Base
	Name        string     `json:"name"`
	KeywordSpan token.Span `json:"keyword_span,omitempty"`
	Value       Expression `json:"value,omitempty"`
	Extends     Expression `json:"extends,omitempty"`
	ExtendsSpan token.Span `json:"extends_span,omitempty"`
}

func (*Directive) node()      {}
func (*Directive) statement() {}
