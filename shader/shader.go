// Package shader parses, transforms, and formats Godot 4 .gdshader and
// .gdshaderinc source files.
package shader

import (
	"github.com/cafecito-games/gdparser/shader/ast"
	shaderformat "github.com/cafecito-games/gdparser/shader/format"
	"github.com/cafecito-games/gdparser/shader/parser"
)

// Parse parses source without attaching a filename to diagnostics.
func Parse(source []byte) (*ast.File, error) { return parser.Parse("", source) }

// ParseString parses string source without attaching a filename.
func ParseString(source string) (*ast.File, error) { return Parse([]byte(source)) }

// ParseFile parses source with filename in diagnostics and the AST.
func ParseFile(filename string, source []byte) (*ast.File, error) {
	return parser.Parse(filename, source)
}

// Format emits canonical Godot shading-language source.
func Format(file *ast.File) string { return shaderformat.File(file) }
