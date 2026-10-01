package format_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A property may name its accessor methods in an indented block as well as on
// its own line. The block is kept, with the comma Godot requires between the
// two accessors, and so are the comments written in it.
func TestShorthandAccessorsWrittenInABlockAreKept(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"a getter alone",
			"var d:\n\tget = _get_d\n",
			"var d:\n\tget = _get_d\n",
		},
		{
			"a setter and a getter",
			"var d:\n\tset = _set_d,\n\tget = _get_d\n",
			"var d:\n\tset = _set_d,\n\tget = _get_d\n",
		},
		{
			"both on one line of the block",
			"var d: int = 1:\n\tget = _get_d, set = _set_d\n",
			"var d: int = 1:\n\tget = _get_d,\n\tset = _set_d\n",
		},
		{
			"with comments around them",
			"var d:  # head\n\t# first\n\tset = _set_d,  # tail\n\t# second\n\tget = _get_d\n\t# last\n",
			"var d:  # head\n\t# first\n\tset = _set_d,  # tail\n\t# second\n\tget = _get_d\n\t# last\n",
		},
		{
			"written on the declaration's own line",
			"var d: get = _get_d, set = _set_d\n",
			"var d: get = _get_d, set = _set_d\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("accessor.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := format.File(file)
			if got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("accessor.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := format.File(reparsed); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}
