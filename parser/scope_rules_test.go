package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/parser"
)

// A class body holds declarations, not code. Godot's parse_class_body takes var,
// const, signal, func, class, enum, static, an annotation, "pass", and a string
// standing in for a block comment, and nothing else.
func TestClassBodyHoldsOnlyDeclarations(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"an assignment at file level", "var error\nerror = true\n"},
		{"a call at file level", "print(1)\n"},
		{"an if at file level", "if true:\n\tpass\n"},
		{"a return at file level", "return 1\n"},
		{"a for inside an inner class", "class A:\n\tfor i in []:\n\t\tpass\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("class.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), "in a class body") {
				t.Fatalf("error does not name the rule: %v", err)
			}
		})
	}
	for _, source := range []string{
		"@tool\nextends Node\nclass_name A\n\n@export var a := 1\nconst B := 2\nsignal s\nenum E { X }\n\nstatic var c := 3\n\nfunc f():\n\tpass\n\nclass Inner:\n\tpass\n",
		"pass\n",
		// A string on its own stands in for a block comment.
		"\"\"\"a block comment\"\"\"\nvar a := 1\n",
	} {
		if _, err := parser.Parse("class.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}

// "break" and "continue" belong inside a loop. A lambda body is outside every
// loop around it, which Godot marks by clearing can_break and can_continue when
// it reads one.
func TestBreakAndContinueNeedALoop(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"break at function level", "func f():\n\tbreak\n"},
		{"continue at function level", "func f():\n\tcontinue\n"},
		{"break inside a match", "func f(v):\n\tmatch v:\n\t\t1:\n\t\t\tbreak\n"},
		{"continue inside a lambda in a loop", "func f():\n\tfor i in []:\n\t\tvar g := func():\n\t\t\tcontinue\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("loop.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), "only allowed inside a loop") {
				t.Fatalf("error does not name the rule: %v", err)
			}
		})
	}
	for _, source := range []string{
		"func f():\n\tfor i in []:\n\t\tbreak\n",
		"func f():\n\twhile true:\n\t\tcontinue\n",
		// A match branch does not close the loop around it.
		"func f(v):\n\twhile true:\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tbreak\n",
		// Nor does an if.
		"func f(v):\n\tfor i in []:\n\t\tif v:\n\t\t\tcontinue\n",
	} {
		if _, err := parser.Parse("loop.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}

// A constructor is called by the engine rather than by a caller: it hands no
// value back, a static one must say it is static, and it takes no arguments.
func TestConstructorRules(t *testing.T) {
	for _, test := range []struct{ name, source, rule string }{
		{
			"a static constructor that is not static",
			"func _static_init():\n\tpass\n",
			"must be declared static",
		},
		{
			"a static constructor with a parameter",
			"static func _static_init(a) -> void:\n\tpass\n",
			"takes no parameters",
		},
		{
			"a static constructor with a rest parameter",
			"static func _static_init(...args) -> void:\n\tpass\n",
			"takes no parameters",
		},
		{
			"a static constructor returning a value",
			"static func _static_init():\n\treturn true\n",
			"cannot return a value",
		},
		{
			"a constructor returning a value",
			"func _init():\n\treturn true\n",
			"cannot return a value",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("ctor.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), test.rule) {
				t.Fatalf("error does not name the rule: %v", err)
			}
		})
	}
	for _, source := range []string{
		"static func _static_init() -> void:\n\tpass\n",
		// A bare return hands nothing back.
		"func _init():\n\treturn\n",
		// A lambda is a function of its own, so the rule does not reach into one.
		"func _init():\n\tvar f := func():\n\t\treturn true\n\tf.call()\n",
		// The names are only special on a function of the class.
		"func f():\n\tvar _init := 1\n\treturn _init\n",
	} {
		if _, err := parser.Parse("ctor.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}

// "class_name" takes one identifier and "extends" takes a path or a name, which
// Godot reads with parse_class_name and parse_extends rather than as expressions.
func TestDirectivesTakeNamesNotExpressions(t *testing.T) {
	for _, source := range []string{
		"class_name true\n",
		"class_name 1\n",
		"extends true\n",
		"extends 1\n",
		"extends Base[int]\n",
	} {
		if _, err := parser.Parse("directive.gd", []byte(source)); err == nil {
			t.Errorf("%q was accepted", source)
		}
	}
	for _, source := range []string{
		"class_name X\n",
		"class_name X extends Y\n",
		"class_name X extends Y.Inner\n",
		// Godot keeps "match" usable as a name, here as everywhere else.
		"class_name match\n",
		"extends Base.Inner\n",
		"extends \"res://base.gd\"\n",
		"extends \"res://base.gd\".Inner\n",
	} {
		if _, err := parser.Parse("directive.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}
