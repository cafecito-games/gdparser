// Package token defines the lexical tokens and source locations used by gdparser.
package token

import "fmt"

// Position is a zero-based byte offset and one-based line and column.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (p Position) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Column) }

// Span is a half-open source range [Start, End).
type Span struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Type identifies a lexical token.
type Type string

const (
	Illegal Type = "ILLEGAL"
	EOF     Type = "EOF"
	Newline Type = "NEWLINE"
	Indent  Type = "INDENT"
	Dedent  Type = "DEDENT"

	Identifier Type = "IDENTIFIER"
	Integer    Type = "INTEGER"
	Float      Type = "FLOAT"
	String     Type = "STRING"
	Comment    Type = "COMMENT"

	LParen    Type = "("
	RParen    Type = ")"
	LBracket  Type = "["
	RBracket  Type = "]"
	LBrace    Type = "{"
	RBrace    Type = "}"
	Comma     Type = ","
	Semicolon Type = ";"
	Colon     Type = ":"
	Dot       Type = "."
	At        Type = "@"
	Dollar    Type = "$"
	Percent   Type = "%"
	Arrow     Type = "->"

	Assign           Type = "="
	InferAssign      Type = ":="
	Plus             Type = "+"
	Minus            Type = "-"
	Star             Type = "*"
	Slash            Type = "/"
	DoubleStar       Type = "**"
	Equal            Type = "=="
	NotEqual         Type = "!="
	Less             Type = "<"
	LessEqual        Type = "<="
	Greater          Type = ">"
	GreaterEqual     Type = ">="
	ShiftLeft        Type = "<<"
	ShiftRight       Type = ">>"
	Ampersand        Type = "&"
	Pipe             Type = "|"
	Caret            Type = "^"
	Tilde            Type = "~"
	Bang             Type = "!"
	PlusAssign       Type = "+="
	MinusAssign      Type = "-="
	StarAssign       Type = "*="
	SlashAssign      Type = "/="
	PercentAssign    Type = "%="
	AmpAssign        Type = "&="
	PipeAssign       Type = "|="
	CaretAssign      Type = "^="
	ShiftLeftAssign  Type = "<<="
	ShiftRightAssign Type = ">>="

	And       Type = "and"
	As        Type = "as"
	Await     Type = "await"
	Break     Type = "break"
	Class     Type = "class"
	ClassName Type = "class_name"
	Const     Type = "const"
	Continue  Type = "continue"
	Elif      Type = "elif"
	Else      Type = "else"
	Enum      Type = "enum"
	Extends   Type = "extends"
	False     Type = "false"
	For       Type = "for"
	Func      Type = "func"
	If        Type = "if"
	In        Type = "in"
	Is        Type = "is"
	Match     Type = "match"
	Not       Type = "not"
	Null      Type = "null"
	Or        Type = "or"
	Pass      Type = "pass"
	Return    Type = "return"
	Signal    Type = "signal"
	Static    Type = "static"
	Tool      Type = "tool"
	True      Type = "true"
	Var       Type = "var"
	While     Type = "while"
)

var keywords = map[string]Type{
	"and": And, "as": As, "await": Await, "break": Break, "class": Class,
	"class_name": ClassName, "const": Const, "continue": Continue, "elif": Elif,
	"else": Else, "enum": Enum, "extends": Extends, "false": False, "for": For,
	"func": Func, "if": If, "in": In, "is": Is, "match": Match, "not": Not,
	"null": Null, "or": Or, "pass": Pass, "return": Return, "signal": Signal,
	"static": Static, "tool": Tool, "true": True, "var": Var, "while": While,
}

// LookupIdentifier returns a keyword token when text is reserved.
func LookupIdentifier(text string) Type {
	if typ, ok := keywords[text]; ok {
		return typ
	}
	return Identifier
}

// Token is a lexeme and its source location.
type Token struct {
	Type   Type   `json:"type"`
	Lexeme string `json:"lexeme,omitempty"`
	Span   Span   `json:"span"`
}

func (t Token) String() string {
	if t.Lexeme == "" {
		return string(t.Type)
	}
	return fmt.Sprintf("%s(%q)", t.Type, t.Lexeme)
}
