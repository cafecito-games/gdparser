package parser_test

import (
	"strings"
	"testing"

	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// Brackets let a comment break a line almost anywhere, and Godot accepts every
// one of them because its tokenizer throws comments away. A formatter may not
// throw them away, and the expression grammar has no node to hang one on, so a
// comment the grammar cannot place waits for the statement to end and stands on
// its own line there, in the scope it was written in.
func TestCommentWhereTheExpressionGrammarHasNoPlace(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"between an operator and its operand",
			"func f():\n\tvar x = (1 +  # c\n\t\t\t2)\n",
			"func f():\n\tvar x = 1 + 2\n\t# c\n",
		},
		{
			"after a subscript's bracket",
			"func f(a):\n\tvar x = a[  # c\n\t\t\t0]\n",
			"func f(a):\n\tvar x = a[0]\n\t# c\n",
		},
		{
			"before a conditional's else",
			"func f(a):\n\tvar x = (1 if a  # c\n\t\t\telse 2)\n",
			"func f(a):\n\tvar x = 1 if a else 2\n\t# c\n",
		},
		{
			// A header's comment opens the block the header introduces, which is
			// where a comment ending the header's own line already goes.
			"inside a parameter's default value",
			"func f(a = 1  # c\n\t\t+ 2):\n\tpass\n",
			"func f(a = 1 + 2):\n\t# c\n\tpass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("comment.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := gdformat.File(file); got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
		})
	}
}

// A comment between the items of a bracketed construct still belongs to that
// construct, which is what keeps it on the item's own line.
func TestCommentBetweenItemsIsStillTheCollection(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{
			"func f():\n\tvar a = [1,  # c\n\t\t\t2]\n",
			"func f():\n\tvar a = [\n\t\t1,  # c\n\t\t2,\n\t]\n",
		},
		{
			"func f():\n\tvar a = [\n\t\t# c\n\t\t1,\n\t]\n",
			"func f():\n\tvar a = [\n\t\t# c\n\t\t1,\n\t]\n",
		},
	} {
		file, err := parser.Parse("comment.gd", []byte(test.source))
		if err != nil {
			t.Fatalf("parse %q: %v", test.source, err)
		}
		if got := gdformat.File(file); got != test.want {
			t.Errorf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
		}
	}
}

// A type's name may be broken by a comment on either side of one of its dots.
func TestCommentInterruptingADottedTypeName(t *testing.T) {
	for _, source := range []string{
		"func f(a: Inner  # c\n\t\t.Nested):\n\tpass\n",
		"func f(a: Inner.  # c\n\t\tNested):\n\tpass\n",
	} {
		file, err := parser.Parse("comment.gd", []byte(source))
		if err != nil {
			t.Fatalf("parse %q: %v", source, err)
		}
		formatted := gdformat.File(file)
		if !strings.Contains(formatted, "Inner.Nested") {
			t.Errorf("the type was broken up: %q", formatted)
		}
		if !strings.Contains(formatted, "# c") {
			t.Errorf("the comment was dropped: %q", formatted)
		}
	}
}

// A comment may sit between a branch's body and the keyword that continues the
// statement. The keyword's line belongs to neither block, so the comment
// introduces the branch it precedes.
func TestCommentBeforeAContinuationKeyword(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{
			"func f(a):\n\tif a:\n\t\tpass\n\t# c\n\telse:\n\t\tpass\n",
			"func f(a):\n\tif a:\n\t\tpass\n\t# c\n\telse:\n\t\tpass\n",
		},
		{
			"func f(a):\n\tif a:\n\t\tpass\n\t# one\n\t# two\n\telif a:\n\t\tpass\n",
			"func f(a):\n\tif a:\n\t\tpass\n\t# one\n\t# two\n\telif a:\n\t\tpass\n",
		},
		{
			// A comment that introduces the next statement instead is left alone.
			"func f(a):\n\tif a:\n\t\tpass\n\t# c\n\tprint(1)\n",
			"func f(a):\n\tif a:\n\t\tpass\n\t# c\n\tprint(1)\n",
		},
	} {
		file, err := parser.Parse("comment.gd", []byte(test.source))
		if err != nil {
			t.Fatalf("parse %q: %v", test.source, err)
		}
		if got := gdformat.File(file); got != test.want {
			t.Errorf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
		}
	}
}
