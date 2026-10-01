package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// TestCollectionCommentsSurviveFormatting covers every bracketed shape that can
// hold a comment between its items.
func TestCollectionCommentsSurviveFormatting(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"array element", "var x = [\n\t1,\n\t\t# c\n\t2,\n]\n"},
		{"array after the last element", "var x = [\n\t1,\n\t# c\n]\n"},
		{"array end of line", "var x = [\n\t1,  # c\n\t2,\n]\n"},
		{"array opening line", "var x = [  # c\n\t1,\n]\n"},
		{"array holding only a comment", "var x = [\n\t# c\n]\n"},
		{"array before the first element", "var x = [\n\t# c\n\t1,\n]\n"},
		{"dictionary entry", "var d = {\n\t\"a\": 1,\n\t# c\n\t\"b\": 2,\n}\n"},
		{"dictionary after the last entry", "var d = {\n\t\"a\": 1,\n\t# c\n}\n"},
		{"call argument", "func w():\n\tf(\n\t\t1,\n\t\t# c\n\t\t2,\n\t)\n"},
		{"call with only a comment", "func w():\n\tf(\n\t\t# c\n\t)\n"},
		{"parameter", "func f(\n\ta,\n\t# c\n\tb\n):\n\tpass\n"},
		{"lambda parameter", "var g := func(\n\t# c\n\ta\n):\n\tpass\n"},
		{"signal parameter", "signal s(\n\t# c\n\ta: int\n)\n"},
		{"annotation argument", "@export_range(\n\t# c\n\t0,\n\t10\n)\nvar x := 1\n"},
		{"enum member", "enum E {\n\t# c\n\tA,\n}\n"},
		{"enum after the last member", "enum E {\n\tA,\n\t# c\n}\n"},
		{"enum end of line", "enum E {\n\tA,  # c\n\tB,\n}\n"},
		{"nested collection", "var x = [\n\t[\n\t\t1,\n\t\t# c\n\t],\n]\n"},
		{"opening line and own line", "var x = [  # c\n\t# d\n]\n"},
		{"empty annotation argument list", "@e(\n\t# c\n)\nvar x := 1\n"},
		{"empty signal parameter list", "signal s(\n\t# c\n)\n"},
		{"empty parameter list", "func f(\n\t# c\n):\n\tpass\n"},
		{"empty lambda parameter list", "var g := func(\n\t# c\n):\n\tpass\n"},
		{"empty dictionary", "var d = {\n\t# c\n}\n"},
		{"empty enum", "enum E {\n\t# c\n}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("comments.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			for _, comment := range commentsIn(test.source) {
				if !strings.Contains(formatted, comment) {
					t.Fatalf("formatted source dropped %q:\n%s", comment, formatted)
				}
			}
			again, err := parser.Parse("comments.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestCollectionCommentAnchors(t *testing.T) {
	file, err := parser.Parse("anchors.gd", []byte("var x = [\n\t1,  # one\n\t# two\n\t2,\n\t# three\n]\n"))
	if err != nil {
		t.Fatal(err)
	}
	array := file.Statements[0].(*ast.VariableDeclaration).Value.(*ast.ArrayLiteral)
	want := []struct {
		text     string
		index    int
		trailing bool
	}{
		{"# one", 1, true},
		{"# two", 1, false},
		{"# three", 2, false},
	}
	if len(array.Comments) != len(want) {
		t.Fatalf("array holds %d comments, want %d: %#v", len(array.Comments), len(want), array.Comments)
	}
	for index, expected := range want {
		got := array.Comments[index]
		if got.Comment.Text != expected.text || got.Index != expected.index || got.Trailing != expected.trailing {
			t.Errorf("comment %d = {%q, %d, %t}, want {%q, %d, %t}",
				index, got.Comment.Text, got.Index, got.Trailing, expected.text, expected.index, expected.trailing)
		}
	}
}

func TestCollectionCommentsAreChildrenInSourceOrder(t *testing.T) {
	file, err := parser.Parse("children.gd", []byte("var x = [\n\t# one\n\t1,\n\t# two\n]\n"))
	if err != nil {
		t.Fatal(err)
	}
	array := file.Statements[0].(*ast.VariableDeclaration).Value.(*ast.ArrayLiteral)
	children := ast.Children(array)
	var got []string
	for _, child := range children {
		switch node := child.(type) {
		case *ast.Comment:
			got = append(got, node.Text)
		case *ast.Literal:
			got = append(got, node.Raw)
		}
	}
	if strings.Join(got, "|") != "# one|1|# two" {
		t.Fatalf("children = %v, want [# one 1 # two]", got)
	}
}

// commentsIn returns the comment text of every line in source that holds one.
// The sources in this file hold no string literal containing a hash, so the
// first hash on a line always opens its comment.
func commentsIn(source string) []string {
	var comments []string
	for _, line := range strings.Split(source, "\n") {
		if index := strings.IndexByte(line, '#'); index >= 0 {
			comments = append(comments, strings.TrimRight(line[index:], " \t"))
		}
	}
	return comments
}
