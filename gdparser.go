// Package gdparser parses and transforms Godot 4 GDScript source code.
package gdparser

import (
	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// Parse parses source without attaching a filename to diagnostics.
func Parse(source []byte) (*ast.File, error) { return parser.Parse("", source) }

// ParseString parses source without attaching a filename to diagnostics.
func ParseString(source string) (*ast.File, error) { return Parse([]byte(source)) }

// ParseFile parses source and includes filename in diagnostics and the AST.
func ParseFile(filename string, source []byte) (*ast.File, error) {
	return parser.Parse(filename, source)
}

// Format emits canonical GDScript for file.
func Format(file *ast.File) string { return gdformat.File(file) }
