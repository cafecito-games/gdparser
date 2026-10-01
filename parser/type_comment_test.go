package parser_test

import (
	"strings"
	"testing"

	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A comment ends a type rather than being read as more type text, so the
// comment survives and the type stays the type.
func TestCommentAfterAType(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "after a parameter's type",
			source: "func f(a: int  # c\n):\n\tpass\n",
			want:   "func f(\n\t\ta: int  # c\n):\n\tpass\n",
		},
		{
			name:   "after a parameter's generic type",
			source: "func f(a: Array[int]  # c\n):\n\tpass\n",
			want:   "func f(\n\t\ta: Array[int]  # c\n):\n\tpass\n",
		},
		{
			name:   "after a parameter's type beside another parameter",
			source: "func f(a: int,  # c\n\t\tb: int):\n\tpass\n",
			want:   "func f(\n\t\ta: int,  # c\n\t\tb: int\n):\n\tpass\n",
		},
		{
			name:   "after a declaration's type",
			source: "var b: int = 1  # c\n",
			want:   "var b: int = 1  # c\n",
		},
		{
			name:   "after a declaration's generic type",
			source: "var b: Array[int]  # c\n",
			want:   "var b: Array[int]  # c\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("types.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			if !strings.Contains(formatted, "# c") {
				t.Fatalf("formatted source dropped the comment:\n%s", formatted)
			}
			again, err := parser.Parse("types.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

// A type argument list is written on one line, so a comment inside it, which
// would carry the rest of the type onto the next line, is an error. Godot
// rejects the shape as well.
func TestCommentInsideATypeArgumentList(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"a declaration's type", "var b: Array[\n\t# c\nint] = []\n"},
		{"a parameter's type", "func f(a: Array[\n\t\t# c\n\t\tint]):\n\tpass\n"},
		{"a return type", "func f() -> Array[\n\t\t# c\n\t\tint]:\n\tpass\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("types.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), "one line") {
				t.Fatalf("error does not name the rule: %v", err)
			}
			if !strings.Contains(err.Error(), "types.gd:") {
				t.Fatalf("error lacks a position: %v", err)
			}
		})
	}
}
