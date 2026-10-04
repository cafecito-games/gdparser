package format_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A comma-separated pattern list can be broken only with a backslash
// continuation: Godot reads a bare line break inside one as the end of the
// list, and a parenthesized group is not a pattern. A list too long for the
// budget takes that continuation, two levels in so that a continued pattern is
// not read as the case body.
func TestMatchPatternListBreaksAtItsCommas(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"a list over the budget takes a pattern per line",
			"func f(value):\n\tmatch value:\n\t\tNode.NOTIFICATION_ENTER_TREE, Node.NOTIFICATION_EXIT_TREE, Node.NOTIFICATION_READY, Node.NOTIFICATION_PAUSED:\n\t\t\treturn \"lifecycle\"\n",
			"func f(value):\n\tmatch value:\n\t\tNode.NOTIFICATION_ENTER_TREE, \\\n\t\t\t\tNode.NOTIFICATION_EXIT_TREE, \\\n\t\t\t\tNode.NOTIFICATION_READY, \\\n\t\t\t\tNode.NOTIFICATION_PAUSED:\n\t\t\treturn \"lifecycle\"\n",
		},
		{
			"a list that fits stays on its line",
			"func f(value):\n\tmatch value:\n\t\t1, 2, 3:\n\t\t\treturn \"small\"\n\t\t_:\n\t\t\treturn \"other\"\n",
			"func f(value):\n\tmatch value:\n\t\t1, 2, 3:\n\t\t\treturn \"small\"\n\t\t_:\n\t\t\treturn \"other\"\n",
		},
		{
			// There is nowhere to break inside one pattern, so the arm is left
			// as written rather than continued to no effect.
			"a single pattern over the budget is left whole",
			"func f(value):\n\tmatch value:\n\t\tUzirNetcodeGameV1WorldEntityType.EntityType.ENTITY_TYPE_SOMETHING_RATHER_LONG_INDEED:\n\t\t\treturn \"one\"\n",
			"func f(value):\n\tmatch value:\n\t\tUzirNetcodeGameV1WorldEntityType.EntityType.ENTITY_TYPE_SOMETHING_RATHER_LONG_INDEED:\n\t\t\treturn \"one\"\n",
		},
		{
			// The guard belongs to the whole list, so it follows the last
			// pattern, which is the one the colon follows too.
			"a guard follows the last pattern",
			"func f(value, flag):\n\tmatch value:\n\t\tNode.NOTIFICATION_ENTER_TREE, Node.NOTIFICATION_EXIT_TREE, Node.NOTIFICATION_READY when flag:\n\t\t\treturn \"lifecycle\"\n",
			"func f(value, flag):\n\tmatch value:\n\t\tNode.NOTIFICATION_ENTER_TREE, \\\n\t\t\t\tNode.NOTIFICATION_EXIT_TREE, \\\n\t\t\t\tNode.NOTIFICATION_READY when flag:\n\t\t\treturn \"lifecycle\"\n",
		},
		{
			// A guard is measured with the patterns, so a list that would fit
			// alone still breaks when the guard pushes the arm over.
			"a guard counts towards the budget",
			"func f(value, flag):\n\tmatch value:\n\t\tNode.NOTIFICATION_ENTER_TREE, Node.NOTIFICATION_EXIT_TREE when flag and value != Node.NOTIFICATION_READY:\n\t\t\treturn \"lifecycle\"\n",
			"func f(value, flag):\n\tmatch value:\n\t\tNode.NOTIFICATION_ENTER_TREE, \\\n\t\t\t\tNode.NOTIFICATION_EXIT_TREE when flag and value != Node.NOTIFICATION_READY:\n\t\t\treturn \"lifecycle\"\n",
		},
		{
			"an annotation on the case keeps its own line",
			"func f(value):\n\tmatch value:\n\t\t_:\n\t\t\treturn \"other\"\n\t\t@warning_ignore(\"unreachable_pattern\")\n\t\tNode.NOTIFICATION_ENTER_TREE, Node.NOTIFICATION_EXIT_TREE, Node.NOTIFICATION_READY, Node.NOTIFICATION_PAUSED:\n\t\t\treturn \"lifecycle\"\n",
			"func f(value):\n\tmatch value:\n\t\t_:\n\t\t\treturn \"other\"\n\t\t@warning_ignore(\"unreachable_pattern\")\n\t\tNode.NOTIFICATION_ENTER_TREE, \\\n\t\t\t\tNode.NOTIFICATION_EXIT_TREE, \\\n\t\t\t\tNode.NOTIFICATION_READY, \\\n\t\t\t\tNode.NOTIFICATION_PAUSED:\n\t\t\treturn \"lifecycle\"\n",
		},
		{
			// The arm is wide because of a nested list, not because of the
			// patterns, so the construct with a bracket to break takes the
			// break and the patterns stay together.
			"a bracketed pattern breaks inside itself",
			"func f(value):\n\tmatch value:\n\t\t[Node.NOTIFICATION_ENTER_TREE, Node.NOTIFICATION_EXIT_TREE, Node.NOTIFICATION_READY, Node.NOTIFICATION_PAUSED]:\n\t\t\treturn \"lifecycle\"\n",
			"func f(value):\n\tmatch value:\n\t\t[\n\t\t\tNode.NOTIFICATION_ENTER_TREE,\n\t\t\tNode.NOTIFICATION_EXIT_TREE,\n\t\t\tNode.NOTIFICATION_READY,\n\t\t\tNode.NOTIFICATION_PAUSED,\n\t\t]:\n\t\t\treturn \"lifecycle\"\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("match.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := format.File(file)
			if got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("match.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := format.File(reparsed); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}

// The last pattern of a broken list is followed by the case's colon, which a
// backslash would comment the list out ahead of, so no continuation is written
// after it.
func TestBrokenMatchPatternListEndsWithoutABackslash(t *testing.T) {
	source := "func f(value):\n\tmatch value:\n\t\tNode.NOTIFICATION_ENTER_TREE, Node.NOTIFICATION_EXIT_TREE, Node.NOTIFICATION_READY, Node.NOTIFICATION_PAUSED:\n\t\t\treturn \"lifecycle\"\n"
	file, err := parser.Parse("match.gd", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := format.File(file)
	if !strings.Contains(got, "\\\n") {
		t.Fatalf("the list was not broken:\n%s", got)
	}
	for _, line := range strings.Split(got, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if !strings.HasSuffix(trimmed, "\\") {
			continue
		}
		if !strings.HasSuffix(strings.TrimSuffix(trimmed, " \\"), ",") {
			t.Fatalf("a continuation follows something other than a comma: %q", line)
		}
	}
	if _, err := parser.Parse("match.gd", []byte(got)); err != nil {
		t.Fatalf("reparse: %v", err)
	}
}
