// Package ast defines the typed abstract syntax tree for Godot 4 shaders.
package ast

import "github.com/cafecito-games/gdparser/shader/token"

// Node is implemented by every AST node.
type Node interface {
	Span() token.Span
	node()
}

// Item is a file-level construct.
type Item interface {
	Node
	item()
}

// Statement is a construct allowed in a block.
type Statement interface {
	Node
	statement()
}

// Expression is a value-producing syntax node.
type Expression interface {
	Node
	expression()
}

// StructMember is a field or comment inside a struct declaration.
type StructMember interface {
	Node
	structMember()
}

// InitializerItem is an expression or comment inside an aggregate initializer.
type InitializerItem interface {
	Node
	initializerItem()
}

// Base stores source location data embedded by concrete nodes.
type Base struct {
	SourceSpan token.Span `json:"span"`
}

func (b Base) Span() token.Span { return b.SourceSpan }

// File is one parsed .gdshader or .gdshaderinc source file.
type File struct {
	Base
	Name  string `json:"name,omitempty"`
	Items []Item `json:"items"`
}

func (*File) node() {}

// Comment preserves a line or block comment.
type Comment struct {
	Base
	Text  string `json:"text"`
	Block bool   `json:"block,omitempty"`
}

func (*Comment) node()            {}
func (*Comment) item()            {}
func (*Comment) statement()       {}
func (*Comment) structMember()    {}
func (*Comment) initializerItem() {}

// PreprocessorDirective preserves one complete directive line. Name excludes
// the leading # and Body contains the remaining macro/token text.
type PreprocessorDirective struct {
	Base
	Name string `json:"name"`
	Body string `json:"body,omitempty"`
}

func (*PreprocessorDirective) node()      {}
func (*PreprocessorDirective) item()      {}
func (*PreprocessorDirective) statement() {}

// ShaderType declares the spatial, canvas_item, particles, sky, fog, or custom shader type.
type ShaderType struct {
	Base
	Name string `json:"name"`
}

func (*ShaderType) node() {}
func (*ShaderType) item() {}

// RenderMode declares a list of render mode expressions.
type RenderMode struct {
	Base
	Modes []Expression `json:"modes"`
}

func (*RenderMode) node() {}
func (*RenderMode) item() {}

// StencilMode declares spatial shader stencil operations and the optional
// numeric reference value.
type StencilMode struct {
	Base
	Modes []Expression `json:"modes"`
}

func (*StencilMode) node() {}
func (*StencilMode) item() {}

// GroupUniforms starts or ends a named uniform group. An empty Name ends it.
type GroupUniforms struct {
	Base
	Name     string `json:"name,omitempty"`
	Subgroup string `json:"subgroup,omitempty"`
}

func (*GroupUniforms) node() {}
func (*GroupUniforms) item() {}

// Qualifiers are declaration modifiers, split by their semantic role.
type Qualifiers struct {
	Storage       []string `json:"storage,omitempty"`
	Interpolation string   `json:"interpolation,omitempty"`
	Precision     string   `json:"precision,omitempty"`
	Invariant     bool     `json:"invariant,omitempty"`
	Const         bool     `json:"const,omitempty"`
}

// ArraySpecifier is an optional array dimension. Size nil means unsized [].
type ArraySpecifier struct {
	Base
	Size Expression `json:"size,omitempty"`
}

func (*ArraySpecifier) node() {}

