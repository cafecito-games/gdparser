package parser_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func TestUntypedPropertyAccessors(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "only a setter",
			source: "var x:\n\tset(value):\n\t\tpass\n",
			want:   "var x:\n\tset(value):\n\t\tpass\n",
		},
		{
			name:   "only a getter",
			source: "var x:\n\tget:\n\t\treturn 1\n",
			want:   "var x:\n\tget:\n\t\treturn 1\n",
		},
		{
			name:   "both accessors",
			source: "var x:\n\tget:\n\t\treturn 1\n\tset(value):\n\t\tpass\n",
			want:   "var x:\n\tget:\n\t\treturn 1\n\tset(value):\n\t\tpass\n",
		},
		{
			name:   "an initializer and an accessor",
			source: "var x = 1:\n\tget:\n\t\treturn 1\n",
			want:   "var x = 1:\n\tget:\n\t\treturn 1\n",
		},
		{
			name:   "a typed property is unchanged",
			source: "var x: int:\n\tset(value):\n\t\tpass\n",
			want:   "var x: int:\n\tset(value):\n\t\tpass\n",
		},
		{
			name:   "an annotated untyped property",
			source: "@export var x:\n\tget:\n\t\treturn 1\n",
			want:   "@export var x:\n\tget:\n\t\treturn 1\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("accessor.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("accessor.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestUntypedPropertyLeavesTheTypeEmpty(t *testing.T) {
	file, err := parser.Parse("accessor.gd", []byte("var x:\n\tset(value):\n\t\tpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	declaration := file.Statements[0].(*ast.VariableDeclaration)
	if declaration.Type != "" {
		t.Errorf("type = %q, want empty", declaration.Type)
	}
	if declaration.TypeSpan != (ast.VariableDeclaration{}).TypeSpan {
		t.Errorf("type span = %v, want the zero value", declaration.TypeSpan)
	}
	if declaration.Setter == nil {
		t.Fatal("the setter is missing")
	}
	if declaration.Setter.Parameter != "value" {
		t.Errorf("setter parameter = %q, want \"value\"", declaration.Setter.Parameter)
	}
}

// A colon with no accessor block after it is not a declaration Godot accepts.
func TestBarePropertyColonIsRejected(t *testing.T) {
	if _, err := parser.Parse("accessor.gd", []byte("var x:\n")); err == nil {
		t.Fatal("expected a parse error")
	}
}
