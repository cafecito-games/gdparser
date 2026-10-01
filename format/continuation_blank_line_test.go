package format_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// A blank line before an elif or else precedes that keyword's own line, which
// belongs to no block, so it must not reappear inside the block the keyword
// opens. A trailing comment makes the mistake visible, because the comment
// becomes the body's first statement and pushes the blank line onto the second.
func TestABlankLineBeforeAContinuationKeywordStaysOut(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "else with a trailing comment",
			source: "func f(x):\n\tif x:\n\t\tpass\n\n\telse:  # c\n\t\tpass\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\telse:  # c\n\t\tpass\n",
		},
		{
			name:   "elif with a trailing comment",
			source: "func f(x):\n\tif x:\n\t\tpass\n\n\telif x:  # c\n\t\tpass\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\telif x:  # c\n\t\tpass\n",
		},
		{
			name:   "several blank lines and a deeper block",
			source: "func f(x):\n\tif x:\n\t\tfor i in []:\n\t\t\tprint(i)\n\n\n\telse:  # c\n\t\tpass\n",
			want:   "func f(x):\n\tif x:\n\t\tfor i in []:\n\t\t\tprint(i)\n\telse:  # c\n\t\tpass\n",
		},
		{
			// A blank line written after the colon is inside the block, and the
			// formatter drops a leading blank line there as it always has.
			name:   "a blank line inside the else body",
			source: "func f(x):\n\tif x:\n\t\tpass\n\telse:\n\n\t\tpass\n",
			want:   "func f(x):\n\tif x:\n\t\tpass\n\telse:\n\t\tpass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("blank.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			first := format.File(file)
			if first != test.want {
				t.Errorf("formatted = %q, want %q", first, test.want)
			}
			again, err := parser.Parse("blank.gd", []byte(first))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if second := format.File(again); second != first {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", first, second)
			}
		})
	}
}

// The count belongs to no statement of the else body, so every statement there
// reports none.
func TestTheElseBodyCarriesNoBlankLineCount(t *testing.T) {
	source := "func f(x):\n\tif x:\n\t\tpass\n\n\telse:  # c\n\t\tpass\n"
	file, err := parser.Parse("blank.gd", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		statement, ok := node.(ast.Statement)
		if !ok {
			return true
		}
		if trivia := ast.TriviaOf(statement); trivia != nil && trivia.BlankLinesBefore != 0 {
			t.Errorf("%T reports %d blank lines before it, want 0", statement, trivia.BlankLinesBefore)
		}
		return true
	})
}
