package parser_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// shapeOf returns a fully parenthesized spelling of expr, so a test can state
// the grouping it expects without depending on the formatter's own choices.
func shapeOf(expr ast.Expression) string {
	switch node := expr.(type) {
	case *ast.BinaryExpression:
		return "(" + shapeOf(node.Left) + " " + node.Operator + " " + shapeOf(node.Right) + ")"
	case *ast.UnaryExpression:
		return "(" + node.Operator + " " + shapeOf(node.Operand) + ")"
	case *ast.TernaryExpression:
		return "(" + shapeOf(node.Value) + " if " + shapeOf(node.Condition) + " else " + shapeOf(node.Alternative) + ")"
	case *ast.Identifier:
		return node.Name
	case *ast.Literal:
		return node.Raw
	case *ast.TypeExpression:
		return node.Name
	case *ast.CallExpression:
		return shapeOf(node.Callee) + "()"
	case *ast.MemberExpression:
		return shapeOf(node.Object) + "." + node.Property
	}
	return "?"
}

func valueShape(t *testing.T, source string) string {
	t.Helper()
	file, err := parser.Parse("precedence.gd", []byte("var x = "+source+"\n"))
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	return shapeOf(file.Statements[0].(*ast.VariableDeclaration).Value)
}

// Godot gives every operator a level in one ladder, declared as the Precedence
// enum in modules/gdscript/gdscript_parser.h, and reads every binary operator
// left-associatively. These are the places where that ladder is surprising.
func TestOperatorPrecedenceFollowsGodot(t *testing.T) {
	cases := []struct{ source, want string }{
		// Every binary operator associates to the left, "**" included.
		{"2 ** 3 ** 4", "((2 ** 3) ** 4)"},
		{"a - b - c", "((a - b) - c)"},
		// "as" binds looser than everything, so it casts the whole expression.
		{"a or b as int", "((a or b) as int)"},
		{"a + b as int", "((a + b) as int)"},
		// "is" binds tighter than every arithmetic operator.
		{"a ** b is int", "(a ** (b is int))"},
		{"a + b is int", "(a + (b is int))"},
		// "in" sits between the logical operators and the comparisons.
		{"a in b == c", "(a in (b == c))"},
		{"a and b in c", "(a and (b in c))"},
		// "not" and "!" are one level, tighter than "and" but looser than "==".
		{"not a == b", "(not (a == b))"},
		{"!a == b", "(! (a == b))"},
		{"not a and b", "((not a) and b)"},
		// A sign reaches past "*" but not past "**"; "~" reaches past neither.
		{"-a * b", "((- a) * b)"},
		{"-a ** b", "(- (a ** b))"},
		{"~a ** b", "(~ (a ** b))"},
		// "await" binds tighter than "**" but looser than a call.
		{"await a ** b", "((await a) ** b)"},
		{"await a()", "(await a())"},
		// The conditional takes the whole expression to its left, and nests to
		// the right without parentheses.
		{"a or b if c else d", "((a or b) if c else d)"},
		{"a if b else c if d else e", "(a if b else (c if d else e))"},
	}
	for _, testCase := range cases {
		if got := valueShape(t, testCase.source); got != testCase.want {
			t.Errorf("%s parsed as %s, want %s", testCase.source, got, testCase.want)
		}
	}
}

// Formatting may only drop a grouping the ladder makes redundant, so every
// spelling above has to survive a format and reparse with its shape intact.
func TestFormattingKeepsExpressionGrouping(t *testing.T) {
	sources := []string{
		"2 ** 3 ** 4", "2 ** (3 ** 4)", "(a or b) as int", "a or (b as int)",
		"a ** b is int", "(a ** b) is int", "a in (b == c)", "(a in b) == c",
		"not (a and b)", "(not a) == b", "not a == b", "-(a * b)", "(-a) ** b",
		"-a ** b", "(await a) ** b", "a if b else (c if d else e)",
		"(a if b else c) if d else e",
	}
	for _, source := range sources {
		before := valueShape(t, source)
		file, err := parser.Parse("precedence.gd", []byte("var x = "+source+"\n"))
		if err != nil {
			t.Fatalf("parse %q: %v", source, err)
		}
		formatted := gdformat.File(file)
		again, err := parser.Parse("precedence.gd", []byte(formatted))
		if err != nil {
			t.Fatalf("formatted %q did not parse: %v\n%s", source, err, formatted)
		}
		after := shapeOf(again.Statements[0].(*ast.VariableDeclaration).Value)
		if before != after {
			t.Errorf("%s was formatted to %q, which parses as %s, want %s", source, formatted, after, before)
		}
	}
}
