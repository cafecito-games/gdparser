package format_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// styleCases covers the layout rules of the Godot GDScript style guide. Each
// case states the exact output the formatter must produce.
var styleCases = []struct {
	name   string
	source string
	want   string
}{
	{
		name:   "tabs indent each nesting level",
		source: "func f() -> void:\n\tif true:\n\t\tpass\n",
		want:   "func f() -> void:\n\tif true:\n\t\tpass\n",
	},
	{
		name:   "single line suite is expanded",
		source: "if x: return\n",
		want:   "if x:\n\treturn\n",
	},
	{
		name:   "two blank lines surround a function",
		source: "var a := 1\nfunc f() -> void:\n\tpass\nvar b := 2\n",
		want:   "var a := 1\n\n\nfunc f() -> void:\n\tpass\n\n\nvar b := 2\n",
	},
	{
		name:   "authored blank lines are clamped",
		source: "func f() -> void:\n\tvar a := 1\n\n\n\n\tvar b := 2\n",
		want:   "func f() -> void:\n\tvar a := 1\n\n\tvar b := 2\n",
	},
	{
		name:   "a blank line inside a function is kept",
		source: "func f() -> void:\n\tvar a := 1\n\n\tvar b := 2\n",
		want:   "func f() -> void:\n\tvar a := 1\n\n\tvar b := 2\n",
	},
	{
		name:   "no blank line opens or closes a suite",
		source: "func f() -> void:\n\n\tvar a := 1\n\n",
		want:   "func f() -> void:\n\tvar a := 1\n",
	},
	{
		name:   "a comment run documents the declaration below it",
		source: "extends Node\n## Does the thing.\n# detail\nfunc f() -> void:\n\tpass\n",
		want:   "extends Node\n\n\n## Does the thing.\n# detail\nfunc f() -> void:\n\tpass\n",
	},
	{
		name:   "a trailing comment stays on its line",
		source: "var a := 1  # why\n",
		want:   "var a := 1  # why\n",
	},
	{
		name:   "a trailing comment is spaced out from short code",
		source: "var a := 1 # why\n",
		want:   "var a := 1  # why\n",
	},
	{
		name:   "an inline annotation stays on the declaration",
		source: "@export var speed := 1.0\n",
		want:   "@export var speed := 1.0\n",
	},
	{
		name:   "an annotation written alone keeps its own line",
		source: "@export_group(\"Movement\")\n@export var jump := 2.0\n",
		want:   "@export_group(\"Movement\")\n@export var jump := 2.0\n",
	},
	{
		name:   "an own-line annotation is not pulled onto the declaration",
		source: "@onready\nvar node := $Sprite2D\n",
		want:   "@onready\nvar node := $Sprite2D\n",
	},
	{
		name:   "a comment gains one space after its marker",
		source: "#no space\n##doc\n# fine\n#\n",
		want:   "# no space\n## doc\n# fine\n#\n",
	},
	{
		name:   "region markers keep no space",
		source: "#region Movement\n#endregion\n",
		want:   "#region Movement\n#endregion\n",
	},
	{
		name:   "boolean operators use their plain English spelling",
		source: "var a := b && c\nvar d := e || f\nvar g := not h\n",
		want:   "var a := b and c\nvar d := e or f\nvar g := not h\n",
	},
	{
		name: "rewriting a bang keeps its meaning",
		// not binds looser than !, so the parenthesis has to appear.
		source: "var a := !b == c\n",
		want:   "var a := (not b) == c\n",
	},
	{
		name:   "unnecessary parentheses are dropped",
		source: "var a := (b and c) or d\nvar e := (f + g) + h\n",
		want:   "var a := b and c or d\nvar e := f + g + h\n",
	},
	{
		name:   "necessary parentheses are kept",
		source: "var a := (b + c) * d\nvar e := f and (g or h)\n",
		want:   "var a := (b + c) * d\nvar e := f and (g or h)\n",
	},
	{
		name:   "a float keeps its leading and trailing zero",
		source: "var a := .5\nvar b := 5.\nvar c := 1.5e-3\n",
		want:   "var a := 0.5\nvar b := 5.0\nvar c := 1.5e-3\n",
	},
	{
		name:   "radix prefixes and hexadecimal digits are lowercased",
		source: "var a := 0XFF\nvar b := 0B1010\nvar c := 0xAb\n",
		want:   "var a := 0xff\nvar b := 0b1010\nvar c := 0xab\n",
	},
	{
		name:   "digit separators are left alone",
		source: "var a := 1_000_000\nvar b := 1000000\n",
		want:   "var a := 1_000_000\nvar b := 1000000\n",
	},
	{
		name:   "strings prefer double quotes",
		source: "var a := 'single'\nvar b := &'name'\nvar c := ^'path'\n",
		want:   "var a := \"single\"\nvar b := &\"name\"\nvar c := ^\"path\"\n",
	},
	{
		name:   "single quotes win when they escape fewer characters",
		source: "var a := \"say \\\"hi\\\"\"\n",
		want:   "var a := 'say \"hi\"'\n",
	},
	{
		name:   "an escaped quote is dropped when the delimiter changes",
		source: "var a := 'it\\'s'\n",
		want:   "var a := \"it's\"\n",
	},
	{
		name:   "a string needing both quotes keeps the preferred one",
		source: "var a := \"both \\\" and '\"\n",
		want:   "var a := \"both \\\" and '\"\n",
	},
	{
		name:   "triple quoted strings are left alone",
		source: "var a := '''triple'''\n",
		want:   "var a := '''triple'''\n",
	},
	{
		name:   "a raw string is requoted but not unescaped",
		source: "var a := r'raw\\d'\n",
		want:   "var a := r\"raw\\d\"\n",
	},
	{
		name:   "a single line dictionary is padded inside its braces",
		source: "var a := {\"k\": 1}\n",
		want:   "var a := { \"k\": 1 }\n",
	},
	{
		name:   "an empty dictionary is not padded",
		source: "var a := {}\nvar b := []\n",
		want:   "var a := {}\nvar b := []\n",
	},
	{
		name:   "a dictionary reference is not padded",
		source: "var a := d[\"k\"]\n",
		want:   "var a := d[\"k\"]\n",
	},
	{
		name:   "a single line list drops its trailing comma",
		source: "var a := [1, 2, 3,]\n",
		want:   "var a := [1, 2, 3]\n",
	},
	{
		name:   "a broken array indents one level and ends with a comma",
		source: "var values := [11111111, 22222222, 33333333, 44444444, 55555555, 66666666, 77777777, 88888888, 999999]\n",
		want: "var values := [\n\t11111111,\n\t22222222,\n\t33333333,\n\t44444444,\n\t55555555,\n" +
			"\t66666666,\n\t77777777,\n\t88888888,\n\t999999,\n]\n",
	},
	{
		name:   "a broken argument list indents two levels and takes no comma",
		source: "func f() -> void:\n\tsome_object.some_method(argument_one, argument_two, argument_three, argument_four, argument_five, argument_six)\n",
		want: "func f() -> void:\n\tsome_object.some_method(\n\t\t\targument_one,\n\t\t\targument_two,\n" +
			"\t\t\targument_three,\n\t\t\targument_four,\n\t\t\targument_five,\n\t\t\targument_six\n\t)\n",
	},
	{
		name:   "a broken parameter list indents two levels",
		source: "func some_function_name(first_parameter: int, second_parameter: String, third_parameter: float) -> void:\n\tpass\n",
		want: "func some_function_name(\n\t\tfirst_parameter: int,\n\t\tsecond_parameter: String,\n" +
			"\t\tthird_parameter: float\n) -> void:\n\tpass\n",
	},
	{
		name:   "a broken condition gains parentheses and leads with its keyword",
		source: "func f() -> void:\n\tif first_condition_is_long and second_condition_is_also_long and third_condition_is_here_as_well:\n\t\tpass\n",
		want: "func f() -> void:\n\tif (\n\t\t\tfirst_condition_is_long\n\t\t\tand second_condition_is_also_long\n" +
			"\t\t\tand third_condition_is_here_as_well\n\t):\n\t\tpass\n",
	},
	{
		name:   "a nested logical group stays on one line",
		source: "func f() -> void:\n\tif first_condition_is_long and (second_or_this_one or third_condition_here) and fourth_condition:\n\t\tpass\n",
		want: "func f() -> void:\n\tif (\n\t\t\tfirst_condition_is_long\n\t\t\tand (second_or_this_one or third_condition_here)\n" +
			"\t\t\tand fourth_condition\n\t):\n\t\tpass\n",
	},
	{
		name:   "a broken while condition takes the same shape",
		source: "func f() -> void:\n\twhile alpha_condition_value or beta_condition_value or gamma_condition_value or delta_value_here:\n\t\tpass\n",
		want: "func f() -> void:\n\twhile (\n\t\t\talpha_condition_value\n\t\t\tor beta_condition_value\n" +
			"\t\t\tor gamma_condition_value\n\t\t\tor delta_value_here\n\t):\n\t\tpass\n",
	},
	{
		name:   "a nested collection breaks only as far as it must",
		source: "var config := {\"movement\": {\"speed\": 100.0, \"jump\": 200.0}, \"combat\": {\"damage\": 10, \"range\": 5}}\n",
		want: "var config := {\n\t\"movement\": { \"speed\": 100.0, \"jump\": 200.0 },\n" +
			"\t\"combat\": { \"damage\": 10, \"range\": 5 },\n}\n",
	},
	{
		name:   "an enum with member comments breaks and keeps them",
		source: "enum State {\n\t# idle\n\tIDLE,\n\tRUNNING = 3,\n}\n",
		want:   "enum State {\n\t# idle\n\tIDLE,\n\tRUNNING = 3,\n}\n",
	},
	{
		name:   "a short enum stays on one line",
		source: "enum State { IDLE, RUNNING = 3 }\n",
		want:   "enum State { IDLE, RUNNING = 3 }\n",
	},
	{
		name:   "property accessors are indented beneath the declaration",
		source: "var health: int = 10:\n\tget:\n\t\treturn health\n\tset(value):\n\t\thealth = value\n",
		want:   "var health: int = 10:\n\tget:\n\t\treturn health\n\tset(value):\n\t\thealth = value\n",
	},
	{
		name:   "an inline lambda stays on one line",
		source: "var f := func(x): return x + 1\n",
		want:   "var f := func(x): return x + 1\n",
	},
	{
		name:   "a block lambda keeps its suite",
		source: "var g := func():\n\tprint(1)\n\tprint(2)\n",
		want:   "var g := func():\n\tprint(1)\n\tprint(2)\n",
	},
	{
		name:   "a match statement keeps its cases",
		source: "func f(v):\n\tmatch v:\n\t\t0:\n\t\t\treturn \"zero\"\n\t\t1, 2:\n\t\t\treturn \"small\"\n\t\t_:\n\t\t\treturn \"other\"\n",
		want:   "func f(v):\n\tmatch v:\n\t\t0:\n\t\t\treturn \"zero\"\n\t\t1, 2:\n\t\t\treturn \"small\"\n\t\t_:\n\t\t\treturn \"other\"\n",
	},
	{
		name:   "a byte order mark is dropped",
		source: "\xef\xbb\xbfextends Node\n",
		want:   "extends Node\n",
	},
	{
		name:   "an empty suite becomes pass",
		source: "class Inner:\n\tpass\n",
		want:   "class Inner:\n\tpass\n",
	},
	{
		name:   "a ternary expression stays on one line",
		source: "var a := b if c else d\n",
		want:   "var a := b if c else d\n",
	},
	{
		name:   "power associates to the right",
		source: "var a := 2 ** 3 ** 4\nvar b := (2 ** 3) ** 4\n",
		want:   "var a := 2 ** 3 ** 4\nvar b := (2 ** 3) ** 4\n",
	},
}

