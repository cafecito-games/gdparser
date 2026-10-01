package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func TestMatchBindingPattern(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a binding on its own",
			source: "func a(x):\n\tmatch x:\n\t\tvar captured:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\tvar captured:\n\t\t\tpass\n",
		},
		{
			name:   "a binding beside another pattern",
			source: "func a(x):\n\tmatch x:\n\t\t1, var c:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t1, var c:\n\t\t\tpass\n",
		},
		{
			name:   "a binding with a guard",
			source: "func a(x):\n\tmatch x:\n\t\tvar c when c > 1:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\tvar c when c > 1:\n\t\t\tpass\n",
		},
		{
			name:   "a binding inside an array pattern",
			source: "func a(x):\n\tmatch x:\n\t\t[1, var rest]:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t[1, var rest]:\n\t\t\tpass\n",
		},
		{
			name:   "a binding inside a dictionary pattern",
			source: "func a(x):\n\tmatch x:\n\t\t{\"k\": var v}:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t{ \"k\": var v }:\n\t\t\tpass\n",
		},
		{
			name:   "bindings nested in both pattern kinds",
			source: "func a(x):\n\tmatch x:\n\t\t[[var a], {\"k\": [var b]}]:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t[[var a], { \"k\": [var b] }]:\n\t\t\tpass\n",
		},
		{
			name:   "the wildcard pattern is unaffected",
			source: "func a(x):\n\tmatch x:\n\t\t_:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t_:\n\t\t\tpass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("match.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("match.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestMatchBindingPatternNode(t *testing.T) {
	file, err := parser.Parse("match.gd", []byte("func a(x):\n\tmatch x:\n\t\tvar captured:\n\t\t\tpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	match := file.Statements[0].(*ast.FunctionDeclaration).Body[0].(*ast.MatchStatement)
	binding, ok := match.Cases[0].Patterns[0].(*ast.BindingPattern)
	if !ok {
		t.Fatalf("pattern 0 = %T, want *ast.BindingPattern", match.Cases[0].Patterns[0])
	}
	if binding.Name != "captured" {
		t.Errorf("name = %q, want \"captured\"", binding.Name)
	}
	if binding.KeywordSpan.Start.Line != 3 || binding.KeywordSpan.Start.Column != 3 {
		t.Errorf("keyword span start = %v, want 3:3", binding.KeywordSpan.Start)
	}
	if binding.NameSpan.Start.Column != 7 {
		t.Errorf("name span start = %v, want column 7", binding.NameSpan.Start)
	}
	if binding.Span().Start.Column != 3 || binding.Span().End.Column != 15 {
		t.Errorf("span = %v..%v, want columns 3..15", binding.Span().Start, binding.Span().End)
	}
}

// A binding belongs to a pattern, so it stays rejected everywhere else.
func TestBindingIsNotAnExpression(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"a variable's value", "func a():\n\tvar y = var z\n"},
		{"a call argument inside a pattern", "func a(x):\n\tmatch x:\n\t\tf(var z):\n\t\t\tpass\n"},
		{"an operand inside a pattern", "func a(x):\n\tmatch x:\n\t\t1 + var z:\n\t\t\tpass\n"},
		{"a guard", "func a(x):\n\tmatch x:\n\t\t1 when var z:\n\t\t\tpass\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parser.Parse("match.gd", []byte(test.source)); err == nil {
				t.Fatal("expected a parse error")
			} else if !strings.Contains(err.Error(), "match.gd:") {
				t.Fatalf("error lacks a position: %v", err)
			}
		})
	}
}
