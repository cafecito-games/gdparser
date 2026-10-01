package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func TestLuaStyleDictionary(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "one entry",
			source: "var d = {x = 1}\n",
			want:   "var d = { x = 1 }\n",
		},
		{
			name:   "several entries",
			source: "var d = {x = 1, y = 2}\n",
			want:   "var d = { x = 1, y = 2 }\n",
		},
		{
			name:   "a trailing comma",
			source: "var d = {x = 1,}\n",
			want:   "var d = { x = 1 }\n",
		},
		{
			name:   "written across lines",
			source: "var d = {\n\tx = 1,\n\ty = 2,\n}\n",
			want:   "var d = { x = 1, y = 2 }\n",
		},
		{
			name:   "a string key",
			source: "var d = {\"x\" = 1}\n",
			want:   "var d = { \"x\" = 1 }\n",
		},
		{
			name:   "nested in both directions",
			source: "var d = {outer = {\"inner\": {deep = 1}}}\n",
			want:   "var d = { outer = { \"inner\": { deep = 1 } } }\n",
		},
		{
			name:   "as a call argument",
			source: "foo({x = 1})\n",
			want:   "foo({ x = 1 })\n",
		},
		{
			name:   "the colon style is unchanged",
			source: "var d = {\"x\": 1}\n",
			want:   "var d = { \"x\": 1 }\n",
		},
		{
			name:   "the empty literal has no style",
			source: "var d = {}\n",
			want:   "var d = {}\n",
		},
		{
			name:   "several entries with mixed key kinds",
			source: "var d = {x = 1, \"y\" = 2, z = 3}\n",
			want:   "var d = { x = 1, \"y\" = 2, z = 3 }\n",
		},
		{
			name:   "a dictionary pattern keeps its colons",
			source: "func f(v):\n\tmatch v:\n\t\t{\"x\": var n}:\n\t\t\tpass\n",
			want:   "func f(v):\n\tmatch v:\n\t\t{ \"x\": var n }:\n\t\t\tpass\n",
		},
		{
			name:   "a comment inside a lua style literal",
			source: "var d = {\n\tx = 1,\n\t# c\n\ty = 2,\n}\n",
			want:   "var d = {\n\tx = 1,\n\t# c\n\ty = 2,\n}\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("lua.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("lua.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestLuaStyleDictionaryNode(t *testing.T) {
	file, err := parser.Parse("lua.gd", []byte("var d = {x = 1}\n"))
	if err != nil {
		t.Fatal(err)
	}
	dictionary := file.Statements[0].(*ast.VariableDeclaration).Value.(*ast.DictionaryLiteral)
	if !dictionary.LuaStyle {
		t.Error("LuaStyle should report the \"{key = value}\" spelling")
	}
	if len(dictionary.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(dictionary.Entries))
	}
	entry := dictionary.Entries[0]
	if identifier, ok := entry.Key.(*ast.Identifier); !ok || identifier.Name != "x" {
		t.Fatalf("key = %#v, want the identifier x", entry.Key)
	}
	if entry.SeparatorSpan.Start.Column != 12 {
		t.Errorf("separator span start = %v, want column 12", entry.SeparatorSpan.Start)
	}

	colon, err := parser.Parse("lua.gd", []byte("var d = {\"x\": 1}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if colon.Statements[0].(*ast.VariableDeclaration).Value.(*ast.DictionaryLiteral).LuaStyle {
		t.Error("LuaStyle should be false for the colon spelling")
	}
}

// A dictionary pattern is written with colons only, as Godot's pattern grammar
// has no assignment in it. A key may stand alone there, so the "=" is what ends
// the pattern short rather than the missing colon.
func TestLuaStyleDictionaryPatternIsRejected(t *testing.T) {
	source := "func f(v):\n\tmatch v:\n\t\t{x = 1}:\n\t\t\tpass\n"
	_, err := parser.Parse("lua.gd", []byte(source))
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), "expected '}' after dictionary pattern") {
		t.Fatalf("error does not name the rule: %v", err)
	}
	if !strings.Contains(err.Error(), "lua.gd:3:") {
		t.Fatalf("error lacks a position: %v", err)
	}
}

// Godot allows one style per literal, and names a Lua-style key with an
// identifier or a string, so the parser holds to both rules.
func TestLuaStyleDictionaryRejections(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{"lua then colon", "var d = {x = 1, \"y\": 2}\n", "expected '='"},
		{"colon then lua", "var d = {\"y\": 2, x = 1}\n", "expected ':'"},
		{"a number as a lua key", "var d = {1 = 2}\n", "identifier or a string"},
		{"a call as a lua key", "var d = {f() = 2}\n", "identifier or a string"},
		{"a bad key in a later entry", "var d = {x = 1, 2 = 2}\n", "identifier or a string"},
		{"a member key in a later entry", "var d = {x = 1, y.z = 2}\n", "identifier or a string"},
		{"a string name key in a later entry", "var d = {x = 1, &\"s\" = 2}\n", "identifier or a string"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("lua.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error does not name the rule: %v", err)
			}
			if !strings.Contains(err.Error(), "lua.gd:1:") {
				t.Fatalf("error lacks a position: %v", err)
			}
		})
	}
}
