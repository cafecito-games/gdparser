package lexer_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/lexer"
	"github.com/cafecito-games/gdparser/token"
)

const byteOrderMark = "\xef\xbb\xbf"

// A source holding nothing but a mark has no tokens of its own, and the mark
// must not be mistaken for one.
func TestLexByteOrderMarkOnly(t *testing.T) {
	tokens, err := lexer.Lex([]byte(byteOrderMark))
	if err != nil {
		t.Fatalf("Lex: %v", err)
	}
	if len(tokens) == 0 || tokens[len(tokens)-1].Type != token.EOF {
		t.Fatalf("tokens = %v, want a source ending in EOF", tokens)
	}
	for _, tok := range tokens {
		if tok.Type == token.Identifier {
			t.Errorf("a mark alone produced an identifier %q", tok.Lexeme)
		}
	}
}
