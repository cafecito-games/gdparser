package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func TestMatchComments(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "before the first case",
			source: "func a(x):\n\tmatch x:\n\t\t# c\n\t\t1:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t# c\n\t\t1:\n\t\t\tpass\n",
		},
		{
			name:   "between two cases",
			source: "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# c\n\t\t2:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# c\n\t\t2:\n\t\t\tpass\n",
		},
		{
			name:   "after the last case",
			source: "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# c\n",
			want:   "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# c\n",
		},
		{
			name:   "ending the match line",
			source: "func a(x):\n\tmatch x:  # c\n\t\t1:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:  # c\n\t\t1:\n\t\t\tpass\n",
		},
		{
			name:   "written outside the block",
			source: "func a(x):\n\tmatch x:\n\t# c\n\t\t1:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t# c\n\t\t1:\n\t\t\tpass\n",
		},
		{
			name:   "a run between two cases",
			source: "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# a\n\t\t# b\n\t\t2:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# a\n\t\t# b\n\t\t2:\n\t\t\tpass\n",
		},
		{
			name:   "a documentation comment between cases",
			source: "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t## doc\n\t\t2:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t## doc\n\t\t2:\n\t\t\tpass\n",
		},
		{
			name:   "a comment deeper than the case it follows",
			source: "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t\t\t# c\n\t\t2:\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t\t# c\n\t\t2:\n\t\t\tpass\n",
		},
		{
			name:   "a comment in a nested match",
			source: "func a(x, y):\n\tmatch x:\n\t\t1:\n\t\t\tmatch y:\n\t\t\t\t# inner\n\t\t\t\t2:\n\t\t\t\t\tpass\n",
			want:   "func a(x, y):\n\tmatch x:\n\t\t1:\n\t\t\tmatch y:\n\t\t\t\t# inner\n\t\t\t\t2:\n\t\t\t\t\tpass\n",
		},
		{
			name:   "a comment in a match inside a lambda inside a collection",
			source: "var f = [func(x):\n\t\tmatch x:\n\t\t\t# c\n\t\t\t1:\n\t\t\t\tpass\n]\n",
			want:   "var f = [\n\tfunc(x):\n\t\tmatch x:\n\t\t\t# c\n\t\t\t1:\n\t\t\t\tpass\n\t\t,\n]\n",
		},
		{
			name:   "a lambda that ends with a match before another element",
			source: "var f = [func(x):\n\t\tmatch x:\n\t\t\t# c\n\t\t\t1:\n\t\t\t\tpass\n\t\t, 2]\n",
			want:   "var f = [\n\tfunc(x):\n\t\tmatch x:\n\t\t\t# c\n\t\t\t1:\n\t\t\t\tpass\n\t\t,\n\t2,\n]\n",
		},
		{
			name:   "a comment after the last case at the end of the file",
			source: "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# c",
			want:   "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\tpass\n\t\t# c\n",
		},
		{
			name:   "a comment in a case body stays there",
			source: "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\t# c\n\t\t\tpass\n",
			want:   "func a(x):\n\tmatch x:\n\t\t1:\n\t\t\t# c\n\t\t\tpass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("match.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("match.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestMatchCommentAnchors(t *testing.T) {
	source := "func a(x):\n\tmatch x:  # head\n\t\t# one\n\t\t1:\n\t\t\tpass\n\t\t# two\n\t\t2:\n\t\t\tpass\n\t\t# tail\n"
	file, err := parser.Parse("match.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	match := file.Statements[0].(*ast.FunctionDeclaration).Body[0].(*ast.MatchStatement)
	want := []struct {
		text     string
		index    int
		trailing bool
	}{
		{"# head", 0, true},
		{"# one", 0, false},
		{"# two", 1, false},
		{"# tail", 2, false},
	}
	if len(match.Comments) != len(want) {
		t.Fatalf("match holds %d comments, want %d: %#v", len(match.Comments), len(want), match.Comments)
	}
	for index, expected := range want {
		got := match.Comments[index]
		if got.Comment.Text != expected.text || got.Index != expected.index || got.Trailing != expected.trailing {
			t.Errorf("comment %d = {%q, %d, %t}, want {%q, %d, %t}",
				index, got.Comment.Text, got.Index, got.Trailing, expected.text, expected.index, expected.trailing)
		}
	}
	var seen []string
	for _, child := range ast.Children(match) {
		if comment, ok := child.(*ast.Comment); ok {
			seen = append(seen, comment.Text)
		}
	}
	if strings.Join(seen, "|") != "# head|# one|# two|# tail" {
		t.Fatalf("children comments = %v, want them in source order", seen)
	}
}
