package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/parser"
)

// A function body holds code, not declarations. Godot's parse_statement has no
// case for static, class, class_name, extends, signal, or enum, and none of them
// opens an expression, so it reports that a statement was expected instead.
func TestFunctionBodyHoldsNoClassMembers(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"a static variable", "func f():\n\tstatic var u = 2\n"},
		{"a static function", "func f():\n\tstatic func g():\n\t\tpass\n"},
		{"an inner class", "func f():\n\tclass A:\n\t\tpass\n"},
		{"a signal", "func f():\n\tsignal s\n"},
		{"an enum", "func f():\n\tenum E { X }\n"},
		{"an extends directive", "func f():\n\textends Node\n"},
		{"a class_name directive", "func f():\n\tclass_name A\n"},
		{"inside a nested block", "func f():\n\tif true:\n\t\tsignal s\n"},
		{"a one-line body", "func f(): static var u = 2\n"},
		{"a loop body", "func f():\n\tfor i in []:\n\t\tstatic var u = 2\n"},
		{"a getter's body", "var v:\n\tget:\n\t\tsignal s\n"},
		{"a setter's body", "var v:\n\tset(value):\n\t\tclass A:\n\t\t\tpass\n"},
		{"after a semicolon on one line", "func f(): pass; class A: pass\n"},
		{"inside a one-line static function of a one-line class", "class A: static func f(): static var u = 2\n"},
		{"inside a match branch", "func f(v):\n\tmatch v:\n\t\t1:\n\t\t\tenum E { X }\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("body.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), "in a function body") {
				t.Fatalf("error does not name the rule: %v", err)
			}
		})
	}
}

// What a function body does hold keeps parsing, including the declarations a
// block shares with a class body and a lambda written as a value.
func TestFunctionBodyHoldsCode(t *testing.T) {
	for _, source := range []string{
		"func f():\n\tvar v = 1\n\tconst C = 2\n\tv += C\n",
		"func f():\n\t@warning_ignore(\"unused_variable\")\n\tvar v = 1\n",
		"func f():\n\tvar g := func(): print(1)\n\tg.call()\n",
		"func f():\n\tfor i in []:\n\t\tif i:\n\t\t\tcontinue\n\t\tbreak\n",
	} {
		if _, err := parser.Parse("body.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}

// A lambda body ends at the first thing that could not continue it, so one of
// these keywords closes the body rather than failing inside it. What is left
// then sits where no statement may begin, or leaves the block it opened
// unclosed, and Godot rejects the script either way: in a function body its
// parse_statement reports the keyword, and in a class body the abandoned indent
// survives as a dedent that reaches the "Expected end of file" check at the end
// of parse_program.
func TestClassMemberKeywordEndsALambdaBody(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"in a class body", "class A:\n\tvar f = func():\n\t\tsignal s\n"},
		{"in a class body, as a static variable", "class A:\n\tvar f = func():\n\t\tstatic var u = 2\n"},
		{"in a function body", "func f():\n\tvar x = func():\n\t\tsignal s\n"},
		{"in a function body, on one line", "func f():\n\tvar x = func(): signal s\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parser.Parse("lambda.gd", []byte(test.source)); err == nil {
				t.Fatal("expected a parse error")
			}
		})
	}
}
