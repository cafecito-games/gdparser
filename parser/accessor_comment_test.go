package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func TestPropertyAccessorComments(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "ending the colon's line",
			source: "var x: int:  # why\n\tget:\n\t\treturn 1\n",
			want:   "var x: int:  # why\n\tget:\n\t\treturn 1\n",
		},
		{
			name:   "ending the colon's line of an untyped property",
			source: "var x:  # why\n\tget:\n\t\treturn 1\n",
			want:   "var x:  # why\n\tget:\n\t\treturn 1\n",
		},
		{
			name:   "the block's first line",
			source: "var x: int:\n\t# c\n\tget:\n\t\treturn 1\n",
			want:   "var x: int:\n\t# c\n\tget:\n\t\treturn 1\n",
		},
		{
			name:   "a run before the first accessor",
			source: "var x: int:\n\t# a\n\t# b\n\tget:\n\t\treturn 1\n",
			want:   "var x: int:\n\t# a\n\t# b\n\tget:\n\t\treturn 1\n",
		},
		{
			name:   "between the accessors",
			source: "var x: int:\n\tget:\n\t\treturn 1\n\t# c\n\tset(v):\n\t\tpass\n",
			want:   "var x: int:\n\tget:\n\t\treturn 1\n\t# c\n\tset(v):\n\t\tpass\n",
		},
		{
			name:   "after the last accessor",
			source: "var x: int:\n\tget:\n\t\treturn 1\n\t# c\n",
			want:   "var x: int:\n\tget:\n\t\treturn 1\n\t# c\n",
		},
		{
			name:   "a comment inside an accessor body stays there",
			source: "var x: int:\n\tget:\n\t\t# c\n\t\treturn 1\n",
			want:   "var x: int:\n\tget:\n\t\t# c\n\t\treturn 1\n",
		},
		{
			name:   "a blank line before the block",
			source: "var x: int:\n\n\tget:\n\t\treturn 1\n",
			want:   "var x: int:\n\tget:\n\t\treturn 1\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("accessor.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("accessor.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestPropertyAccessorCommentAnchors(t *testing.T) {
	source := "var x: int:  # head\n\t# one\n\tget:\n\t\treturn 1\n\t# two\n\tset(v):\n\t\tpass\n\t# tail\n"
	file, err := parser.Parse("accessor.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	declaration := file.Statements[0].(*ast.VariableDeclaration)
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
	if len(declaration.AccessorComments) != len(want) {
		t.Fatalf("declaration holds %d accessor comments, want %d: %#v",
			len(declaration.AccessorComments), len(want), declaration.AccessorComments)
	}
	for index, expected := range want {
		got := declaration.AccessorComments[index]
		if got.Comment.Text != expected.text || got.Index != expected.index || got.Trailing != expected.trailing {
			t.Errorf("comment %d = {%q, %d, %t}, want {%q, %d, %t}",
				index, got.Comment.Text, got.Index, got.Trailing, expected.text, expected.index, expected.trailing)
		}
	}
	var seen []string
	for _, child := range ast.Children(declaration) {
		if comment, ok := child.(*ast.Comment); ok {
			seen = append(seen, comment.Text)
		}
	}
	if strings.Join(seen, "|") != "# head|# one|# two|# tail" {
		t.Fatalf("children comments = %v, want them in accessor order", seen)
	}
}

// A property has one getter and one setter, so a repeated accessor would
// overwrite the one before it and lose its body. Godot rejects it too.
func TestDuplicatePropertyAccessorIsRejected(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{"two getters", "var x: int:\n\tget:\n\t\treturn 1\n\tget:\n\t\treturn 2\n", "one getter"},
		{"two setters", "var x: int:\n\tset(v):\n\t\tpass\n\tset(w):\n\t\tpass\n", "one setter"},
		{"two getters around a setter", "var x: int:\n\tget:\n\t\treturn 1\n\tset(v):\n\t\tpass\n\tget:\n\t\treturn 2\n", "one getter"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("accessor.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error does not name the rule: %v", err)
			}
			if !strings.Contains(err.Error(), "accessor.gd:") {
				t.Fatalf("error lacks a position: %v", err)
			}
		})
	}
}
