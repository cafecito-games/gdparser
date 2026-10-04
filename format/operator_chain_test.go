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
			assertFormats(t, "chain.gd", test.source, test.want)
		})
	}
}

// A chain that no precedence rule has parenthesized still has to fit the line,
// so it adds the parentheses that let it continue and breaks before each
// operator, which is the shape an and/or chain already takes.
func TestUnparenthesizedOperatorChainGainsParenthesesAndBreaks(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"an arithmetic chain",
			"func arithmetic(first_value, second_value, third_value, fourth_value, fifth_value) -> int:\n\treturn first_value + second_value + third_value + fourth_value + fifth_value + first_value + second_value + 1\n",
			"func arithmetic(first_value, second_value, third_value, fourth_value, fifth_value) -> int:\n\treturn (\n\t\t\tfirst_value\n\t\t\t+ second_value\n\t\t\t+ third_value\n\t\t\t+ fourth_value\n\t\t\t+ fifth_value\n\t\t\t+ first_value\n\t\t\t+ second_value\n\t\t\t+ 1\n\t)\n",
		},
		{
			"a bitwise chain",
			"func bitwise(first_value, second_value, third_value, fourth_value) -> int:\n\treturn first_value | second_value | third_value | fourth_value | first_value | second_value | third_value | 1\n",
			"func bitwise(first_value, second_value, third_value, fourth_value) -> int:\n\treturn (\n\t\t\tfirst_value\n\t\t\t| second_value\n\t\t\t| third_value\n\t\t\t| fourth_value\n\t\t\t| first_value\n\t\t\t| second_value\n\t\t\t| third_value\n\t\t\t| 1\n\t)\n",
		},
		{
			// The comparison is the loosest operator in the expression, so it is the
			// one the chain breaks at, and each side of it stays whole.
			"a comparison breaks at itself, not inside its operands",
			"func comparison(first_value, second_value) -> bool:\n\treturn first_value + second_value + first_value + second_value > second_value + first_value + second_value + 10\n",
			"func comparison(first_value, second_value) -> bool:\n\treturn (\n\t\t\tfirst_value + second_value + first_value + second_value\n\t\t\t> second_value + first_value + second_value + 10\n\t)\n",
		},
		{
			"an assigned chain",
			"func assigned(first_value, second_value, third_value, fourth_value, fifth_value) -> void:\n\tvar total = first_value + second_value + third_value + fourth_value + fifth_value + first_value + 1\n",
			"func assigned(first_value, second_value, third_value, fourth_value, fifth_value) -> void:\n\tvar total = (\n\t\t\tfirst_value\n\t\t\t+ second_value\n\t\t\t+ third_value\n\t\t\t+ fourth_value\n\t\t\t+ fifth_value\n\t\t\t+ first_value\n\t\t\t+ 1\n\t)\n",
		},
		{
			"a chain that fits gains nothing",
			"func fits(a, b, c) -> int:\n\treturn a + b + c\n",
			"func fits(a, b, c) -> int:\n\treturn a + b + c\n",
		},
		{
			// The call can continue across lines on the brackets it already has, so
			// the chain stays whole rather than adding parentheses of its own.
			"a chain leaves the break to a call it holds",
			"func calling(a, b) -> int:\n\treturn a * some_function_with_a_long_name(a.first_argument_name, b.second_argument_name, a.third_argument)\n",
			"func calling(a, b) -> int:\n\treturn a * some_function_with_a_long_name(\n\t\t\ta.first_argument_name,\n\t\t\tb.second_argument_name,\n\t\t\ta.third_argument\n\t)\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertFormats(t, "chain.gd", test.source, test.want)
		})
	}
}

// A conditional expression is the style guide's own example of a construct that
// multiple lines make more readable, and it wraps before each "else" so that a
// value stays with the condition that chooses it.
func TestTernaryChainBreaksBeforeEachElse(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{
			"a nested conditional",
			"func ternary(reason: int) -> String:\n\treturn \"alpha_value_one\" if reason == 1 else \"beta_value_two\" if reason == 2 else \"gamma_value_three\" if reason == 3 else \"delta\"\n",
			"func ternary(reason: int) -> String:\n\treturn (\n\t\t\t\"alpha_value_one\" if reason == 1\n\t\t\telse \"beta_value_two\" if reason == 2\n\t\t\telse \"gamma_value_three\" if reason == 3\n\t\t\telse \"delta\"\n\t)\n",
		},
		{
			"a conditional that fits gains nothing",
			"func fits(a) -> int:\n\treturn 1 if a else 2\n",
			"func fits(a) -> int:\n\treturn 1 if a else 2\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertFormats(t, "ternary.gd", test.source, test.want)
		})
	}
}

// assertFormats formats source and checks it against want, that the output
// parses, and that formatting it again leaves it alone.
func assertFormats(t *testing.T, name, source, want string) {
	t.Helper()
	file, err := parser.Parse(name, []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := format.File(file)
	if got != want {
		t.Fatalf("formatted:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	reparsed, err := parser.Parse(name, []byte(got))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if again := format.File(reparsed); again != got {
		t.Fatalf("not idempotent:\n--- first ---\n%s--- second ---\n%s", got, again)
	}
}
