package parser_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A class written on one line holds one member. Godot's parse_class_body stops
// after the first when the body is not a block, so what a semicolon separates
// from that member belongs to the scope around the class.
func TestOneLineClassHoldsOneMember(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"a variable after the member",
			"class A: var v = 1; var u = 2\n",
			"class A:\n\tvar v = 1\n\n\nvar u = 2\n",
		},
		{
			"a function after pass",
			"class A: pass; func f(): pass\n",
			"class A:\n\tpass\n\n\nfunc f():\n\tpass\n",
		},
		{
			"with a base class",
			"class A extends RefCounted: var v = 1; signal s\n",
			"class A extends RefCounted:\n\tvar v = 1\n\n\nsignal s\n",
		},
		{
			"inside another class",
			"class O:\n\tclass A: var v = 1; var u = 2\n\tvar w = 3\n",
			"class O:\n\tclass A:\n\t\tvar v = 1\n\n\n\tvar u = 2\n\tvar w = 3\n",
		},
		{
			"a semicolon ending the line",
			"class A: pass;\nvar u = 2\n",
			"class A:\n\tpass\n\n\nvar u = 2\n",
		},
		{
			"a comment ending the line belongs to the last declaration",
			"class A: pass; var u = 2  # c\n",
			"class A:\n\tpass\n\n\nvar u = 2  # c\n",
		},
		{
			"a comment after the only member stays with the class",
			"class A: pass  # c\n",
			"class A:  # c\n\tpass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("class.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := gdformat.File(file)
			if got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("class.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := gdformat.File(reparsed); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}

func TestOneLineClassMembersInTheTree(t *testing.T) {
	file, err := parser.Parse("class.gd", []byte("class A: var v = 1; var u = 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Statements) != 2 {
		t.Fatalf("the file holds %d statements, want the class and the variable after it", len(file.Statements))
	}
	class := file.Statements[0].(*ast.ClassDeclaration)
	if len(class.Body) != 1 || class.Body[0].(*ast.VariableDeclaration).Name != "v" {
		t.Fatalf("class body = %#v, want only v", class.Body)
	}
	if outer := file.Statements[1].(*ast.VariableDeclaration); outer.Name != "u" {
		t.Fatalf("the statement after the class is %q, want u", outer.Name)
	}
}

// A one-line block that is no class keeps every statement the semicolons
// separate, as Godot's parse_suite does.
func TestOneLineBlockKeepsEveryStatement(t *testing.T) {
	file, err := parser.Parse("block.gd", []byte("func f(x):\n\tif x: print(1); print(2)\n"))
	if err != nil {
		t.Fatal(err)
	}
	branch := file.Statements[0].(*ast.FunctionDeclaration).Body[0].(*ast.IfStatement).Branches[0]
	if len(branch.Body) != 2 {
		t.Fatalf("the branch holds %d statements, want 2", len(branch.Body))
	}
}
