package format_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A block lambda that ends a bracketed construct leaves its block open, and the
// engine closes it only at an indentation it already knows. The bracket of a
// construct nested in another sits at a continuation indentation instead, so
// the comma after the lambda is what ends its block, whatever the comma style.
func TestBlockLambdaEndingANestedCollectionKeepsItsComma(t *testing.T) {
	noCommas := format.GodotStyle()
	noCommas.TrailingCommas = format.NoTrailingCommas
	for _, test := range []struct {
		name    string
		options format.Options
		source  string
		want    string
	}{
		{
			"an array in an array without trailing commas",
			noCommas,
			"var items = [\n\t[\n\t\tfunc():\n\t\t\tpass,\n\t],\n]\n",
			"var items = [\n\t[\n\t\tfunc():\n\t\t\tpass,\n\t]\n]\n",
		},
		{
			"a dictionary in an array without trailing commas",
			noCommas,
			"var items = [\n\t{\n\t\t\"a\": func():\n\t\t\tpass,\n\t},\n]\n",
			"var items = [\n\t{\n\t\t\"a\": func():\n\t\t\tpass,\n\t}\n]\n",
		},
		{
			"a call in a call",
			format.GodotStyle(),
			"func f():\n\tg(h(func():\n\t\tpass))\n",
			"func f():\n\tg(\n\t\t\th(\n\t\t\t\t\tfunc():\n\t\t\t\t\t\tpass,\n\t\t\t)\n\t)\n",
		},
		{
			"a call in a call with arguments around it",
			format.GodotStyle(),
			"func f():\n\tg(1, h(2, func():\n\t\tpass\n\t), 3)\n",
			"func f():\n\tg(\n\t\t\t1,\n\t\t\th(\n\t\t\t\t\t2,\n\t\t\t\t\tfunc():\n\t\t\t\t\t\tpass,\n\t\t\t),\n\t\t\t3\n\t)\n",
		},
		{
			// The chain leaves the break to the call's brackets and stays whole,
			// so the bracket lands at the statement's own indentation and needs
			// no comma before it, exactly as it would with no chain around it.
			"a call in a chain that stays whole",
			format.GodotStyle(),
			"func f(a):\n\tvar x = a + g(func():\n\t\tpass)\n",
			"func f(a):\n\tvar x = a + g(\n\t\t\tfunc():\n\t\t\t\tpass\n\t)\n",
		},
		{
			"a call in a logical chain",
			format.GodotStyle(),
			"func f(a):\n\tvar x = a and h(func():\n\t\tpass)\n",
			"func f(a):\n\tvar x = (\n\t\t\ta\n\t\t\tand h(\n\t\t\t\t\tfunc():\n\t\t\t\t\t\tpass,\n\t\t\t)\n\t)\n",
		},
		{
			// The bracket lands on the statement's own indentation, which the
			// engine knows, so no comma is needed.
			"a call at the statement's level takes none",
			format.GodotStyle(),
			"func f():\n\tg(func():\n\t\tpass)\n",
			"func f():\n\tg(\n\t\t\tfunc():\n\t\t\t\tpass\n\t)\n",
		},
		{
			// A lambda body is a block of its own, so a call written in it is at
			// a statement's level again.
			"a call inside a nested lambda's body takes none",
			format.GodotStyle(),
			"func f():\n\tg(h(func():\n\t\tk(func():\n\t\t\tpass)))\n",
			"func f():\n\tg(\n\t\t\th(\n\t\t\t\t\tfunc():\n\t\t\t\t\t\tk(\n\t\t\t\t\t\t\t\tfunc():\n\t\t\t\t\t\t\t\t\tpass\n\t\t\t\t\t\t),\n\t\t\t)\n\t)\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("nested.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := format.FileWithOptions(file, test.options)
			if got != test.want {
				t.Errorf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("nested.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v\n%s", err, got)
			}
			if again := format.FileWithOptions(reparsed, test.options); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}

// A subscript and a property's accessor colon follow an expression with no
// comma between, so a block lambda written there is parenthesized to end its
// body before they are read.
func TestBlockLambdaBeforeAClosingTokenIsParenthesized(t *testing.T) {
	for _, source := range []string{
		"func f():\n\tg(a[func():\n\t\tpass  # c\n\t])\n",
		"func f():\n\tg(a[func():\n\t\tpass\n\t])\n",
		"func f():\n\tvar x = a[func():\n\t\tpass\n\t]\n",
		"func f():\n\tvar x = [a[func():\n\t\tpass  # c\n\t], 1]\n",
		"var d = func():\n\tpass\n:\n\tget = _get_d\n",
		"var d = func():\n\tpass\n:\n\tget:\n\t\treturn 1\n",
	} {
		for _, commas := range []format.TrailingCommaStyle{format.TrailingCommasWhenBroken, format.NoTrailingCommas} {
			file, err := parser.Parse("closing.gd", []byte(source))
			if err != nil {
				t.Fatalf("parse %q: %v", source, err)
			}
			options := format.GodotStyle()
			options.TrailingCommas = commas
			got := format.FileWithOptions(file, options)
			reparsed, err := parser.Parse("closing.gd", []byte(got))
			if err != nil {
				t.Errorf("reparse of %q: %v\n%s", source, err, got)
				continue
			}
			if again := format.FileWithOptions(reparsed, options); again != got {
				t.Errorf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
			var before, after strings.Builder
			if err := ast.Dump(&before, file); err != nil {
				t.Fatal(err)
			}
			if err := ast.Dump(&after, reparsed); err != nil {
				t.Fatal(err)
			}
			if before.String() != after.String() {
				t.Errorf("the tree changed:\n--- source ---\n%s--- formatted ---\n%s\n%s", before.String(), after.String(), got)
			}
		}
	}
}
