package parser_test

import (
	"strings"
	"testing"

	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A comma at the bracket depth of a multiline lambda body ends that body, but
// not when it separates the parts of a line that opens a block: a match
// branch's patterns are written that way.
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
			name:   "on a line of its own",
			source: "var x = [func():\n\t\tpass\n\t, 2]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass,\n\t2,\n]\n",
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
