package lexer_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/lexer"
	"github.com/cafecito-games/gdparser/token"
)

// Godot's tokenizer takes a line break inside a quoted string as part of its
// text, whether the string is triple-quoted or not, and only the end of the
// source leaves one unterminated.
func TestLexStringHoldingALineBreak(t *testing.T) {
	for _, source := range []string{
		"var s = \"a\n\t\tb\"\n",
		"var s = 'a\n\t\tb'\n",
		"var s = r\"\\t\n\t\t\\t\"\n",
	} {
		tokens, err := lexer.Lex([]byte(source))
		if err != nil {
			t.Fatalf("lex %q: %v", source, err)
		}
		found := false
		for _, tok := range tokens {
			if tok.Type == token.String && strings.Contains(tok.Lexeme, "\n") {
				found = true
			}
		}
		if !found {
			t.Errorf("no string token holding a line break for %q", source)
		}
	}
	if _, err := lexer.Lex([]byte("var s = \"oops\n")); err == nil {
		t.Fatal("a string running to the end of the source should be unterminated")
	}
}

// A comment on a continuation line sits in the middle of a statement, where
// nothing can carry it. Godot discards it; the lexer holds it until the logical
// line ends, so the statement keeps it.
func TestLexCommentOnAContinuationLine(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var a = 1 \\\n\t# one\n\t# two\n\t+ 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []token.Type
	for _, tok := range tokens {
		kinds = append(kinds, tok.Type)
	}
	// The comments come after the whole expression and before its line ends.
	want := []token.Type{
		token.Var, token.Identifier, token.Assign, token.Integer, token.Plus,
		token.Integer, token.Comment, token.Comment, token.Newline, token.EOF,
	}
	if len(kinds) != len(want) {
		t.Fatalf("got %v, want %v", kinds, want)
	}
	for index, typ := range want {
		if kinds[index] != typ {
			t.Fatalf("got %v, want %v", kinds, want)
		}
	}
}
