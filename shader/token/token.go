// Package token defines the lexical tokens and source locations used by the
// Godot shading-language frontend.
package token

import "fmt"

// Kind identifies a lexical token category.
type Kind string

const (
	EOF        Kind = "eof"
	Identifier Kind = "identifier"
	Number     Kind = "number"
	String     Kind = "string"
	Symbol     Kind = "symbol"
	Comment    Kind = "comment"
	Directive  Kind = "directive"
)

// Position is a byte offset and one-based line and column position.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Span is a half-open source range.
type Span struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Token is one lexical unit. Text retains its exact spelling.
type Token struct {
	Kind Kind   `json:"kind"`
	Text string `json:"text"`
	Span Span   `json:"span"`
}

// Error is a positioned lexical or syntactic diagnostic.
type Error struct {
	Filename string
	Position Position
	Message  string
}

func (e *Error) Error() string {
	location := fmt.Sprintf("%d:%d", e.Position.Line, e.Position.Column)
	if e.Filename != "" {
		location = e.Filename + ":" + location
	}
	return location + ": " + e.Message
}

// Join returns the smallest span containing a and b.
func Join(a, b Span) Span { return Span{Start: a.Start, End: b.End} }
