package parser_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// Godot lets an annotation stand on its own line ahead of a match branch, which
// is how @warning_ignore reaches a pattern. parse_match collects them and hands
// them to the branch that follows.
func TestAnnotationBeforeAMatchBranch(t *testing.T) {
	source := "func w(f):\n\tmatch f:\n\t\t_:\n\t\t\tprint(0)\n\t\t@warning_ignore(\"unreachable_pattern\")\n\t\t1:\n\t\t\tprint(1)\n"
	file, err := parser.Parse("match.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	statement := file.Statements[0].(*ast.FunctionDeclaration).Body[0].(*ast.MatchStatement)
	if len(statement.Cases) != 2 {
		t.Fatalf("got %d branches, want 2", len(statement.Cases))
	}
	if len(statement.Cases[0].Annotations) != 0 {
		t.Error("the first branch picked up an annotation")
	}
	if len(statement.Cases[1].Annotations) != 1 {
		t.Fatalf("the second branch holds %d annotations, want 1", len(statement.Cases[1].Annotations))
	}
	if name := statement.Cases[1].Annotations[0].Name; name != "warning_ignore" {
		t.Errorf("annotation = %q, want warning_ignore", name)
	}
	if formatted := gdformat.File(file); formatted != source {
		t.Errorf("formatted output changed the source:\n--- got ---\n%s--- want ---\n%s", formatted, source)
	}
}

// An annotation that decorates no branch is an error rather than something to
// drop on the floor.
func TestAnnotationDecoratingNoMatchBranch(t *testing.T) {
	source := "func w(f):\n\tmatch f:\n\t\t1:\n\t\t\tprint(1)\n\t\t@warning_ignore(\"unreachable_pattern\")\n"
	if _, err := parser.Parse("match.gd", []byte(source)); err == nil {
		t.Fatal("expected a parse error")
	}
}

// A lambda may carry no body at all, which Godot reads as an empty one. The
// canonical spelling of an empty body is "pass", so the formatter writes that.
func TestLambdaWithNoBody(t *testing.T) {
	source := "func w(a = func():):\n\tvar b = (func():)\n\tprint(a, b)\n"
	file, err := parser.Parse("empty.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	formatted := gdformat.File(file)
	again, err := parser.Parse("empty.gd", []byte(formatted))
	if err != nil {
		t.Fatalf("formatted output did not parse: %v\n%s", err, formatted)
	}
	if gdformat.File(again) != formatted {
		t.Errorf("formatting is not idempotent:\n%s", formatted)
	}
}

// An abstract function declares no body, so it ends with its line and a
// semicolon may follow it, as it may any other simple statement.
func TestSemicolonAfterAnAbstractFunction(t *testing.T) {
	source := "@abstract\nclass_name A\n\n@abstract func f() -> int;\n@abstract func g() -> int; @abstract func h() -> int\n"
	file, err := parser.Parse("abstract.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	functions := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if declaration, ok := node.(*ast.FunctionDeclaration); ok && declaration.Abstract {
			functions++
		}
		return true
	})
	if functions != 3 {
		t.Fatalf("got %d abstract functions, want 3", functions)
	}
}
