package shader_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/shader"
	"github.com/cafecito-games/gdparser/shader/ast"
	"github.com/cafecito-games/gdparser/shader/token"
)

const byteOrderMark = "\xef\xbb\xbf"

// A byte order mark is encoding, not syntax, so it must not reach the first
// declaration. Its bytes still count toward offsets, but not toward columns.
func TestParseSkipsByteOrderMark(t *testing.T) {
	const body = "shader_type canvas_item;\n\nvoid fragment() {}\n"
	file, err := shader.ParseString(byteOrderMark + body)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if len(file.Items) == 0 {
		t.Fatal("parsed file has no items")
	}
	shaderType, ok := file.Items[0].(*ast.ShaderType)
	if !ok {
		t.Fatalf("first item = %T, want *ast.ShaderType", file.Items[0])
	}
	if shaderType.Name != "canvas_item" {
		t.Errorf("shader type = %q, want %q", shaderType.Name, "canvas_item")
	}
	want := token.Position{Offset: len(byteOrderMark), Line: 1, Column: 1}
	if start := shaderType.Span().Start; start != want {
		t.Errorf("first item start = offset %d at %d:%d, want offset %d at %d:%d",
			start.Offset, start.Line, start.Column, want.Offset, want.Line, want.Column)
	}
	formatted := shader.Format(file)
	if formatted != body {
		t.Errorf("Format = %q, want %q", formatted, body)
	}
	if strings.Contains(formatted, byteOrderMark) {
		t.Error("formatted output still carries a byte order mark")
	}
}
