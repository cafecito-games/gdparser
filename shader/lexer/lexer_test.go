package lexer

import (
	"strings"
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

// A carriage return at the end of a line ends the line rather than belonging to
// the text before it, so a comment or a directive read from a CRLF file carries
// none into output whose other lines end with a line feed alone.
func TestCommentAndDirectiveExcludeACarriageReturn(t *testing.T) {
	tokens, err := Lex("t.gdshader", []byte("#define X 1\r\n// a comment\r\nvoid f() {}\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range tokens {
		if strings.Contains(tok.Text, "\r") {
			t.Errorf("token %s holds a carriage return: %q", tok.Kind, tok.Text)
		}
	}
	// A comment that runs to the end of the file is no different.
	last, err := Lex("t.gdshader", []byte("// a comment\r"))
	if err != nil {
		t.Fatal(err)
	}
	if last[0].Text != "// a comment" {
		t.Errorf("comment = %q, want %q", last[0].Text, "// a comment")
	}
}
