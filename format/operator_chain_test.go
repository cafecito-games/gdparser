package format_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// Parentheses that precedence requires let the operands inside them continue
// across lines, so a line too long for the budget breaks before its operators
// there rather than staying whole and leaving the break to a call further along.
func TestParenthesizedOperatorChainBreaksAtItsOperators(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"a concatenation formatted by a call",
			"func f(resource_path):\n\tvar message = (\"the pack entry %s is not a resource path at all, which should never happen \" + \"— the pack is corrupt; replace it\") % _describe(resource_path)\n\treturn message\n",
			"func f(resource_path):\n\tvar message = (\n\t\t\t\"the pack entry %s is not a resource path at all, which should never happen \"\n\t\t\t+ \"— the pack is corrupt; replace it\"\n\t) % _describe(resource_path)\n\treturn message\n",
		},
		{
			"operators of one level share the chain",
			"func f(a):\n\tvar total = (a.first_quite_long_operand_name + a.second_quite_long_operand_name - a.third_quite_long_operand_name + a.fourth_one) * 2\n",
			"func f(a):\n\tvar total = (\n\t\t\ta.first_quite_long_operand_name\n\t\t\t+ a.second_quite_long_operand_name\n\t\t\t- a.third_quite_long_operand_name\n\t\t\t+ a.fourth_one\n\t) * 2\n",
		},
		{
			"a chain that fits stays on its line",
			"func f(a, b):\n\tvar x = (a + b) * 2\n",
			"func f(a, b):\n\tvar x = (a + b) * 2\n",
		},
		{
			// The chain itself fits, so the call after it takes the break, as it
			// would have with no chain before it.
			"a short chain leaves the break to a long call",
			"func f(a, b):\n\tvar x = (a + b) * some_function_with_a_long_name(a.first_argument_name, b.second_argument_name, a.third_argument_name)\n",
			"func f(a, b):\n\tvar x = (a + b) * some_function_with_a_long_name(\n\t\t\ta.first_argument_name,\n\t\t\tb.second_argument_name,\n\t\t\ta.third_argument_name\n\t)\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("chain.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := format.File(file)
			if got != test.want {
				t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, test.want)
			}
			reparsed, err := parser.Parse("chain.gd", []byte(got))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if again := format.File(reparsed); again != got {
				t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}
