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

func TestLexSkipsByteOrderMark(t *testing.T) {
	tokens, err := lexer.Lex([]byte("\xef\xbb\xbfpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != token.Pass {
		t.Fatalf("first token = %s, want %s", tokens[0].Type, token.Pass)
	}
	// The mark occupies no column, so the statement still starts the line.
	if start := tokens[0].Span.Start; start.Line != 1 || start.Column != 1 {
		t.Fatalf("first token at %d:%d, want 1:1", start.Line, start.Column)
	}
}

func TestLexRawStringKeepsItsPrefix(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var a = r\"\\d+\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	var found *token.Token
	for index := range tokens {
		if tokens[index].Type == token.String {
			found = &tokens[index]
			break
		}
	}
	if found == nil {
		t.Fatal("no string token")
	}
	if found.Lexeme != `r"\d+"` {
		t.Fatalf("lexeme = %s, want %s", found.Lexeme, `r"\d+"`)
	}
}

// An identifier beginning with r is not mistaken for a raw string prefix.
func TestLexIdentifierStartingWithR(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var radius = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tokens[1].Type != token.Identifier || tokens[1].Lexeme != "radius" {
		t.Fatalf("token 1 = %s", tokens[1])
	}
}

func TestLexLeadingDotFloat(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var a = .5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tokens[3].Type != token.Float || tokens[3].Lexeme != ".5" {
		t.Fatalf("token 3 = %s, want FLOAT(\".5\")", tokens[3])
	}
}

// Member access on a number-like property must still lex as a dot.
func TestLexDotIsNotSwallowedAfterAnIdentifier(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var a = b.c\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tokens[4].Type != token.Dot {
		t.Fatalf("token 4 = %s, want %s", tokens[4].Type, token.Dot)
	}
}

func TestLexCommentIndentedDeeperThanItsBlock(t *testing.T) {
	tokens, err := lexer.Lex([]byte("func a():\n\tpass\n\t\t# deeper\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.Func, token.Identifier, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.Pass, token.Newline,
		token.Comment, token.Newline,
		token.Dedent, token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %#v", len(tokens), len(want), tokens)
	}
	for i, typ := range want {
		if tokens[i].Type != typ {
			t.Errorf("token %d: got %s, want %s", i, tokens[i].Type, typ)
		}
	}
}

func TestLexCommentAtColumnZeroStillDedents(t *testing.T) {
	tokens, err := lexer.Lex([]byte("func a():\n\tpass\n# outer\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.Func, token.Identifier, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.Pass, token.Newline, token.Dedent,
		token.Comment, token.Newline, token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %#v", len(tokens), len(want), tokens)
	}
	for i, typ := range want {
		if tokens[i].Type != typ {
			t.Errorf("token %d: got %s, want %s", i, tokens[i].Type, typ)
		}
	}
}
