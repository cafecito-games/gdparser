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

func TestLexOwnLineCommaEndsALambdaBody(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var x = [\n\tfunc():\n\t\tpass\n\t,\n\t2,\n]\n"))
	if err != nil {
		t.Fatal(err)
	}
	var kinds []token.Type
	for _, tok := range tokens {
		kinds = append(kinds, tok.Type)
	}
	var dedents int
	for _, kind := range kinds {
		if kind == token.Dedent {
			dedents++
		}
	}
	if dedents != 1 {
		t.Fatalf("got %d DEDENT tokens, want 1: %v", dedents, kinds)
	}
}

func TestLexNestedLambdaLayoutsUnwindIndependently(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var x = [func():\n\t\tf(func():\n\t\t\t\tpass\n\t\t)\n\t\tpass\n]\n"))
	if err != nil {
		t.Fatal(err)
	}
	// The inner body closes with the parenthesis that ends it, and the outer
	// body stays open for the statement that follows.
	want := []token.Type{
		token.Var, token.Identifier, token.Assign, token.LBracket,
		token.Func, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.Identifier, token.LParen,
		token.Func, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.Pass, token.Newline, token.Dedent,
		token.RParen, token.Newline,
		token.Pass, token.Newline, token.Dedent,
		token.RBracket, token.Newline, token.EOF,
	}
	var kinds []token.Type
	for _, tok := range tokens {
		kinds = append(kinds, tok.Type)
	}
	if len(kinds) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(kinds), len(want), kinds)
	}
	for i, typ := range want {
		if kinds[i] != typ {
			t.Fatalf("token %d: got %s, want %s: %v", i, kinds[i], typ, kinds)
		}
	}
}

func TestLexCommentUnindentedToNoOuterBlock(t *testing.T) {
	tokens, err := lexer.Lex([]byte("func a():\n\tpass\n  # c\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.Func, token.Identifier, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.Pass, token.Newline, token.Dedent,
		token.Comment, token.Newline, token.EOF,
	}
	var kinds []token.Type
	for _, tok := range tokens {
		kinds = append(kinds, tok.Type)
	}
	if len(kinds) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(kinds), len(want), kinds)
	}
	for i, typ := range want {
		if kinds[i] != typ {
			t.Fatalf("token %d: got %s, want %s: %v", i, kinds[i], typ, kinds)
		}
	}
}

func TestLexCommentDoesNotCloseABlockTheCodeBelowReenters(t *testing.T) {
	tokens, err := lexer.Lex([]byte("func a():\n\tif true:\n\t\tpass\n\t# c\n\t\tprint()\n"))
	if err != nil {
		t.Fatal(err)
	}
	// The comment sits at the function body's level, but the line below it
	// returns to the if body, so no block closes at the comment.
	want := []token.Type{
		token.Func, token.Identifier, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.If, token.True, token.Colon, token.Newline,
		token.Indent, token.Pass, token.Newline,
		token.Comment, token.Newline,
		token.Identifier, token.LParen, token.RParen, token.Newline,
		token.Dedent, token.Dedent, token.EOF,
	}
	var kinds []token.Type
	for _, tok := range tokens {
		kinds = append(kinds, tok.Type)
	}
	if len(kinds) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(kinds), len(want), kinds)
	}
	for i, typ := range want {
		if kinds[i] != typ {
			t.Fatalf("token %d: got %s, want %s: %v", i, kinds[i], typ, kinds)
		}
	}
}

// The lookahead skips the rest of the comment line and any blank line between
// the comment and the code, with either line ending.
func TestLexCommentLookaheadSkipsBlankCarriageReturnLines(t *testing.T) {
	tokens, err := lexer.Lex([]byte("func a():\r\n\tif true:\r\n\t\tpass\r\n\t# c\r\n\r\n\t\tprint()\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []token.Type{
		token.Func, token.Identifier, token.LParen, token.RParen, token.Colon, token.Newline,
		token.Indent, token.If, token.True, token.Colon, token.Newline,
		token.Indent, token.Pass, token.Newline,
		token.Comment, token.Newline,
		// The blank line between the comment and the code ends with a newline
		// of its own, as a blank line does anywhere.
		token.Newline,
		token.Identifier, token.LParen, token.RParen, token.Newline,
		token.Dedent, token.Dedent, token.EOF,
	}
	var kinds []token.Type
	for _, tok := range tokens {
		kinds = append(kinds, tok.Type)
	}
	if len(kinds) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(kinds), len(want), kinds)
	}
	for i, typ := range want {
		if kinds[i] != typ {
			t.Fatalf("token %d: got %s, want %s: %v", i, kinds[i], typ, kinds)
		}
	}
}

func TestLexCommentStaysInsideTheInnerLambdaBody(t *testing.T) {
	tokens, err := lexer.Lex([]byte("var x = [func():\n\t\tvar y = [func():\n\t\t\t\tpass\n\t\t# c\n\t\t]\n]\n"))
	if err != nil {
		t.Fatal(err)
	}
	// Only the comma or bracket that ends a lambda body may leave it, so the
	// comment below the inner body's level keeps that body open.
	var kinds []token.Type
	var firstDedent, comment int
	for index, tok := range tokens {
		kinds = append(kinds, tok.Type)
		if tok.Type == token.Dedent && firstDedent == 0 {
			firstDedent = index
		}
		if tok.Type == token.Comment {
			comment = index
		}
	}
	if comment == 0 {
		t.Fatalf("no comment token: %v", kinds)
	}
	if firstDedent < comment {
		t.Fatalf("the inner body closed before its comment: %v", kinds)
	}
}
