package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/parser"
)

// Godot refuses a name declared twice where one declaration can see the other.
// Its parser keeps a class's members apart from a function's locals, and reaches
// a local through SuiteNode::get_local, which walks the enclosing blocks.
func TestNameDeclaredTwiceInOneScope(t *testing.T) {
	for _, test := range []struct{ name, source, rule string }{
		{
			"two locals in one block",
			"func f():\n\tvar TEST = 1\n\tvar TEST = 2\n",
			`there is already a variable named "TEST"`,
		},
		{
			"a constant over a variable",
			"func f():\n\tvar TEST = 50\n\tconst TEST = 25\n",
			`there is already a variable named "TEST"`,
		},
		{
			"a constant twice",
			"func f():\n\tconst TEST = 25\n\tconst TEST = 50\n",
			`there is already a constant named "TEST"`,
		},
		{
			// A nested block sees the one around it, so it cannot reuse the name.
			"a local in a nested block",
			"func f():\n\tvar TEST = 1\n\tif true:\n\t\tvar TEST = 2\n",
			`there is already a variable named "TEST"`,
		},
		{
			"a loop variable over a local",
			"func f():\n\tvar TEST = 1\n\tfor TEST in 2:\n\t\tpass\n",
			`there is already a variable named "TEST"`,
		},
		{
			"a local over a parameter",
			"func f(a):\n\tvar a = 1\n",
			`there is already a parameter named "a"`,
		},
		{
			// A lambda body does see the locals around it.
			"a local in a lambda over one outside",
			"func f():\n\tvar TEST = 1\n\tvar g := func():\n\t\tvar TEST = 2\n",
			`there is already a variable named "TEST"`,
		},
		{
			// A parameter may shadow a name outside the lambda, and a local of
			// the body then meets the parameter.
			"a local in a lambda over the lambda's parameter",
			"func f(a):\n\tvar g := func(a):\n\t\tvar a = 2\n",
			`there is already a parameter named "a"`,
		},
		{
			"a parameter twice in one function",
			"func f(a, a):\n\tpass\n",
			`there is already a parameter named "a"`,
		},
		{
			"a parameter twice in one lambda",
			"func f():\n\tvar g := func(a, a): return a\n",
			`there is already a parameter named "a"`,
		},
		{
			"a member over an earlier member",
			"func test():\n\tpass\n\nvar test = 25\n",
			`variable "test" has the same name as a previously declared function`,
		},
		{
			"a function over an earlier member",
			"var test = 25\n\nfunc test():\n\tpass\n",
			`function "test" has the same name as a previously declared variable`,
		},
		{
			"an inner class over a member",
			"var Inner = 1\n\nclass Inner:\n\tpass\n",
			`class "Inner" has the same name as a previously declared variable`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("scope.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), test.rule) {
				t.Fatalf("error does not name the rule: %v", err)
			}
		})
	}
}

// A name is free again where the scope that held it has closed, and a class's
// members live apart from the locals of its functions.
func TestNameReusedWhereGodotAllowsIt(t *testing.T) {
	for _, source := range []string{
		// A local may carry a member's name.
		"var TEST = 1\n\nfunc f():\n\tvar TEST = 2\n\tprint(TEST)\n",
		// Two functions each have their own scope.
		"func f():\n\tvar TEST = 1\n\tprint(TEST)\n\nfunc g():\n\tvar TEST = 2\n\tprint(TEST)\n",
		// Sibling blocks do not see each other.
		"func f(c):\n\tif c:\n\t\tvar TEST = 1\n\t\tprint(TEST)\n\telse:\n\t\tvar TEST = 2\n\t\tprint(TEST)\n",
		// A loop variable lives in the body, so the name is free afterwards.
		"func f():\n\tfor i in 2:\n\t\tprint(i)\n\tvar i = 1\n\tprint(i)\n",
		// A pattern bind lives in the branch it belongs to.
		"func f(v):\n\tmatch v:\n\t\tvar a:\n\t\t\tprint(a)\n\t\t_:\n\t\t\tvar a = 1\n\t\t\tprint(a)\n",
		// An inner class has a member table of its own.
		"var TEST = 1\n\nclass Inner:\n\tvar TEST = 2\n",
		// An anonymous enum declares no name.
		"enum { A }\nenum { B }\n",
		// A getter and a setter each open a scope of their own.
		"var x: int = 1:\n\tget:\n\t\tvar v = 1\n\t\treturn v\n\tset(value):\n\t\tvar v = value\n\t\tprint(v)\n",
		// A parameter is held only against the others of its own list, so a
		// lambda's may carry the name of a parameter of the function around it,
		"func outer(x):\n\tvar callback = func(x): return x\n\treturn callback\n",
		// or of a local,
		"func outer():\n\tvar x = 1\n\tvar callback = func(x): return x\n\treturn callback.call(x)\n",
		// or of a loop variable,
		"func outer():\n\tfor i in 2:\n\t\tvar g = func(i): return i\n\t\tprint(g)\n",
		// or of a parameter of the lambda around it.
		"func outer():\n\tvar g = func(x): return func(x): return x\n\treturn g\n",
	} {
		if _, err := parser.Parse("scope.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}
