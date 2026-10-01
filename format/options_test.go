package format_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func formatWith(t *testing.T, source string, options format.Options) string {
	t.Helper()
	file, err := parser.Parse("test.gd", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return format.FileWithOptions(file, options)
}

// TestZeroOptionsMatchGodotStyle documents that every field's zero value is the
// style guide default, so a partially populated Options inherits the rest.
func TestZeroOptionsMatchGodotStyle(t *testing.T) {
	source := "var a := 'q'\nvar b := .5\nfunc f(alpha, beta, gamma, delta, epsilon, zeta, eta, theta, iota, kappa, lambda_) -> void:\n\tpass\n"
	file, err := parser.Parse("test.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	want := format.FileWithOptions(file, format.GodotStyle())
	if got := format.FileWithOptions(file, format.Options{}); got != want {
		t.Fatalf("zero options differ from GodotStyle\n--- got ---\n%s--- want ---\n%s", got, want)
	}
	if plain := format.File(file); plain != want {
		t.Fatalf("File differs from GodotStyle\n--- got ---\n%s--- want ---\n%s", plain, want)
	}
}

func TestOptionCases(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		options format.Options
		want    string
	}{
		{
			name:    "a narrower budget breaks earlier",
			source:  "var a := [1, 2, 3]\n",
			options: format.Options{LineWidth: 10},
			want:    "var a := [\n\t1,\n\t2,\n\t3,\n]\n",
		},
		{
			name:    "spaces indent by the tab width",
			source:  "func f() -> void:\n\tpass\n",
			options: format.Options{Indent: format.Spaces, TabWidth: 2},
			want:    "func f() -> void:\n  pass\n",
		},
		{
			name:    "single quotes can be preferred",
			source:  "var a := \"text\"\n",
			options: format.Options{QuoteStyle: format.SingleQuotes},
			want:    "var a := 'text'\n",
		},
		{
			name:    "quotes can be left alone",
			source:  "var a := 'text'\n",
			options: format.Options{QuoteStyle: format.PreserveQuotes},
			want:    "var a := 'text'\n",
		},
		{
			name:    "comment spacing can be left alone",
			source:  "#no space\n",
			options: format.Options{CommentSpacing: format.PreserveComments},
			want:    "#no space\n",
		},
		{
			name:    "operator spelling can be left alone",
			source:  "var a := b && c\n",
			options: format.Options{Operators: format.PreserveOperators},
			want:    "var a := b && c\n",
		},
		{
			name:    "numeric spelling can be left alone",
			source:  "var a := .5\nvar b := 0XFF\n",
			options: format.Options{Numbers: format.PreserveNumbers},
			want:    "var a := .5\nvar b := 0XFF\n",
		},
		{
			name:    "trailing commas can be suppressed",
			source:  "var a := [1, 2, 3]\n",
			options: format.Options{LineWidth: 10, TrailingCommas: format.NoTrailingCommas},
			want:    "var a := [\n\t1,\n\t2,\n\t3\n]\n",
		},
		{
			name:    "blank line counts are configurable",
			source:  "var a := 1\nfunc f() -> void:\n\tpass\n",
			options: format.Options{BlankLines: format.BlankLineLimits{TopLevel: 1, Nested: 1}},
			want:    "var a := 1\n\nfunc f() -> void:\n\tpass\n",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := formatWith(t, testCase.source, testCase.options); got != testCase.want {
				t.Fatalf("formatted output\n--- got ---\n%s--- want ---\n%s", got, testCase.want)
			}
		})
	}
}

// TestPreserveNumbersStillParses guards the one option that can emit a literal
// the formatter would otherwise have normalized.
func TestPreserveNumbersStillParses(t *testing.T) {
	source := "var a := .5\n"
	output := formatWith(t, source, format.Options{Numbers: format.PreserveNumbers})
	if _, err := parser.Parse("test.gd", []byte(output)); err != nil {
		t.Fatalf("output did not parse: %v\n%s", err, output)
	}
}
