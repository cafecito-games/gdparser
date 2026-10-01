package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func TestVariadicParameter(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a rest parameter on its own",
			source: "func a(...rest):\n\tpass\n",
			want:   "func a(...rest):\n\tpass\n",
		},
		{
			name:   "a rest parameter after required ones",
			source: "func a(first, second, ...rest):\n\tpass\n",
			want:   "func a(first, second, ...rest):\n\tpass\n",
		},
		{
			name:   "a typed rest parameter",
			source: "func a(...rest: Array):\n\tpass\n",
			want:   "func a(...rest: Array):\n\tpass\n",
		},
		{
			name:   "a space after the dots is normalized away",
			source: "func a(... rest):\n\tpass\n",
			want:   "func a(...rest):\n\tpass\n",
		},
		{
			name:   "a rest parameter of a lambda",
			source: "var f = func(...rest):\n\tpass\n",
			want:   "var f = func(...rest):\n\tpass\n",
		},
		{
			name:   "a comment between the dots and the name",
			source: "func a(...  # dots\n\t\trest):\n\tpass\n",
			// The comment ended the opening line, so it stays on that line.
			want: "func a(  # dots\n\t\t...rest\n):\n\tpass\n",
		},
		{
			name:   "a comment before a rest parameter",
			source: "func a(first,\n\t\t# note\n\t\t...rest):\n\tpass\n",
			want:   "func a(\n\t\tfirst,\n\t\t# note\n\t\t...rest\n):\n\tpass\n",
		},
		{
			name:   "a rest parameter beside a default",
			source: "func a(first = 1, ...rest):\n\tpass\n",
			want:   "func a(first = 1, ...rest):\n\tpass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("variadic.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("variadic.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestVariadicParameterNode(t *testing.T) {
	file, err := parser.Parse("variadic.gd", []byte("func a(first, ...rest):\n\tpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	parameters := file.Statements[0].(*ast.FunctionDeclaration).Parameters
	if len(parameters) != 2 {
		t.Fatalf("got %d parameters, want 2", len(parameters))
	}
	if parameters[0].Variadic {
		t.Error("the first parameter is not variadic")
	}
	rest := parameters[1]
	if !rest.Variadic || rest.Name != "rest" {
		t.Fatalf("parameter 1 = %+v, want a variadic named rest", rest)
	}
	if rest.VariadicSpan.Start.Column != 15 {
		t.Errorf("variadic span start = %v, want column 15", rest.VariadicSpan.Start)
	}
	// The parameter's own span covers the dots that introduce it.
	if rest.Span().Start.Column != 15 || rest.Span().End.Column != 22 {
		t.Errorf("span = %v..%v, want columns 15..22", rest.Span().Start, rest.Span().End)
	}
}

// Godot rejects each of these, so the parser does too, with a position.
func TestVariadicParameterRejections(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"a parameter after a rest parameter", "func a(...rest, x):\n\tpass\n"},
		{"a second rest parameter", "func a(...a, ...b):\n\tpass\n"},
		{"a default on a rest parameter", "func a(...rest = []):\n\tpass\n"},
		{"a rest parameter of a signal", "signal s(...rest)\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parser.Parse("variadic.gd", []byte(test.source)); err == nil {
				t.Fatal("expected a parse error")
			} else if !strings.Contains(err.Error(), "variadic.gd:1:") {
				t.Fatalf("error lacks a position on line 1: %v", err)
			}
		})
	}
}