func TestStyleRules(t *testing.T) {
	for _, testCase := range styleCases {
		t.Run(testCase.name, func(t *testing.T) {
			file, err := parser.Parse("test.gd", []byte(testCase.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := format.File(file); got != testCase.want {
				t.Fatalf("formatted output\n--- got ---\n%s--- want ---\n%s", got, testCase.want)
			}
		})
	}
}

// TestOutputIsParseableAndIdempotent guards the two invariants that every rule
// has to respect: output must parse, and formatting it again must not change it.
func TestOutputIsParseableAndIdempotent(t *testing.T) {
	for _, testCase := range styleCases {
		t.Run(testCase.name, func(t *testing.T) {
			file, err := parser.Parse("test.gd", []byte(testCase.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			first := format.File(file)
			reparsed, err := parser.Parse("test.gd", []byte(first))
			if err != nil {
				t.Fatalf("formatted output did not parse: %v\n%s", err, first)
			}
			if second := format.File(reparsed); second != first {
				t.Fatalf("formatting is not idempotent\n--- first ---\n%s--- second ---\n%s", first, second)
			}
		})
	}
}

// TestLinesStayWithinBudget checks that no case produces an overlong line unless
// a single indivisible token is itself too wide.
func TestLinesStayWithinBudget(t *testing.T) {
	options := format.GodotStyle()
	for _, testCase := range styleCases {
		t.Run(testCase.name, func(t *testing.T) {
			file, err := parser.Parse("test.gd", []byte(testCase.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			for index, line := range strings.Split(strings.TrimRight(format.File(file), "\n"), "\n") {
				if width := columnWidth(line, options.TabWidth); width > options.LineWidth {
					t.Fatalf("line %d is %d columns wide: %s", index+1, width, line)
				}
			}
		})
	}
}

func TestNilFileFormatsEmpty(t *testing.T) {
	if got := format.File(nil); got != "" {
		t.Fatalf("format.File(nil) = %q", got)
	}
}

func TestNoTrailingWhitespace(t *testing.T) {
	source := "func f() -> void:\n\tvar a := 1\n\n\tvar b := 2\n"
	file, err := parser.Parse("test.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for index, line := range strings.Split(format.File(file), "\n") {
		if strings.TrimRight(line, " \t") != line {
			t.Fatalf("line %d has trailing whitespace: %q", index+1, line)
		}
	}
}

func columnWidth(line string, tabWidth int) int {
	width := 0
	for _, character := range line {
		if character == '\t' {
			width += tabWidth
			continue
		}
		width++
	}
	return width
}

func TestCommaAfterItemEndingInAComment(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "required separator moves below the comment",
			source: "var x = [\n\tfunc():\n\t\tpass\n\t\t# c\n, 2]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass\n\t\t# c\n\t\t,\n\t2,\n]\n",
		},
		{
			name:   "optional trailing comma is dropped",
			source: "var x = [\n\tfunc():\n\t\tpass\n\t\t# c\n]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass\n\t\t# c\n\t\t,\n]\n",
		},
		{
			name:   "same-line trailing comment keeps the separator",
			source: "var x = [\n\tfunc():\n\t\tpass  # c\n, 2]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass  # c\n\t\t,\n\t2,\n]\n",
		},
		{
			name:   "same-line trailing comment drops the optional comma",
			source: "var x = [\n\tfunc():\n\t\tvar y = 1  # c\n]\n",
			want:   "var x = [\n\tfunc():\n\t\tvar y = 1  # c\n\t\t,\n]\n",
		},
		{
			name:   "dictionary value ending in a comment",
			source: "var d = {\n\t\"k\": func():\n\t\tpass\n\t\t# c\n}\n",
			want:   "var d = {\n\t\"k\": func():\n\t\tpass\n\t\t# c\n\t\t,\n}\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("commas.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := format.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("commas.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := format.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

// TestCollectionCommentPlacement states the exact output for a comment written
// between the items of a bracketed construct.
func TestCollectionCommentPlacement(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a comment between elements keeps its own line",
			source: "var x = [\n\t1,\n\t\t# c\n\t2,\n]\n",
			want:   "var x = [\n\t1,\n\t# c\n\t2,\n]\n",
		},
		{
			name:   "a comment after the last element follows its comma",
			source: "var x = [\n\t1,\n\t# c\n]\n",
			want:   "var x = [\n\t1,\n\t# c\n]\n",
		},
		{
			name:   "an end of line comment stays on its element's line",
			source: "var x = [\n\t1,  # c\n\t2,\n]\n",
			want:   "var x = [\n\t1,  # c\n\t2,\n]\n",
		},
		{
			name:   "a comment on the opening line stays there",
			source: "var x = [  # c\n\t1,\n]\n",
			want:   "var x = [  # c\n\t1,\n]\n",
		},
		{
			name:   "a comment keeps an otherwise empty collection broken",
			source: "var x = [\n\t# c\n]\n",
			want:   "var x = [\n\t# c\n]\n",
		},
		{
			name:   "a comment breaks a collection that would otherwise fit",
			source: "var x = [1,\n\t# c\n\t2]\n",
			want:   "var x = [\n\t1,\n\t# c\n\t2,\n]\n",
		},
		{
			name:   "a dictionary comment keeps its own line",
			source: "var d = {\"a\": 1,\n\t# c\n\t\"b\": 2}\n",
			want:   "var d = {\n\t\"a\": 1,\n\t# c\n\t\"b\": 2,\n}\n",
		},
		{
			name:   "an argument list indents a comment with its arguments",
			source: "f(1,\n\t# c\n\t2)\n",
			want:   "f(\n\t\t1,\n\t\t# c\n\t\t2\n)\n",
		},
		{
			name:   "an enum comment keeps its own line",
			source: "enum E {\n\t# c\n\tA,\n}\n",
			want:   "enum E {\n\t# c\n\tA,\n}\n",
		},
		{
			name:   "a comment after the last enum member survives",
			source: "enum E {\n\tA,\n\t# c\n}\n",
			want:   "enum E {\n\tA,\n\t# c\n}\n",
		},
		{
			name:   "an opening line comment survives beside an own-line one",
			source: "var x = [  # c\n\t# d\n]\n",
			want:   "var x = [  # c\n\t# d\n]\n",
		},
		{
			name:   "an argument list holding only comments is still written",
			source: "@e(\n\t# c\n)\nvar x := 1\n",
			want:   "@e(\n\t\t# c\n)\nvar x := 1\n",
		},
		{
			name:   "a signal parameter list holding only comments is still written",
			source: "signal s(\n\t# c\n)\n",
			want:   "signal s(\n\t\t# c\n)\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("comments.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := format.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
		})
	}
}

// TestCommentInsideAnInlineLambdaBody covers the comment an inline lambda body
// may hold. A comment ends its line, so it stays at the end of the one-line
// form, and nothing may share that line after it — not even a closing bracket,
// which is why the construct around it breaks however short it is.
func TestCommentInsideAnInlineLambdaBody(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a declaration's value",
			source: "var f = func(): return 1  # c\n",
			want:   "var f = func(): return 1  # c\n",
		},
		{
			name:   "two statements before the comment",
			source: "var f = func(): print(1); return 1  # c\n",
			want:   "var f = func(): print(1); return 1  # c\n",
		},
		{
			name:   "a parameter's default value",
			source: "func g(b, a = func(): return 1  # c\n):\n\tpass\n",
			want:   "func g(\n\t\tb,\n\t\ta = func(): return 1  # c\n\t\t\t,\n):\n\tpass\n",
		},
		{
			name:   "an array element",
			source: "var x = [func(): return 1  # c\n, 2]\n",
			want:   "var x = [\n\tfunc(): return 1  # c\n\t\t,\n\t2,\n]\n",
		},
		{
			name:   "a call argument",
			source: "func w():\n\tg(func(): return 1  # c\n\t)\n",
			want:   "func w():\n\tg(\n\t\t\tfunc(): return 1  # c\n\t\t\t\t,\n\t)\n",
		},
		{
			name:   "a dictionary value",
			source: "var d = {\"k\": func(): return 1  # c\n}\n",
			want:   "var d = {\n\t\"k\": func(): return 1  # c\n\t\t,\n}\n",
		},
		{
			name:   "a subscript index",
			source: "var x = {}\nvar y = x[func(): return 1  # c\n]\n",
			want:   "var x = {}\nvar y = x[func(): return 1  # c\n]\n",
		},
		{
			name:   "a ternary's alternative, which takes parentheses",
			source: "var b = true\nvar y = 1 if b else func(): return 1  # c\n",
			want:   "var b = true\nvar y = 1 if b else (func(): return 1  # c\n\t)\n",
		},
		{
			name:   "an await operand, which takes parentheses",
			source: "var f = await func(): return 1  # c\n",
			want:   "var f = await (func(): return 1  # c\n\t)\n",
		},
		{
			name:   "an if condition, whose colon follows it",
			source: "func w():\n\tif (func(): return 1  # c\n\t\t):\n\t\tpass\n",
			want:   "func w():\n\tif (func(): return 1  # c\n\t\t):\n\t\tpass\n",
		},
		{
			name:   "a while condition",
			source: "func w():\n\twhile (func(): return 1  # c\n\t\t):\n\t\tpass\n",
			want:   "func w():\n\twhile (func(): return 1  # c\n\t\t):\n\t\tpass\n",
		},
		{
			name:   "a match value",
			source: "func w():\n\tmatch (func(): return 1  # c\n\t\t):\n\t\t1:\n\t\t\tpass\n",
			want:   "func w():\n\tmatch (func(): return 1  # c\n\t\t):\n\t\t1:\n\t\t\tpass\n",
		},
		{
			name:   "a dictionary key, whose separator follows it",
			source: "var d = {(func(): return 1  # c\n): 2}\n",
			want:   "var d = {\n\t(func(): return 1  # c\n\t\t): 2,\n}\n",
		},
		{
			name:   "an initializer an accessor block follows",
			source: "var x = (func(): return 1  # c\n\t):\n\tget:\n\t\treturn 1\n",
			want:   "var x = (func(): return 1  # c\n\t):\n\tget:\n\t\treturn 1\n",
		},
		{
			name:   "a match guard, whose colon follows it",
			source: "func w(x):\n\tmatch x:\n\t\t1 when (func(): return true  # c\n\t\t\t):\n\t\t\tpass\n",
			want:   "func w(x):\n\tmatch x:\n\t\t1 when (func(): return true  # c\n\t\t\t):\n\t\t\tpass\n",
		},
		{
			name:   "a member access on a parenthesized lambda",
			source: "var y = (func(): return 1  # c\n).call()\n",
			want:   "var y = (func(): return 1  # c\n\t).call()\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("inline.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := format.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("inline.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := format.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

// A block lambda in a statement header keeps its parentheses, so the statement
// after it stays the statement's own rather than becoming the lambda's.
func TestBlockLambdaInAHeaderKeepsItsParentheses(t *testing.T) {
	source := "func w():\n\tif (func():\n\t\t\t\treturn 1\n\t\t\t\t# c\n\t):\n\t\tpass\n"
	file, err := parser.Parse("header.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	formatted := format.File(file)
	again, err := parser.Parse("header.gd", []byte(formatted))
	if err != nil {
		t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
	}
	branch := again.Statements[0].(*ast.FunctionDeclaration).Body[0].(*ast.IfStatement).Branches[0]
	if len(branch.Body) != 1 {
		t.Fatalf("the if body holds %d statements, want 1:\n%s", len(branch.Body), formatted)
	}
	if keyword, ok := branch.Body[0].(*ast.KeywordStatement); !ok || keyword.Keyword != "pass" {
		t.Fatalf("the if body statement = %#v, want pass:\n%s", branch.Body[0], formatted)
	}
}

// A lambda whose body the one-line form cannot hold is written out instead of
// reaching the inline printer, which supports only what a line can hold.
func TestInlineLambdaFallsBackToABlock(t *testing.T) {
	file, err := parser.Parse("inline.gd", []byte("var f = func(): return 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	lambda := file.Statements[0].(*ast.VariableDeclaration).Value.(*ast.LambdaExpression)
	if !lambda.Inline {
		t.Fatal("the lambda should have been read as inline")
	}
	// A statement no single line can hold, as a tree built by hand may carry.
	lambda.Body = append(lambda.Body, &ast.IfStatement{
		Branches: []ast.Branch{{
			Condition: &ast.Identifier{Name: "ready"},
			Body:      []ast.Statement{&ast.KeywordStatement{Keyword: "pass"}},
		}},
	})
	formatted := format.File(file)
	if !strings.Contains(formatted, "\n") || strings.Contains(formatted, "; if") {
		t.Fatalf("the body should have been written out:\n%s", formatted)
	}
	if _, err := parser.Parse("inline.gd", []byte(formatted)); err != nil {
		t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
	}
}

// TestCommaAfterItemEndingInsideAMatch covers the comma that follows an item
// whose last line lies inside a match statement's case list. Written straight
// after such an item, Godot reads the comma as another pattern, so it takes the
// next line, indented with the body it closes.
func TestCommaAfterItemEndingInsideAMatch(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "the last element of an array",
			source: "var x = [func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n]\n",
			want:   "var x = [\n\tfunc(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t\t,\n]\n",
		},
		{
			name:   "an element another element follows",
			source: "var x = [func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t, 2]\n",
			want:   "var x = [\n\tfunc(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t\t,\n\t2,\n]\n",
		},
		{
			name:   "a match inside the block that ends the body",
			source: "var x = [func(v):\n\t\tfor i in []:\n\t\t\tmatch v:\n\t\t\t\t1:\n\t\t\t\t\tpass\n]\n",
			want:   "var x = [\n\tfunc(v):\n\t\tfor i in []:\n\t\t\tmatch v:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t,\n]\n",
		},
		{
			name:   "a dictionary value",
			source: "var d = {\n\t\"run\": func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n}\n",
			want:   "var d = {\n\t\"run\": func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t\t,\n}\n",
		},
		{
			name:   "a call argument",
			source: "func w(v):\n\tg(func(x):\n\t\t\tmatch x:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t)\n",
			want:   "func w(v):\n\tg(\n\t\t\tfunc(x):\n\t\t\t\tmatch x:\n\t\t\t\t\t1:\n\t\t\t\t\t\tpass\n\t\t\t\t,\n\t)\n",
		},
		{
			name:   "a parameter's default value",
			source: "func f(a = func(v):\n\t\t\tmatch v:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t):\n\tpass\n",
			want:   "func f(\n\t\ta = func(v):\n\t\t\tmatch v:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t\t,\n):\n\tpass\n",
		},
		{
			name:   "an operand that precedence parenthesizes",
			source: "func w(q):\n\tvar x = [not func(a):\n\t\t\tmatch a:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t]\n",
			want:   "func w(q):\n\tvar x = [\n\t\tnot (func(a):\n\t\t\tmatch a:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t\t),\n\t]\n",
		},
		{
			// The parenthesis closes the case list, so the comma follows it on
			// that line rather than taking one of its own.
			name:   "an operand reached through a binary expression",
			source: "func w(q, r):\n\tvar x = [r + func(a):\n\t\t\tmatch a:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t]\n",
			want:   "func w(q, r):\n\tvar x = [\n\t\tr + (func(a):\n\t\t\tmatch a:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t\t),\n\t]\n",
		},
		{
			// A comment at the end of the body sits inside the case body it
			// was written in, so the case list is still open behind it.
			name:   "a body ending in a comment after the match",
			source: "func w(q):\n\tvar x = [not func(v):\n\t\t\tmatch v:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t\t# c\n\t]\n",
			want:   "func w(q):\n\tvar x = [\n\t\tnot (func(v):\n\t\t\tmatch v:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t\t# c\n\t\t\t),\n\t]\n",
		},
		{
			name:   "an await operand",
			source: "func w(q):\n\tvar x = [await func(a):\n\t\t\tmatch a:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t]\n",
			want:   "func w(q):\n\tvar x = [\n\t\tawait (func(a):\n\t\t\tmatch a:\n\t\t\t\t1:\n\t\t\t\t\tpass\n\t\t\t),\n\t]\n",
		},
		{
			// A body that does not end inside a match keeps its comma in place,
			// which is where the style guide wants it.
			name:   "a body that ends in an ordinary statement",
			source: "var x = [func():\n\t\tpass\n]\n",
			want:   "var x = [\n\tfunc():\n\t\tpass,\n]\n",
		},
		{
			name:   "a match followed by an ordinary statement",
			source: "var x = [func(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t\tprint(v)\n]\n",
			want:   "var x = [\n\tfunc(v):\n\t\tmatch v:\n\t\t\t1:\n\t\t\t\tpass\n\t\tprint(v),\n]\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("comma.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := format.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("comma.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := format.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}
