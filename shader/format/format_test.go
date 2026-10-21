package format

import (
	"testing"

	"github.com/cafecito-games/gdparser/shader/ast"
	"github.com/cafecito-games/gdparser/shader/parser"
)

// formatExpression parses source as a single initializer expression and returns
// its canonical form, so the formatter's precedence and associativity handling
// can be exercised in isolation.
func formatExpression(t *testing.T, source string) string {
	t.Helper()
	file, err := parser.Parse("test.gdshader", []byte("void f() { float x = "+source+"; }"))
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	function := file.Items[0].(*ast.FunctionDeclaration)
	declaration := function.Body.Statements[0].(*ast.VariableDeclaration)
	var f formatter
	f.expression(declaration.Declarators[0].Value, 0, false)
	return f.b.String()
}

func TestExpressionParenthesesPreserveSemantics(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"a + b * c", "a + b * c"},
		{"a * (b + c)", "a * (b + c)"},
		{"(a + b) * c", "(a + b) * c"},
		{"a - b - c", "a - b - c"},
		{"a - (b - c)", "a - (b - c)"},
		{"a + b - c", "a + b - c"},
		{"a + (b - c)", "a + (b - c)"},
		{"a = b = c", "a = b = c"},
		{"a ? b : c ? d : e", "a ? b : c ? d : e"},
		{"-a * b", "-a * b"},
		{"-(a * b)", "-(a * b)"},
		{"a && b || c", "a && b || c"},
		{"(a || b) && c", "(a || b) && c"},
	}
	for _, test := range tests {
		if got := formatExpression(t, test.source); got != test.want {
			t.Errorf("format(%q) = %q, want %q", test.source, got, test.want)
		}
	}
}
