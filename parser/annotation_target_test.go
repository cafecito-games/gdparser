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
		{
			source: "class Inner:\n\tvar a = 1\n\t@warning_ignore(\"unused_variable\")\nclass Other: var b = 2\n",
			want:   "class Inner:\n\tvar a = 1\n\n\n@warning_ignore(\"unused_variable\")\nclass Other:\n\tvar b = 2\n",
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

// A body written on the line of its header is the next place an annotation left
// waiting by the block before it can land, which is where Godot's parse_suite
// reads the statement that pops it off the annotation stack. The else, elif and
// match branches that follow a block are all written this way.
func TestAnnotationDecoratingAOneLineBody(t *testing.T) {
	for _, test := range []struct {
		source string
		want   string
	}{
		{
			source: "func f(a):\n\tif a:\n\t\t@warning_ignore(\"unused_variable\")\n\telse: var q = 1\n",
			want:   "func f(a):\n\tif a:\n\t\tpass\n\telse:\n\t\t@warning_ignore(\"unused_variable\")\n\t\tvar q = 1\n",
		},
		{
			source: "func f(a, b):\n\tif a:\n\t\t@warning_ignore(\"unused_variable\")\n\telif b: var q = 1\n",
			want:   "func f(a, b):\n\tif a:\n\t\tpass\n\telif b:\n\t\t@warning_ignore(\"unused_variable\")\n\t\tvar q = 1\n",
		},
		{
			source: "func f(a):\n\tmatch a:\n\t\t1:\n\t\t\t@warning_ignore(\"unused_variable\")\n\t\t2: var q = 1\n",
			want:   "func f(a):\n\tmatch a:\n\t\t1:\n\t\t\tpass\n\t\t2:\n\t\t\t@warning_ignore(\"unused_variable\")\n\t\t\tvar q = 1\n",
		},
		{
			source: "func f(a):\n\tif a:\n\t\t@warning_ignore(\"unused_variable\")\n\telse: var q = 1; var w = 2\n",
			want:   "func f(a):\n\tif a:\n\t\tpass\n\telse:\n\t\t@warning_ignore(\"unused_variable\")\n\t\tvar q = 1\n\t\tvar w = 2\n",
		},
		{
			source: "func f(a):\n\tif a:\n\t\t@warning_ignore(\"unused_variable\")\n\telse: print(1)\n",
			want:   "func f(a):\n\tif a:\n\t\tpass\n\telse:\n\t\t@warning_ignore(\"unused_variable\")\n\t\tprint(1)\n",
		},
	} {
		file, err := parser.Parse("oneline.gd", []byte(test.source))
		if err != nil {
			t.Errorf("parse %q: %v", test.source, err)
			continue
		}
		formatted := gdformat.File(file)
		if formatted != test.want {
			t.Errorf("formatted %q as %q, want %q", test.source, formatted, test.want)
		}
		again, err := parser.Parse("oneline.gd", []byte(formatted))
		if err != nil {
			t.Errorf("formatted %q did not parse: %v", formatted, err)
			continue
		}
		if second := gdformat.File(again); second != formatted {
			t.Errorf("formatting %q is not idempotent: %q", test.source, second)
		}
	}
}

// The annotation goes to the statement that follows it, which is the one the
// body on the header's line holds, not the statement after the whole block.
func TestAnnotationLandsInTheBranchThatFollowsIt(t *testing.T) {
	source := "func f(a):\n\tif a:\n\t\t@warning_ignore(\"unused_variable\")\n\telse: var q = 1\n\tvar r = 2\n"
	file, err := parser.Parse("branch.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	body := file.Statements[0].(*ast.FunctionDeclaration).Body
	statement := body[0].(*ast.IfStatement)
	inBranch, ok := statement.Else[0].(*ast.VariableDeclaration)
	if !ok {
		t.Fatalf("the else branch opens with %T, want a variable declaration", statement.Else[0])
	}
	if len(inBranch.Annotations) != 1 {
		t.Fatalf("%q carries %d annotations, want 1", inBranch.Name, len(inBranch.Annotations))
	}
	after, ok := body[1].(*ast.VariableDeclaration)
	if !ok {
		t.Fatalf("the statement after the branch is %T, want a variable declaration", body[1])
	}
	if len(after.Annotations) != 0 {
		t.Errorf("%q picked up %d annotations", after.Name, len(after.Annotations))
	}
}
