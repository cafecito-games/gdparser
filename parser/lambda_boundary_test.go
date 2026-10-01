package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/parser"
)

// Where a lambda body ends is a decision for the parser, not for the layout of
// the source: Godot resets its tokenizer's line-break handling for the body and
// ends the body at the first thing that could not continue it. These are the
// shapes that tells apart.
func TestLambdaBodyEndsWhereTheParserSaysItDoes(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			// The body ends at the dedent, so the "if" opens a statement rather
			// than continuing the initializer as a conditional expression.
			name:   "a statement follows a block body at the outer level",
			source: "func w():\n\tvar f = func():\n\t\tprint(1)\n\tif 0 == 0:\n\t\tf.call()\n",
		},
		{
			// The body ended inside the brackets, which have since closed, so
			// the expression carries on with the operator after them.
			name:   "an operator follows a call holding an inline body",
			source: "func w(a):\n\tif a.all(func(x) -> bool: return x) != null:\n\t\tprint(1)\n",
		},
		{
			name:   "a disjunction joins two calls holding inline bodies",
			source: "func w(a, b):\n\treturn a.any(func(x): return x) or b.any(func(x): return x)\n",
		},
		{
			// The comma ends the first body; the element after it is a whole
			// expression, not just its first operand.
			name:   "an element after a block body is read in full",
			source: "func w():\n\tvar a = [func():\n\t\tpass\n\t\t, 2 + 3]\n\tprint(a)\n",
		},
		{
			name:   "two block bodies in one array",
			source: "func w():\n\tvar a = [func():\n\t\t\tpass, func():\n\t\t\t\tpass]\n\tprint(a)\n",
		},
		{
			// A body nested at the statement level shares the indentation of the
			// one holding it, so each closes at its own dedent.
			name:   "bodies nested three deep at the statement level",
			source: "static var v = func():\n\tvar f := func():\n\t\tvar g := func():\n\t\t\tprint(1)\n\t\tg.call()\n\tf.call()\n\nfunc test():\n\tpass\n",
		},
		{
			// The statement after the nested body's bracket belongs to the outer
			// body, which has not ended.
			name:   "a statement follows a nested body's bracket",
			source: "func w(f):\n\tvar a = [func():\n\t\t\tf(func():\n\t\t\t\t\tpass\n\t\t\t)\n\t\t\tpass\n\t]\n\tprint(a)\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parser.Parse("lambda.gd", []byte(test.source)); err != nil {
				t.Fatalf("parse: %v", err)
			}
		})
	}
}

// A lambda body builds its own indentation on top of the line that opened it, so
// the line that ends the body has to line up with a block the body left open or
// with the body's own level. Godot reports "Unindent doesn't match the previous
// indentation level" for anything else, and so does this.
func TestLambdaTerminatorIndentationMustMatchAnOuterBlock(t *testing.T) {
	// The body opens blocks at two tabs, three and four, so one tab lines up
	// with nothing.
	source := "var x = [func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t, 2]\n"
	_, err := parser.Parse("lambda.gd", []byte(source))
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), "indentation does not match an outer block") {
		t.Fatalf("error does not name the rule: %v", err)
	}
	// The body's own level does line up.
	matching := "var x = [func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t\t, 2]\n"
	if _, err := parser.Parse("lambda.gd", []byte(matching)); err != nil {
		t.Fatalf("the body's own indentation was rejected: %v", err)
	}
}

// A lambda body ends at a dedent, which carries no text, so a comment written
// after it belongs to whatever scope the dedent left the parser in rather than
// trailing the statement the lambda was written in.
func TestCommentAfterABlockLambdaKeepsItsOwnLine(t *testing.T) {
	source := "var f: Callable = func(p: String) -> bool:\n\treturn exists(p)\n\n# c\nvar g := 1\n"
	file, err := parser.Parse("lambda.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Statements) != 3 {
		t.Fatalf("got %d statements, want 3", len(file.Statements))
	}
	comment, ok := file.Statements[1].(*ast.Comment)
	if !ok {
		t.Fatalf("statement 1 = %T, want a comment", file.Statements[1])
	}
	if comment.Text != "# c" {
		t.Fatalf("comment = %q, want \"# c\"", comment.Text)
	}
	if trivia := ast.TriviaOf(file.Statements[0]); trivia != nil && trivia.TrailingComment != nil {
		t.Fatal("the comment was read as trailing the declaration above it")
	}
}
