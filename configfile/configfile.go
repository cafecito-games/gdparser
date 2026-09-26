// Package configfile parses, transforms, and formats Godot 4 ConfigFile
// syntax, including project.godot files.
package configfile

import (
	"github.com/cafecito-games/gdparser/configfile/ast"
	configformat "github.com/cafecito-games/gdparser/configfile/format"
	"github.com/cafecito-games/gdparser/configfile/parser"
)

// Parse parses source without attaching a filename to diagnostics.
func Parse(source []byte) (*ast.File, error) { return parser.Parse("", source) }

// ParseString parses source without attaching a filename to diagnostics.
func ParseString(source string) (*ast.File, error) { return Parse([]byte(source)) }

// ParseFile parses source and includes filename in diagnostics and the AST.
func ParseFile(filename string, source []byte) (*ast.File, error) {
	return parser.Parse(filename, source)
}

// Format emits canonical ConfigFile source for file.
func Format(file *ast.File) string { return configformat.File(file) }
