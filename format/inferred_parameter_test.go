package format_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A default written with ":=" gives the parameter the type of its value, and one
// written with "=" leaves it untyped, so the two are different programs and the
// operator is emitted as it was written.
func TestInferredParameterDefaultKeepsItsOperator(t *testing.T) {
	for _, source := range []string{
		"func f(a := 1):\n\treturn a\n",
		"func f(a = 1):\n\treturn a\n",
		"func f(a: int = 1, b := 2.0, c = 3):\n\treturn a\n",
		"func f():\n\tvar g = func(a := 1): return a\n\treturn g\n",
	} {
		file, err := parser.Parse("infer.gd", []byte(source))
		if err != nil {
			t.Fatalf("parse %q: %v", source, err)
		}
		if got := format.File(file); got != source {
			t.Errorf("formatted %q, want %q", got, source)
		}
	}
}

// Godot reads the colon and the "=" separately, so the two may be written apart,
// and the declaration is the same one.
func TestInferredParameterDefaultWrittenApart(t *testing.T) {
	file, err := parser.Parse("infer.gd", []byte("func f(a: = 1):\n\treturn a\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := format.File(file), "func f(a := 1):\n\treturn a\n"; got != want {
		t.Fatalf("formatted %q, want %q", got, want)
	}
}

// The tree says which operator a default was written with, so a comparison of
// two trees that sets spans aside still tells the two programs apart.
func TestInferredParameterDefaultIsMarkedInTheTree(t *testing.T) {
	file, err := parser.Parse("infer.gd", []byte("func f(a := 1, b = 2):\n\tpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	function := file.Statements[0].(*ast.FunctionDeclaration)
	if !function.Parameters[0].Inferred || function.Parameters[1].Inferred {
		t.Fatalf("inferred = %v, %v, want true, false", function.Parameters[0].Inferred, function.Parameters[1].Inferred)
	}
	parameters := ast.JSONValue(file).(map[string]any)["statements"].([]any)[0].(map[string]any)["parameters"].([]any)
	if parameters[0].(map[string]any)["inferred"] != true {
		t.Fatalf("JSON does not mark the inferred default: %#v", parameters[0])
	}
	if _, marked := parameters[1].(map[string]any)["inferred"]; marked {
		t.Fatalf("JSON marks a plain default as inferred: %#v", parameters[1])
	}
}
