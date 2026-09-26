// Package ast defines the typed, mutable abstract syntax tree for Godot
// project configuration files.
package ast

import "github.com/cafecito-games/gdparser/projectconfig/token"

// Node is implemented by every AST node.
type Node interface {
	Span() token.Span
	node()
}

// Statement is an assignment or comment in a preamble or section.
type Statement interface {
	Node
	statement()
}

// Expression is a ConfigFile Variant value.
type Expression interface {
	Node
	expression()
}

// ArrayItem is an element or comment in an array literal.
type ArrayItem interface {
	Node
	arrayItem()
}

// DictionaryItem is an entry or comment in a dictionary literal.
type DictionaryItem interface {
	Node
	dictionaryItem()
}

// ConstructorItem is an argument or comment in a constructor call.
type ConstructorItem interface {
	Node
	constructorItem()
}

// Base stores the source range embedded by concrete nodes.
type Base struct {
	SourceSpan token.Span `json:"span"`
}

func (b Base) Span() token.Span { return b.SourceSpan }

// File is one parsed project.godot-style configuration file. Preamble holds
// assignments and comments before the first section.
type File struct {
	Base
	Name     string      `json:"name,omitempty"`
	Preamble []Statement `json:"preamble"`
	Sections []*Section  `json:"sections"`
}

func (*File) node() {}

// Section is an ordered [name] section.
type Section struct {
	Base
	Name       string      `json:"name"`
	Statements []Statement `json:"statements"`
}

func (*Section) node() {}

// Assignment binds a slash-separated project setting key to a value.
type Assignment struct {
	Base
	Key   string     `json:"key"`
	Value Expression `json:"value"`
}

func (*Assignment) node()      {}
func (*Assignment) statement() {}

// Comment preserves a hash or semicolon line comment, including its marker.
type Comment struct {
	Base
	Text string `json:"text"`
}

func (*Comment) node()            {}
func (*Comment) statement()       {}
func (*Comment) arrayItem()       {}
func (*Comment) dictionaryItem()  {}
func (*Comment) constructorItem() {}