// Parameter is one function parameter.
type Parameter struct {
	Base
	Qualifiers Qualifiers        `json:"qualifiers,omitempty"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Arrays     []*ArraySpecifier `json:"arrays,omitempty"`
}

func (*Parameter) node() {}

// Declarator is one name, optional dimensions, and initializer in a declaration.
type Declarator struct {
	Base
	Name   string            `json:"name"`
	Arrays []*ArraySpecifier `json:"arrays,omitempty"`
	Value  Expression        `json:"value,omitempty"`
}

func (*Declarator) node() {}

// Hint is the colon suffix of a uniform declaration.
type Hint struct {
	Base
	Name      string       `json:"name"`
	Arguments []Expression `json:"arguments,omitempty"`
}

func (*Hint) node() {}

// VariableDeclaration declares one or more variables of a common type.
type VariableDeclaration struct {
	Base
	Qualifiers  Qualifiers    `json:"qualifiers,omitempty"`
	Type        string        `json:"type"`
	Declarators []*Declarator `json:"declarators"`
	Hints       []*Hint       `json:"hints,omitempty"`
}

func (*VariableDeclaration) node()         {}
func (*VariableDeclaration) item()         {}
func (*VariableDeclaration) statement()    {}
func (*VariableDeclaration) structMember() {}

// StructDeclaration declares a structure type.
type StructDeclaration struct {
	Base
	Name    string         `json:"name"`
	Members []StructMember `json:"members"`
}

func (*StructDeclaration) node() {}
func (*StructDeclaration) item() {}

// FunctionDeclaration declares a shader function.
type FunctionDeclaration struct {
	Base
	ReturnQualifiers Qualifiers        `json:"return_qualifiers,omitempty"`
	ReturnType       string            `json:"return_type"`
	ReturnArrays     []*ArraySpecifier `json:"return_arrays,omitempty"`
	Name             string            `json:"name"`
	Parameters       []*Parameter      `json:"parameters"`
	Body             *Block            `json:"body"`
}

func (*FunctionDeclaration) node() {}
func (*FunctionDeclaration) item() {}

// Block is a brace-delimited statement list.
type Block struct {
	Base
	Statements []Statement `json:"statements"`
}

func (*Block) node()      {}
func (*Block) statement() {}

// ExpressionStatement evaluates an expression for side effects.
type ExpressionStatement struct {
	Base
	Expression Expression `json:"expression,omitempty"`
}

func (*ExpressionStatement) node()      {}
func (*ExpressionStatement) statement() {}

// IfStatement is a conditional with an optional else branch.
type IfStatement struct {
	Base
	Condition Expression `json:"condition"`
	Then      Statement  `json:"then"`
	Else      Statement  `json:"else,omitempty"`
}

func (*IfStatement) node()      {}
func (*IfStatement) statement() {}

// ForStatement is a C-style loop.
type ForStatement struct {
	Base
	Initializer Statement  `json:"initializer,omitempty"`
	Condition   Expression `json:"condition,omitempty"`
	Update      Expression `json:"update,omitempty"`
	Body        Statement  `json:"body"`
}

func (*ForStatement) node()      {}
func (*ForStatement) statement() {}

// WhileStatement is a condition-controlled loop.
type WhileStatement struct {
	Base
	Condition Expression `json:"condition"`
	Body      Statement  `json:"body"`
}

func (*WhileStatement) node()      {}
func (*WhileStatement) statement() {}

// DoWhileStatement tests its condition after the body.
type DoWhileStatement struct {
	Base
	Body      Statement  `json:"body"`
	Condition Expression `json:"condition"`
}

func (*DoWhileStatement) node()      {}
func (*DoWhileStatement) statement() {}

// ReturnStatement optionally returns a value.
type ReturnStatement struct {
	Base
	Value Expression `json:"value,omitempty"`
}

func (*ReturnStatement) node()      {}
func (*ReturnStatement) statement() {}

// KeywordStatement is break, continue, or discard.
type KeywordStatement struct {
	Base
	Keyword string `json:"keyword"`
}

func (*KeywordStatement) node()      {}
func (*KeywordStatement) statement() {}

// SwitchStatement selects among case clauses.
type SwitchStatement struct {
	Base
	Value Expression    `json:"value"`
	Cases []*SwitchCase `json:"cases"`
}

func (*SwitchStatement) node()      {}
func (*SwitchStatement) statement() {}

// SwitchCase is a case expression or default clause and its statements.
type SwitchCase struct {
	Base
	Value      Expression  `json:"value,omitempty"`
	Default    bool        `json:"default,omitempty"`
	Statements []Statement `json:"statements"`
}

func (*SwitchCase) node() {}
