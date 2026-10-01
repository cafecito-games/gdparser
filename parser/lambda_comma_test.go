package parser_test

import (
	"strings"
	"testing"

	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A comma at the bracket depth of a multiline lambda body ends that body, but
// not when it separates a match branch's patterns. Every source here is
// accepted by Godot 4.7.2.
func TestCommaInsideAMultilineLambdaBody(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"comma separated patterns", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, 2:\n\t\t\t\tpass\n]\n"},
		{"patterns and a guard", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, 2 when v > 0:\n\t\t\t\tpass\n]\n"},
		{"a trailing comment on the branch", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, 2:  # c\n\t\t\t\tpass\n]\n"},
		{"a string holding a hash", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, \"a#b\":\n\t\t\t\tpass\n]\n"},
		{"a string holding a colon", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, \"a:b\":\n\t\t\t\tpass\n]\n"},
		{"patterns in a nested match", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tmatch v:\n\t\t\t\t\t2, 3:\n\t\t\t\t\t\tpass\n]\n"},
		{"a branch inside an if", "var x = [func(v):\n\t\tif true:\n\t\t\tmatch v:\n\t\t\t\t1, 2:\n\t\t\t\t\tpass\n]\n"},
		{"a continuation inside the patterns", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, \\\n\t\t\t\t2:\n\t\t\t\tpass\n]\n"},
		{"a branch body on the header line", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, 2: pass\n]\n"},
		{"a match after another statement", "var x = [func(v):\n\t\tprint(v)\n\t\tmatch v:\n\t\t\t1, 2:\n\t\t\t\tpass\n]\n"},
		{"two branches with pattern lists", "var x = [func(v):\n\t\tmatch v:\n\t\t\t1, 2:\n\t\t\t\tpass\n\t\t\t3, 4:\n\t\t\t\tpass\n]\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("comma.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			again, err := parser.Parse("comma.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
			if !strings.Contains(formatted, "match") {
				t.Errorf("the match statement did not survive:\n%s", formatted)
			}
		})
	}
}

// A comma that ends a line's statement still ends the body it sits in.
func TestCommaStillEndsALambdaBody(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "after the body's last statement",
			source: "var x = [func():\n\t\tpass, 2]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass,\n\t2,\n]\n",
		},
		{
			name:   "after a statement in a nested block",
			source: "var x = [func():\n\t\tif true:\n\t\t\tpass, 2]\n",
			want:   "var x = [\n\tfunc():\n\t\tif true:\n\t\t\tpass,\n\t2,\n]\n",
		},
		{
			// Godot rejects a one-tab indent here but accepts the body's own,
			// so the comma is written at the indentation the engine allows.
			name:   "on a line of its own",
			source: "var x = [func():\n\t\tpass\n\t\t, 2]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass,\n\t2,\n]\n",
		},
		{
			name:   "before a second multiline lambda",
			source: "var x = [func():\n\t\tpass, func():\n\t\t\tpass]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass,\n\tfunc():\n\t\tpass,\n]\n",
		},
		{
			// Godot accepts match as a name, so the keyword also appears where
			// it opens no block and holds no pattern list.
			name:   "after a call to a method named match",
			source: "var x = [func(v):\n\t\tv.match(\"a\"), 2]\n",
			want:   "var x = [\n\tfunc(v):\n\t\tv.match(\"a\"),\n\t2,\n]\n",
		},
		{
			name:   "after a variable named match",
			source: "var x = [func(_v):\n\t\tvar match = 1, 2]\n",
			want:   "var x = [\n\tfunc(_v):\n\t\tvar match = 1,\n\t2,\n]\n",
		},
		{
			name:   "between two lambda dictionary values",
			source: "var d = {1: func():\n\t\tpass, 2: func():\n\t\tpass}\n",
			want:   "var d = {\n\t1: func():\n\t\tpass,\n\t2: func():\n\t\tpass,\n}\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("comma.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if formatted := gdformat.File(file); formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
		})
	}
}

// Godot rejects a match branch whose pattern list is split across lines, even
// inside brackets, so the comma at the end of the line must not be read as a
// pattern separator that holds the body open.
func TestASplitPatternListIsRejected(t *testing.T) {
	source := "var x = [func(v):\n\t\tmatch v:\n\t\t\t1,\n\t\t\t2:\n\t\t\t\tpass\n]\n"
	if _, err := parser.Parse("comma.gd", []byte(source)); err == nil {
		t.Fatal("parse succeeded, want a positioned error")
	}
}
