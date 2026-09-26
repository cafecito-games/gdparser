// Package token defines lexical tokens and source locations for Godot project
// configuration files.
package token

import "fmt"

// Position is a zero-based byte offset and a one-based line and column.
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
	Illegal    Type = "ILLEGAL"
	EOF        Type = "EOF"
	Newline    Type = "NEWLINE"
	Identifier Type = "IDENTIFIER"
	Integer    Type = "INTEGER"
	Float      Type = "FLOAT"
	String     Type = "STRING"
	Comment    Type = "COMMENT"
	LParen     Type = "("
	RParen     Type = ")"
	LBracket   Type = "["
	RBracket   Type = "]"
	LBrace     Type = "{"
	RBrace     Type = "}"
	Comma      Type = ","
	Colon      Type = ":"
	Assign     Type = "="
	Plus       Type = "+"
	Minus      Type = "-"
	Ampersand  Type = "&"
	Caret      Type = "^"
)

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
