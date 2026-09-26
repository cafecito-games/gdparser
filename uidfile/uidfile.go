// Package uidfile parses, transforms, and formats Godot resource UID sidecar
// files. Godot stores these next to source resources using a .uid suffix.
package uidfile

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

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

// Node is implemented by every UID file syntax node.
type Node interface {
	Span() Span
	node()
}

// Base stores the source range embedded by concrete nodes.
type Base struct {
	SourceSpan Span `json:"span"`
}

// Span returns the node's source range.
func (b Base) Span() Span { return b.SourceSpan }

// File is one parsed Godot resource UID sidecar.
type File struct {
	Base
	Name string `json:"name,omitempty"`
	UID  *UID   `json:"uid"`
}

func (*File) node() {}

// UID is a textual Godot resource identifier such as uid://c3m2k2i8we5da.
type UID struct {
	Base
	Value string `json:"value"`
}

func (*UID) node() {}

// Error describes invalid UID sidecar source.
type Error struct {
	Filename string
	Position Position
	Message  string
}

func (e *Error) Error() string {
	location := e.Position.String()
	if e.Filename != "" {
		location = e.Filename + ":" + location
	}
	return location + ": " + e.Message
}

// Parse parses source without attaching a filename to diagnostics.
func Parse(source []byte) (*File, error) { return ParseFile("", source) }

// ParseString parses source without attaching a filename to diagnostics.
func ParseString(source string) (*File, error) { return Parse([]byte(source)) }

// ParseFile parses source and includes filename in diagnostics and the AST.
func ParseFile(filename string, source []byte) (*File, error) {
	if !utf8.Valid(source) {
		offset := firstInvalidUTF8(source)
		return nil, parseError(filename, source, offset, "invalid UTF-8")
	}

	lineEnd := len(source)
	for i, c := range source {
		if c == '\r' || c == '\n' {
			lineEnd = i
			break
		}
	}
	documentEnd := lineEnd
	if documentEnd < len(source) {
		if source[documentEnd] == '\r' && documentEnd+1 < len(source) && source[documentEnd+1] == '\n' {
			documentEnd += 2
		} else {
			documentEnd++
		}
	}
	if documentEnd < len(source) {
		return nil, parseError(filename, source, documentEnd, "expected end of UID file")
	}

	value := string(source[:lineEnd])
	const prefix = "uid://"
	if !strings.HasPrefix(value, prefix) {
		return nil, parseError(filename, source, firstMismatch(value, prefix), "expected UID beginning with uid://")
	}
	if len(value) == len(prefix) {
		return nil, parseError(filename, source, len(prefix), "UID value must not be empty")
	}
	if value == "uid://<invalid>" {
		return newFile(filename, source, lineEnd, value), nil
	}
	// Keep the identifier body syntactic and opaque. Resolution to Godot's
	// numeric ResourceUID representation is a separate engine-level concern,
	// and source projects can contain alphanumeric IDs from external tooling.
	for i := len(prefix); i < len(value); i++ {
		c := value[i]
		if c < 'a' || c > 'z' {
			if c >= 'A' && c <= 'Z' {
				continue
			}
			if c < '0' || c > '9' {
				return nil, parseError(filename, source, i, "UID may contain only ASCII letters and digits")
			}
		}
	}

	return newFile(filename, source, lineEnd, value), nil
}

func newFile(filename string, source []byte, lineEnd int, value string) *File {
	uidSpan := Span{Start: positionAt(source, 0), End: positionAt(source, lineEnd)}
	return &File{
		Base: Base{SourceSpan: Span{Start: positionAt(source, 0), End: positionAt(source, len(source))}},
		Name: filename,
		UID:  &UID{Base: Base{SourceSpan: uidSpan}, Value: value},
	}
}

// Format emits canonical UID sidecar source.
func Format(file *File) string {
	if file == nil || file.UID == nil {
		return ""
	}
	return file.UID.Value + "\n"
}

func firstMismatch(value, prefix string) int {
	limit := min(len(value), len(prefix))
	for i := 0; i < limit; i++ {
		if value[i] != prefix[i] {
			return i
		}
	}
	return limit
}

func firstInvalidUTF8(source []byte) int {
	for offset := 0; offset < len(source); {
		_, size := utf8.DecodeRune(source[offset:])
		if size == 1 && source[offset] >= utf8.RuneSelf {
			return offset
		}
		offset += size
	}
	return len(source)
}

func parseError(filename string, source []byte, offset int, message string) error {
	return &Error{Filename: filename, Position: positionAt(source, offset), Message: message}
}

func positionAt(source []byte, offset int) Position {
	position := Position{Offset: offset, Line: 1, Column: 1}
	for current := 0; current < offset; {
		r, size := utf8.DecodeRune(source[current:])
		if r == '\r' {
			if current+size < offset && source[current+size] == '\n' {
				size++
			}
			position.Line++
			position.Column = 1
		} else if r == '\n' {
			position.Line++
			position.Column = 1
		} else {
			position.Column++
		}
		current += size
	}
	return position
}
