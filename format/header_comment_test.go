package format_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A comment that ends a block header's line says something about that line, and
// a tool reading a same-line directive such as "gdlint:ignore" finds it only
// there, so it stays on the header rather than opening the body.
func TestHeaderCommentStaysOnTheHeaderLine(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"a function",
			"func BadName():  # gdlint:ignore = function-name\n\tpass\n",
			"func BadName():  # gdlint:ignore = function-name\n\tpass\n",
		},
		{
			"the branches of an if",
			"func g(x):\n\tif x:  # why\n\t\treturn 1\n\telif x > 1:  # then\n\t\treturn 3\n\telse:  # other\n\t\treturn 2\n",
			"func g(x):\n\tif x:  # why\n\t\treturn 1\n\telif x > 1:  # then\n\t\treturn 3\n\telse:  # other\n\t\treturn 2\n",
		},
		{
			"a for and a while",
			"func g(x):\n\tfor i in x:  # each\n\t\tprint(i)\n\twhile x:  # spin\n\t\tx -= 1\n",
			"func g(x):\n\tfor i in x:  # each\n\t\tprint(i)\n\twhile x:  # spin\n\t\tx -= 1\n",
		},
		{
			"a match arm",
			"func g(x):\n\tmatch x:\n\t\t1:  # one\n\t\t\tpass\n\t\t_ when x:  # rest\n\t\t\tpass\n",
			"func g(x):\n\tmatch x:\n\t\t1:  # one\n\t\t\tpass\n\t\t_ when x:  # rest\n\t\t\tpass\n",
		},
		{
			"an inner class",
			"class Inner:  # c\n\tvar a = 1\n",
			"class Inner:  # c\n\tvar a = 1\n",
		},
		{
			"a getter and a setter",
			"var x: int:\n\tget:  # g\n\t\treturn 1\n\tset(value):  # s\n\t\tpass\n",
			"var x: int:\n\tget:  # g\n\t\treturn 1\n\tset(value):  # s\n\t\tpass\n",
		},
		{
			"a block lambda",
			"func f():\n\tvar g = func():  # c\n\t\tpass\n\treturn g\n",
			"func f():\n\tvar g = func():  # c\n\t\tpass\n\treturn g\n",
		},
		{
			// The body moves to a line of its own, and the comment stays on the
			// line it was written on, which is the header's.
			"a one-line block",
			"func g(x):\n\tif x: pass  # c\n",
			"func g(x):\n\tif x:  # c\n\t\tpass\n",
		},
		{
			"a one-line block of several statements",
			"func g(x):\n\tif x: print(1); print(2)  # c\n",
			"func g(x):\n\tif x:  # c\n\t\tprint(1)\n\t\tprint(2)\n",
		},
		{
			// Comments on lines of their own below the header still open the body.
			"followed by a comment line",
			"func f():  # c\n\t# d\n\tpass\n",
			"func f():  # c\n\t# d\n\tpass\n",
		},
		{
			"a comment line alone",
			"func f():\n\t# d\n\tpass\n",
			"func f():\n\t# d\n\tpass\n",
		},
		{
			// A comment the header's expression had no place for is not the one
			// that ended the header's line, so it still opens the body.
			"beside a comment from inside the header",
			"func g(a, b):\n\tif (a  # inner\n\t\t\tand b):  # why\n\t\tpass\n",
			"func g(a, b):\n\tif a and b:  # why\n\t\t# inner\n\t\tpass\n",
		},
		{
			"a blank line under the header is not kept",
			"func f():  # c\n\n\tpass\n",
			"func f():  # c\n\tpass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("header.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := format.File(file)
			if got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("header.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := format.File(reparsed); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}

// The parser marks the comment that ended a header's line, which is what lets
// the tree tell it from a comment on the body's first line.
func TestHeaderCommentIsMarkedInTheTree(t *testing.T) {
	file, err := parser.Parse("header.gd", []byte("func f():  # c\n\t# d\n\tpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	body := file.Statements[0].(*ast.FunctionDeclaration).Body
	header, ok := body[0].(*ast.Comment)
	if !ok || !header.TrailsHeader {
		t.Fatalf("body[0] = %#v, want the header's comment", body[0])
	}
	if own, ok := body[1].(*ast.Comment); !ok || own.TrailsHeader {
		t.Fatalf("body[1] = %#v, want a comment on its own line", body[1])
	}
	var dump strings.Builder
	if err := ast.Dump(&dump, file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump.String(), "Comment # c (trails header)\n") {
		t.Fatalf("dump does not mark the header's comment:\n%s", dump.String())
	}
	comment := ast.JSONValue(header).(map[string]any)
	if comment["trails_header"] != true {
		t.Fatalf("JSON does not mark the header's comment: %#v", comment)
	}
}

// A body built by hand may hold nothing but the header's comment, and still
// needs a statement to be a block.
func TestHeaderCommentAloneStillLeavesABody(t *testing.T) {
	file := &ast.File{Statements: []ast.Statement{&ast.FunctionDeclaration{
		Name: "f",
		Body: []ast.Statement{&ast.Comment{Text: "# c", TrailsHeader: true}},
	}}}
	if got, want := format.File(file), "func f():  # c\n\tpass\n"; got != want {
		t.Fatalf("formatted %q, want %q", got, want)
	}
}
