package parser_test

import (
	"strings"
	"testing"

	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A one-line lambda body takes the line break that ends it: Godot's
// end_statement consumes the break while it is still inside the lambda. A
// property initialized with such a lambda therefore finds the colon of its
// accessor block on the next line, as though it stood on the lambda's own.
func TestAccessorColonOnTheLineAfterAOneLineLambda(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"accessor bodies",
			"var d = func(): return 1\n:\n\tget:\n\t\treturn d\n",
			"var d = func(): return 1:\n\tget:\n\t\treturn d\n",
		},
		{
			"a named accessor",
			"var d = func(): pass\n:\n\tget = _g\n",
			"var d = func(): pass:\n\tget = _g\n",
		},
		{
			"blank lines before the colon",
			"var d = func(): pass\n\n\n:\n\tget = _g\n",
			"var d = func(): pass:\n\tget = _g\n",
		},
		{
			"a comment ending the lambda's line",
			"var d = func(): pass  # c\n:\n\tget = _g\n",
			"var d = (func(): pass  # c\n\t):\n\tget = _g\n",
		},
		{
			"a comment line before the colon",
			"var d = func(): pass\n# c\n:\n\tget = _g\n",
			"var d = func(): pass:\n\t# c\n\tget = _g\n",
		},
		{
			// Only a block has room for the comment, so the accessors move
			// into one.
			"a comment line before accessors on the colon's line",
			"var d = func(): pass\n# c\n: get = _g\n",
			"var d = func(): pass:\n\t# c\n\tget = _g\n",
		},
		{
			"a semicolon before the colon",
			"var d = func(): pass\n;\n:\n\tget = _g\n",
			"var d = func(): pass:\n\tget = _g\n",
		},
		{
			"a lambda ending an operator's right operand",
			"var d = 1 + func(): pass\n:\n\tget = _g\n",
			"var d = 1 + (func(): pass):\n\tget = _g\n",
		},
		{
			"in an inner class",
			"class Inner:\n\tvar d = func(): pass\n\t:\n\t\tget = _g\n",
			"class Inner:\n\tvar d = func(): pass:\n\t\tget = _g\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("property.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := gdformat.File(file)
			if got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("property.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := gdformat.File(reparsed); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}

// Only the lambda's own body takes the line break. A value that merely holds a
// lambda, or is none, ends its declaration at the break, and the colon below it
// belongs to nothing.
func TestAccessorColonOnTheNextLineOtherwiseBelongsToNothing(t *testing.T) {
	for _, source := range []string{
		"var d = 1\n:\n\tget = _g\n",
		"var d = f(func(): pass)\n:\n\tget = _g\n",
		"var d = [func(): pass]\n:\n\tget = _g\n",
		// A lambda the value moved on from took no line break of the value's.
		"var d = 1 if func(): pass else 2\n:\n\tget = _g\n",
		// A constant reads no accessors.
		"const d = func(): pass\n:\n\tget = _g\n",
		// The colon has to stand where the declaration does.
		"var d = func(): pass\n\t:\n\t\tget = _g\n",
		// A local is no property.
		"func f():\n\tvar d = func(): pass\n\t:\n\t\tget = _g\n",
	} {
		_, err := parser.Parse("property.gd", []byte(source))
		if err == nil {
			t.Errorf("parse %q: expected an error", source)
			continue
		}
		if !strings.Contains(err.Error(), "property.gd:") {
			t.Errorf("parse %q: error carries no position: %v", source, err)
		}
	}
}
