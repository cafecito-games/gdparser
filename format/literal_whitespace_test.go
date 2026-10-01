package format_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// Whitespace that ends a line inside a string literal is part of the string's
// value, so only the whitespace the formatter itself would leave at the end of
// a line is trimmed.
func TestWhitespaceInsideAMultilineStringIsKept(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"spaces ending a line of a triple-quoted string",
			"var a = \"\"\"one  \ntwo\"\"\"\n",
			"var a = \"\"\"one  \ntwo\"\"\"\n",
		},
		{
			"a tab ending a line and a line of spaces alone",
			"func f():\n\tvar a = '''one\t\n   \n\ttwo'''\n\treturn a\n",
			"func f():\n\tvar a = '''one\t\n   \n\ttwo'''\n\treturn a\n",
		},
		{
			"inside a call that breaks around it",
			"func f():\n\tprint(\"\"\"one \ntwo\"\"\", 1)\n",
			"func f():\n\tprint(\n\t\t\t\"\"\"one \ntwo\"\"\",\n\t\t\t1\n\t)\n",
		},
		{
			// Outside a literal the trimming still happens.
			"a comment ending in spaces and an indented blank line",
			"func f():\n\tvar a = 1  # c  \n\t\n\treturn a\n",
			"func f():\n\tvar a = 1  # c\n\n\treturn a\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("literal.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := format.File(file)
			if got != test.want {
				t.Fatalf("formatted %q, want %q", got, test.want)
			}
			reparsed, err := parser.Parse("literal.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := format.File(reparsed); again != got {
				t.Fatalf("not idempotent: %q then %q", got, again)
			}
		})
	}
}
