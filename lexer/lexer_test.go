package lexer_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/lexer"
	"github.com/cafecito-games/gdparser/token"
)

func TestLexIndentationAndLocations(t *testing.T) {
	tokens, err := lexer.Lex([]byte("func ready():\n\tif true:\n\t\tpass\n\treturn\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.Func, token.Identifier, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.If, token.True, token.Colon, token.Newline,
		token.Indent, token.Pass, token.Newline, token.Dedent,
		token.Return, token.Newline, token.Dedent, token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %#v", len(tokens), len(want), tokens)
	}
	for i, typ := range want {
		if tokens[i].Type != typ {
			t.Errorf("token %d: got %s, want %s", i, tokens[i].Type, typ)
		}
	}
	if tokens[7].Span.Start.Line != 2 || tokens[7].Span.Start.Column != 2 {
		t.Fatalf("if location = %v", tokens[7].Span.Start)
	}
}

func TestLexRejectsInvalidDedent(t *testing.T) {
	_, err := lexer.Lex([]byte("if true:\n    pass\n  pass\n"))
	if err == nil {
		t.Fatal("expected invalid indentation error")
	}
}

func TestLexMultilineString(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var text = \"\"\"one\ntwo\"\"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tokens[3].Type != token.String || tokens[3].Lexeme != "\"\"\"one\ntwo\"\"\"" {
		t.Fatalf("unexpected string token: %#v", tokens[3])
	}
}

func TestLexContinuedExpressionDoesNotCreateIndent(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var values = [\n    1,\n    2] + other\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range tokens {
		if tok.Type == token.Indent || tok.Type == token.Dedent {
			t.Fatalf("unexpected %s token at %s", tok.Type, tok.Span.Start)
		}
	}
}
