package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// An annotation that waits for a target no statement or member below it can be
// is an error. Godot holds one on its annotation stack until something takes it
// and reports every annotation still waiting when the file ends, which is what
// clear_unused_annotations does at the end of parse_program.
func TestAnnotationDecoratingNothing(t *testing.T) {
	for _, source := range []string{
		"func f(x):\n\tif x: @warning_ignore(\"unused_variable\")\n",
		"func f(x):\n\t@warning_ignore(\"unused_variable\")\n",
		"func f(x):\n\tpass\n\t@warning_ignore(\"unused_variable\")\n",
		"func f(): @warning_ignore(\"unused_variable\")\n",
		"var a = 1\n@warning_ignore(\"unused_variable\")\n",
		"class Inner:\n\tvar a = 1\n\t@warning_ignore(\"unused_variable\")\n",
		"func f(x):\n\tif x:\n\t\t@warning_ignore(\"unused_variable\")\n",
		"func f(x):\n\t@warning_ignore(\"unused_variable\")\n\t@warning_ignore(\"shadowed_variable\")\n",
		"func g():\n\tpass\n\n@export\n",
	} {
		_, err := parser.Parse("nothing.gd", []byte(source))
		if err == nil {
			t.Errorf("parsed %q, want an error", source)
			continue
		}
		if !strings.Contains(err.Error(), "decorates nothing") {
			t.Errorf("parse %q reported %v, want the annotation to decorate nothing", source, err)
		}
	}
}

// Godot keeps one annotation stack for the whole file, so an annotation written
// as the whole body of a block decorates the statement after the block rather
// than nothing. The annotation moves out to the scope that takes it, and the
// block it was written in is empty, which the formatter spells as "pass".
func TestAnnotationDecoratingTheStatementAfterItsBlock(t *testing.T) {
	for _, test := range []struct {
		source string
		want   string
	}{
		{
			source: "func f(x):\n\tif x: @warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
		},
		{
			source: "func f(x):\n\tif x:\n\t\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
		},
		{
			source: "class Inner:\n\tvar a = 1\n\t@warning_ignore(\"unused_variable\")\nvar b = 2\n",
			want:   "class Inner:\n\tvar a = 1\n\n\n@warning_ignore(\"unused_variable\")\nvar b = 2\n",
		},
		{
			source: "func f(x):\n\tif x:\n\t\tif x:\n\t\t\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
			want:   "func f(x):\n\tif x:\n\t\tif x:\n\t\t\tpass\n\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
		},
		{
			source: "func f():\n\tvar c = func():\n\t\tpass\n\t\t@warning_ignore(\"unused_variable\")\n\tvar d = 1\n",
			want:   "func f():\n\tvar c = func():\n\t\tpass\n\t@warning_ignore(\"unused_variable\")\n\tvar d = 1\n",
		},
		{
			source: "func f(x):\n\tif x: @warning_ignore(\"unused_variable\")\nfunc g():\n\tpass\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\n\n@warning_ignore(\"unused_variable\")\nfunc g():\n\tpass\n",
		},
	} {
		file, err := parser.Parse("leak.gd", []byte(test.source))
		if err != nil {
			t.Errorf("parse %q: %v", test.source, err)
			continue
		}
		formatted := gdformat.File(file)
		if formatted != test.want {
			t.Errorf("formatted %q as %q, want %q", test.source, formatted, test.want)
		}
		again, err := parser.Parse("leak.gd", []byte(formatted))
		if err != nil {
			t.Errorf("formatted %q did not parse: %v", formatted, err)
			continue
		}
		if second := gdformat.File(again); second != formatted {
			t.Errorf("formatting %q is not idempotent: %q", test.source, second)
		}
	}
}

// A file of nothing but annotations leaves the head of the script open to the
// end, and Godot's parse_program hands the annotations still waiting there to
// the script's own class rather than reporting them.
func TestAnnotationEndingAnOpenScriptHead(t *testing.T) {
	source := "@warning_ignore(\"unused_variable\")\n"
	file, err := parser.Parse("head.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(file.Statements))
	}
	if _, ok := file.Statements[0].(*ast.Annotation); !ok {
		t.Fatalf("statement = %T, want an annotation", file.Statements[0])
	}
	if formatted := gdformat.File(file); formatted != source {
		t.Errorf("formatted output changed the source: %q", formatted)
	}
}
