package parser_test

import (
	"strings"
	"testing"

	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A block written on one line whose body is only an annotation ends with the
// annotation consumed, so the keyword that continues the statement around it is
// what comes next. Godot's parse_suite reads the annotation through
// parse_statement, which holds it on the annotation stack and returns no
// statement, and parse_if then still finds its "elif" or "else".
func TestOneLineAnnotationBlockKeepsItsContinuation(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "else on the next line",
			source: "func f(x):\n\tif x: @warning_ignore(\"unused_variable\")\n\telse: var y = 1\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\telse:\n\t\t@warning_ignore(\"unused_variable\")\n\t\tvar y = 1\n",
		},
		{
			name:   "elif on the next line",
			source: "func f(x):\n\tif x: @warning_ignore(\"unused_variable\")\n\telif x: var y = 1\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\telif x:\n\t\t@warning_ignore(\"unused_variable\")\n\t\tvar y = 1\n",
		},
		{
			name:   "a while body, which has no continuation",
			source: "func f(x):\n\twhile x: @warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
			want:   "func f(x):\n\twhile x:\n\t\tpass\n\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
		},
		{
			name:   "a for body, which has no continuation",
			source: "func f(x):\n\tfor i in x: @warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
			want:   "func f(x):\n\tfor i in x:\n\t\tpass\n\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
		},
		{
			name:   "a standalone annotation, which waits for nothing",
			source: "func f(x):\n\tif x: @warning_ignore_start(\"unused_variable\")\n\telse: pass\n",
			want:   "func f(x):\n\tif x:\n\t\t@warning_ignore_start(\"unused_variable\")\n\telse:\n\t\tpass\n",
		},
		{
			name:   "a trailing comment after the annotation",
			source: "func f(x):\n\tif x: @warning_ignore(\"unused_variable\")  # note\n\telse: var y = 1\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\telse:\n\t\t@warning_ignore(\"unused_variable\")  # note\n\t\tvar y = 1\n",
		},
		{
			// Godot's parse_class_body hands a standalone group marker to the
			// class it is reading, through add_member_group, so a one-line class
			// body keeps the one it holds.
			name:   "a group marker as a one-line class member",
			source: "class A: @export_group(\"Move\")\nvar v = 1\n",
			want:   "class A:\n\t@export_group(\"Move\")\n\n\nvar v = 1\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("block.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse %q: %v", test.source, err)
			}
			got := gdformat.File(file)
			if got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("block.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := gdformat.File(reparsed); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}

// A standalone annotation must still end its line, as it does anywhere else.
func TestOneLineAnnotationBlockRejectsAStandaloneAnnotationSharingItsLine(t *testing.T) {
	source := "func f(x):\n\tif x: @warning_ignore_start(\"unused_variable\"); pass\n"
	_, err := parser.Parse("block.gd", []byte(source))
	if err == nil {
		t.Fatal("parsed a standalone annotation followed by a statement on its line")
	}
	if !strings.Contains(err.Error(), "end of line") {
		t.Fatalf("parse reported %v, want the annotation to need the rest of its line", err)
	}
}
