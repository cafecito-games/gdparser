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
	Trivia
	Name      string       `json:"name"`
	NameSpan  token.Span   `json:"name_span,omitempty"`
	Arguments []Expression `json:"arguments,omitempty"`
	// OwnLine reports that the annotation was written on its own source line
	// rather than ahead of a declaration on the same line. Formatting keeps that
	// choice, which the style guide does not prescribe.
	OwnLine bool `json:"own_line,omitempty"`
}

func (*Annotation) node()      {}
func (*Annotation) statement() {}

// Comment preserves a regular (#) or documentation (##) comment.
type Comment struct {
	Base
	Trivia
	Text          string `json:"text"`
	Documentation bool   `json:"documentation,omitempty"`
}

func (*Comment) node()      {}
func (*Comment) statement() {}

// Directive represents file/class directives such as extends and class_name.
type Directive struct {
	Base
	Trivia
	Name        string     `json:"name"`
	KeywordSpan token.Span `json:"keyword_span,omitempty"`
	Value       Expression `json:"value,omitempty"`
	Extends     Expression `json:"extends,omitempty"`
	ExtendsSpan token.Span `json:"extends_span,omitempty"`
}

func (*Directive) node()      {}
func (*Directive) statement() {}

// Trivia records the formatting-relevant source context of a statement.
// Mutating an AST does not update it, and programmatically constructed
// statements may leave it zero.
type Trivia struct {
	// BlankLinesBefore is the number of blank source lines that preceded the
	// statement. For a declaration carrying annotations it is measured before
	// the first annotation.
	BlankLinesBefore int `json:"blank_lines_before,omitempty"`
	// TrailingComment is a comment that followed the statement on its own line
	// of source, as in "x = 1  # why".
	TrailingComment *Comment `json:"trailing_comment,omitempty"`
}

// StatementTrivia returns t so that statements embedding Trivia expose it
// through a common interface.
func (t *Trivia) StatementTrivia() *Trivia { return t }

// TriviaBearer is implemented by every statement node that carries Trivia.
type TriviaBearer interface {
	StatementTrivia() *Trivia
}

// TriviaOf returns the trivia attached to statement, or nil when statement is
// nil or does not carry any.
func TriviaOf(statement Statement) *Trivia {
	bearer, ok := statement.(TriviaBearer)
	if !ok {
		return nil
	}
	return bearer.StatementTrivia()
}
