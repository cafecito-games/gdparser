package lexer

import (
	"testing"

	"github.com/cafecito-games/gdparser/shader/token"
)

func TestCommentsDirectivesAndPositions(t *testing.T) {
	tokens, err := Lex("test.gdshaderinc", []byte("  #define X 1\n/* two\nlines */\nvec3 value; // tail\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Kind != token.Directive || tokens[0].Span.Start.Line != 1 || tokens[0].Span.Start.Column != 3 {
		t.Fatalf("bad directive token: %#v", tokens[0])
	}
	if tokens[1].Kind != token.Comment || tokens[1].Span.End.Line != 3 {
		t.Fatalf("bad block comment: %#v", tokens[1])
	}
	if got := tokens[len(tokens)-2]; got.Kind != token.Comment || got.Span.Start.Line != 4 {
		t.Fatalf("bad trailing comment: %#v", got)
	}
}

func TestUnterminatedBlockComment(t *testing.T) {
	if _, err := Lex("bad.gdshader", []byte("/* no")); err == nil {
		t.Fatal("expected lexical error")
	}
}
