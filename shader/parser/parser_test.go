package parser

import (
	"testing"

	"github.com/cafecito-games/gdparser/shader/ast"
)

func TestDirectiveNameAndBody(t *testing.T) {
	tests := []struct {
		source string
		name   string
		body   string
	}{
		{"#define MAX_LIGHTS 4", "define", "MAX_LIGHTS 4"},
		{"#define\tMAX 4", "define", "MAX 4"},
		{"#if\tdefined(FOO)", "if", "defined(FOO)"},
		{"#define\tALPHA\t0.5", "define", "ALPHA\t0.5"},
		{`#include "common.inc"`, "include", `"common.inc"`},
		{"#version", "version", ""},
	}
	for _, test := range tests {
		file, err := Parse("test.gdshader", []byte(test.source+"\n"))
		if err != nil {
			t.Fatalf("Parse(%q): %v", test.source, err)
		}
		if len(file.Items) != 1 {
			t.Fatalf("Parse(%q): %d items, want 1", test.source, len(file.Items))
		}
		directive, ok := file.Items[0].(*ast.PreprocessorDirective)
		if !ok {
			t.Fatalf("Parse(%q): item is %T, want *PreprocessorDirective", test.source, file.Items[0])
		}
		if directive.Name != test.name || directive.Body != test.body {
			t.Errorf("Parse(%q): name=%q body=%q, want name=%q body=%q", test.source, directive.Name, directive.Body, test.name, test.body)
		}
	}
}

func parseInitializer(t *testing.T, expression string) ast.Expression {
	t.Helper()
	file, err := Parse("test.gdshader", []byte("void f() { float x = "+expression+"; }"))
	if err != nil {
		t.Fatalf("Parse(%q): %v", expression, err)
	}
	function, ok := file.Items[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Fatalf("item is %T, want *FunctionDeclaration", file.Items[0])
	}
	declaration := function.Body.Statements[0].(*ast.VariableDeclaration)
	return declaration.Declarators[0].Value
}

func TestExpressionPrecedenceAndAssociativity(t *testing.T) {
	t.Run("multiplication binds tighter than addition", func(t *testing.T) {
		add, ok := parseInitializer(t, "a + b * c").(*ast.BinaryExpression)
		if !ok || add.Operator != "+" {
			t.Fatalf("root = %#v, want a +", parseInitializer(t, "a + b * c"))
		}
		mul, ok := add.Right.(*ast.BinaryExpression)
		if !ok || mul.Operator != "*" {
			t.Fatalf("right = %#v, want b * c", add.Right)
		}
	})

	t.Run("subtraction is left associative", func(t *testing.T) {
		outer, ok := parseInitializer(t, "a - b - c").(*ast.BinaryExpression)
		if !ok || outer.Operator != "-" {
			t.Fatalf("root = %#v, want a - b - c", parseInitializer(t, "a - b - c"))
		}
		inner, ok := outer.Left.(*ast.BinaryExpression)
		if !ok || inner.Operator != "-" {
			t.Fatalf("left = %#v, want a - b", outer.Left)
		}
	})

	t.Run("assignment is right associative", func(t *testing.T) {
		outer, ok := parseInitializer(t, "a = b = c").(*ast.AssignmentExpression)
		if !ok || outer.Operator != "=" {
			t.Fatalf("root = %#v, want a =", parseInitializer(t, "a = b = c"))
		}
		inner, ok := outer.Value.(*ast.AssignmentExpression)
		if !ok || inner.Operator != "=" {
			t.Fatalf("value = %#v, want b = c", outer.Value)
		}
	})

	t.Run("conditional is right associative", func(t *testing.T) {
		outer, ok := parseInitializer(t, "a ? b : c ? d : e").(*ast.ConditionalExpression)
		if !ok {
			t.Fatalf("root = %#v, want a ? b : c ? d : e", parseInitializer(t, "a ? b : c ? d : e"))
		}
		if _, ok := outer.Else.(*ast.ConditionalExpression); !ok {
			t.Fatalf("else = %#v, want c ? d : e", outer.Else)
		}
	})
}
