package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/parser"
)

func declaredType(t *testing.T, source string) string {
	t.Helper()
	file, err := parser.Parse("types.gd", []byte(source))
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	return file.Statements[0].(*ast.VariableDeclaration).Type
}

// A type is read with the grammar of parse_type in Godot's
// modules/gdscript/gdscript_parser.cpp, which is narrower than "whatever stands
// before the initializer".
func TestTypeGrammarFollowsGodot(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{"var a: int = 1\n", "int"},
		{"var a: Base.Inner = null\n", "Base.Inner"},
		{"var a: Array[int] = []\n", "Array[int]"},
		{"var a: Dictionary[String, Array[int]] = {}\n", "Dictionary[String, Array[int]]"},
	} {
		if got := declaredType(t, test.source); got != test.want {
			t.Errorf("%q has type %q, want %q", test.source, got, test.want)
		}
	}
}

// Godot reads a type's brackets and its dotted name as alternatives, keeps line
// breaks meaningful inside the brackets, and allows "void" only as a return type.
func TestTypeGrammarRejectsWhatGodotRejects(t *testing.T) {
	for _, test := range []struct{ name, source, rule string }{
		{
			"a bracket after a dotted name",
			"var a: Base.Generic[int] = null\n",
			"expected end of line",
		},
		{
			"a line break inside the brackets",
			"var a: Array[\n\tint] = []\n",
			"one line",
		},
		{
			"void as a declared type",
			"var a: void = null\n",
			"only names a function's return type",
		},
		{
			// extends takes a name or a path, read by parse_extends rather than
			// by the type grammar, so it carries no type arguments.
			"type arguments after extends",
			"class Inner extends Base.Generic[int]:\n\tpass\n",
			"expected ':' before block",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("types.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), test.rule) {
				t.Fatalf("error does not name the rule: %v", err)
			}
		})
	}
}

// "extends" may name a script by path, and reach an inner class of it.
func TestExtendsTakesAPathOrAName(t *testing.T) {
	for _, source := range []string{
		"class A extends \"res://base.gd\":\n\tpass\n",
		"class A extends \"res://base.gd\".Inner:\n\tpass\n",
		"class A extends Base.Inner:\n\tpass\n",
	} {
		if _, err := parser.Parse("types.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}
