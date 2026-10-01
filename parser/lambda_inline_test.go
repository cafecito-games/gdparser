package parser_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// Inline says that a lambda's body can stay on the lambda's line. A body that
// opens a block of its own cannot, so the lambda is written out as a block and
// reads back the same.
func TestLambdaInlineSurvivesFormatting(t *testing.T) {
	for _, test := range []struct {
		name, source string
		inline       bool
	}{
		{"a simple body", "func f(a):\n\tvar c = func(): return a\n\treturn c\n", true},
		{"several statements", "func f(a):\n\tvar c = func(): print(a); return a\n\treturn c\n", true},
		{"an if as the body", "func f(a):\n\tvar c = func(): if a: return 1\n\treturn c\n", false},
		{"a for as the body", "func f(a):\n\tvar c = func(): for i in a: print(i)\n\treturn c\n", false},
		{"a block body", "func f(a):\n\tvar c = func():\n\t\treturn a\n\treturn c\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("lambda.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := lambdaOf(t, file).Inline; got != test.inline {
				t.Fatalf("inline = %v, want %v", got, test.inline)
			}
			formatted := gdformat.File(file)
			reparsed, err := parser.Parse("lambda.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("reparse: %v\n%s", err, formatted)
			}
			if got := lambdaOf(t, reparsed).Inline; got != test.inline {
				t.Fatalf("inline after formatting = %v, want %v\n%s", got, test.inline, formatted)
			}
		})
	}
}

func lambdaOf(t *testing.T, file *ast.File) *ast.LambdaExpression {
	t.Helper()
	var found *ast.LambdaExpression
	ast.Inspect(file, func(node ast.Node) bool {
		if lambda, ok := node.(*ast.LambdaExpression); ok && found == nil {
			found = lambda
		}
		return true
	})
	if found == nil {
		t.Fatal("no lambda in the tree")
	}
	return found
}
