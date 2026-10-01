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

// Every block written on one line ends the same way, so a match branch and a
// block nested in another one-line block keep what follows them too. Godot reads
// a branch body with parse_suite as well, and the annotation it holds decorates
// the first statement read after the branch, wherever that is.
func TestOneLineAnnotationBlockInAMatchAndNested(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "the branch after a match branch holding only an annotation",
			source: "func f(x):\n\tmatch x:\n\t\t1: @warning_ignore(\"unused_variable\")\n\t\t2: var y = 1\n",
			want:   "func f(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t2:\n\t\t\t@warning_ignore(\"unused_variable\")\n\t\t\tvar y = 1\n",
		},
		{
			name:   "the statement after a match whose last branch holds only an annotation",
			source: "func f(x):\n\tmatch x:\n\t\t1: @warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
			want:   "func f(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t@warning_ignore(\"unused_variable\")\n\tvar y = 1\n",
		},
		{
			// The "else" stands at the inner if's level, so it continues the
			// inner one rather than the one holding it.
			name:   "an else after a one-line if nested in another",
			source: "func f(x):\n\tif x: if x: @warning_ignore(\"unused_variable\")\n\telse: var y = 1\n",
			want:   "func f(x):\n\tif x:\n\t\tif x:\n\t\t\tpass\n\t\telse:\n\t\t\t@warning_ignore(\"unused_variable\")\n\t\t\tvar y = 1\n",
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

// A block written on one line reads one annotation and no more. Godot's
// parse_suite runs its body once unless the statement it read ended in a
// semicolon: the annotation branch of parse_statement returns no statement and
// continues, which lands on that condition, so the second annotation of a pair
// is left outside the block. The scope around the block takes it, and a keyword
// that would have continued the statement is then read where no statement can
// begin, which Godot rejects as well.
func TestOneLineAnnotationBlockReadsOneAnnotation(t *testing.T) {
	source := "func f(x):\n\tif x: @warning_ignore(\"unused_variable\") @warning_ignore(\"shadowed_variable\")\n\tvar y = 1\n"
	want := "func f(x):\n\tif x:\n\t\tpass\n\t@warning_ignore(\"unused_variable\") @warning_ignore(\"shadowed_variable\")\n\tvar y = 1\n"
	file, err := parser.Parse("block.gd", []byte(source))
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	if got := gdformat.File(file); got != want {
		t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	withElse := "func f(x):\n\tif x: @warning_ignore(\"unused_variable\") @warning_ignore(\"shadowed_variable\")\n\telse: pass\n"
	if _, err := parser.Parse("block.gd", []byte(withElse)); err == nil {
		t.Fatal("parsed an \"else\" after a one-line block holding two annotations")
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
