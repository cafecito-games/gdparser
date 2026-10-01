package lexer_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/lexer"
)

// Godot's number() states the rules a numeric literal holds to. Each of these is
// one of its errors.
func TestLexRejectsMalformedNumbers(t *testing.T) {
	for _, test := range []struct{ source, rule string }{
		{"var a = 1__0\n", "adjacent"},
		{"var a = 0x_1\n", "may not open a literal"},
		{"var a = 0b_1\n", "may not open a literal"},
		{"var a = 1.__0\n", "may not follow a decimal point"},
		{"var a = 1.2.3\n", "at most one decimal point"},
		{"var a = 0x1.2\n", "hexadecimal literal has no decimal point"},
		{"var a = 0b1.0\n", "binary literal has no decimal point"},
		{"var a = 1e\n", "exponent value"},
		{"var a = 0x\n", "expected a digit"},
	} {
		_, err := lexer.Lex([]byte(test.source))
		if err == nil {
			t.Errorf("%q was accepted", test.source)
			continue
		}
		if !strings.Contains(err.Error(), test.rule) {
			t.Errorf("%q: error does not name the rule: %v", test.source, err)
		}
	}
	// An underscore may still separate digits anywhere a digit may go.
	for _, source := range []string{
		"var a = 1_000_000\n", "var a = 0xdead_beef\n", "var a = 0b1010_1010\n",
		"var a = 1_0.0_1e1_0\n", "var a = .5\n", "var a = 1e-10\n",
	} {
		if _, err := lexer.Lex([]byte(source)); err != nil {
			t.Errorf("%q was rejected: %v", source, err)
		}
	}
}

// A backslash in a string opens an escape, and Godot accepts only the ones its
// string() lists.
func TestLexRejectsInvalidEscapes(t *testing.T) {
	for _, source := range []string{
		`var a = "\q"` + "\n",
		`var a = "\u00g0"` + "\n",
		`var a = "\U00g000"` + "\n",
	} {
		if _, err := lexer.Lex([]byte(source)); err == nil {
			t.Errorf("%q was accepted", source)
		}
	}
	for _, source := range []string{
		`var a = "\n\t\r\a\b\f\v\'\"\\"` + "\n",
		`var a = "é\U0001F600"` + "\n",
		// A backslash is part of the text of an r-string, not an escape.
		`var a = r"\q"` + "\n",
		// A backslash before a line break escapes the break.
		"var a = \"one\\\ntwo\"\n",
	} {
		if _, err := lexer.Lex([]byte(source)); err != nil {
			t.Errorf("%q was rejected: %v", source, err)
		}
	}
}

// A file indents one way throughout, and a line indents one way within itself.
func TestLexRejectsInconsistentIndentation(t *testing.T) {
	for _, test := range []struct{ source, rule string }{
		{"func w():\n\tpass\nfunc x():\n    pass\n", "where the file indents with a tab"},
		{"func w():\n    pass\nfunc x():\n\tpass\n", "where the file indents with a space"},
		{"func w():\n \tpass\n", "mixes tabs and spaces"},
	} {
		_, err := lexer.Lex([]byte(test.source))
		if err == nil {
			t.Errorf("%q was accepted", test.source)
			continue
		}
		if !strings.Contains(err.Error(), test.rule) {
			t.Errorf("%q: error does not name the rule: %v", test.source, err)
		}
	}
	for _, source := range []string{
		"func w():\n\tif true:\n\t\tpass\n",
		"func w():\n    if true:\n        pass\n",
	} {
		if _, err := lexer.Lex([]byte(source)); err != nil {
			t.Errorf("%q was rejected: %v", source, err)
		}
	}
}
