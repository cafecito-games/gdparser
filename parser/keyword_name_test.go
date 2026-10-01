package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/parser"
)

// declaredNamePositions holds one source per position where Godot declares or
// binds a name, with NAME standing in for the name itself.
var declaredNamePositions = map[string]string{
	"a variable":         "var NAME = 1\n",
	"a constant":         "const NAME = 1\n",
	"a static variable":  "static var NAME = 1\n",
	"a function":         "func NAME():\n\tpass\n",
	"a parameter":        "func f(NAME):\n\tpass\n",
	"a setter parameter": "var a:\n\tset(NAME):\n\t\tpass\n",
	"a class":            "class NAME:\n\tpass\n",
	"a signal":           "signal NAME\n",
	"an enum":            "enum NAME { A }\n",
	"an enum member":     "enum E { NAME }\n",
	"a loop variable":    "func f():\n\tfor NAME in []:\n\t\tpass\n",
	"a lambda":           "func f():\n\tvar x = func NAME(): pass\n",
	"a type":             "var a: NAME = null\n",
	"a type after a dot": "var a: Node.NAME = null\n",
	"a return type":      "func f() -> NAME:\n\treturn null\n",
	"a cast type":        "func f():\n\tvar a = 1 as NAME\n",
	"a match bind":       "func f(v):\n\tmatch v:\n\t\tvar NAME:\n\t\t\tpass\n",
}

// Godot reserves its keywords where a name is declared or bound, so a function
// named "if" is not a function declaration. Verified against Godot 4.7.2, which
// rejects every one of these.
func TestAReservedKeywordIsNotADeclaredName(t *testing.T) {
	for _, keyword := range []string{
		"and", "as", "await", "break", "class", "class_name", "const", "continue",
		"elif", "else", "enum", "extends", "false", "for", "func", "if", "in",
		"is", "not", "null", "or", "pass", "return", "signal", "static", "true",
		"var", "while",
	} {
		for position, template := range declaredNamePositions {
			t.Run(keyword+" as "+position, func(t *testing.T) {
				source := strings.ReplaceAll(template, "NAME", keyword)
				_, err := parser.Parse("name.gd", []byte(source))
				if err == nil {
					t.Fatalf("parse succeeded for %q, want a positioned error", source)
				}
				if !strings.Contains(err.Error(), "name.gd:") {
					t.Errorf("error %q carries no position", err)
				}
			})
		}
	}
}

// Godot keeps "match" and "tool" usable as names, so they are contextual rather
// than reserved and must still be accepted wherever a name is declared.
func TestAContextualKeywordIsStillAName(t *testing.T) {
	for _, keyword := range []string{"match", "tool"} {
		for position, template := range declaredNamePositions {
			t.Run(keyword+" as "+position, func(t *testing.T) {
				source := strings.ReplaceAll(template, "NAME", keyword)
				if _, err := parser.Parse("name.gd", []byte(source)); err != nil {
					t.Fatalf("parse %q: %v", source, err)
				}
			})
		}
	}
}

// After a dot Godot accepts every keyword as a member name, because the name is
// read as text rather than as a token that could open a statement.
func TestAKeywordIsAMemberName(t *testing.T) {
	for _, keyword := range []string{
		"and", "as", "await", "break", "class", "class_name", "const", "continue",
		"elif", "else", "enum", "extends", "for", "func", "if", "in", "is",
		"match", "not", "or", "pass", "return", "signal", "static", "tool", "var",
		"while",
	} {
		t.Run(keyword, func(t *testing.T) {
			source := "func f():\n\tvar a = self." + keyword + "\n"
			if _, err := parser.Parse("name.gd", []byte(source)); err != nil {
				t.Fatalf("parse %q: %v", source, err)
			}
		})
	}
	// A literal is not a name even there, so Godot rejects it after a dot.
	for _, literal := range []string{"true", "false", "null"} {
		t.Run(literal+" is rejected", func(t *testing.T) {
			source := "func f():\n\tvar a = self." + literal + "\n"
			if _, err := parser.Parse("name.gd", []byte(source)); err == nil {
				t.Fatalf("parse succeeded for %q, want a positioned error", source)
			}
		})
	}
}

// An annotation name is read as text, so Godot accepts every keyword after '@'
// and reports an unknown annotation rather than a syntax error.
func TestAKeywordIsAnAnnotationName(t *testing.T) {
	for _, keyword := range []string{"if", "class", "true", "null", "var", "match"} {
		t.Run(keyword, func(t *testing.T) {
			source := "@" + keyword + "\nvar a = 1\n"
			if _, err := parser.Parse("name.gd", []byte(source)); err != nil {
				t.Fatalf("parse %q: %v", source, err)
			}
		})
	}
}

// The wildcard names nothing, so it is not a bind name even though it is a
// pattern of its own.
func TestTheWildcardIsNotABindName(t *testing.T) {
	source := "func f(v):\n\tmatch v:\n\t\tvar _:\n\t\t\tpass\n"
	if _, err := parser.Parse("name.gd", []byte(source)); err == nil {
		t.Fatal("parse succeeded, want a positioned error")
	}
}
