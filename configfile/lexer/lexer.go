// Package lexer tokenizes Godot ConfigFile syntax.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cafecito-games/gdparser/configfile/token"
	"github.com/cafecito-games/gdparser/internal/encoding"
)

// Error describes invalid source encountered by the lexer.
type Error struct {
	Position token.Position
	Message  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Position, e.Message) }

// Lex returns every token in source, including comments and newlines. A leading
// byte order mark is skipped, since it says how the file is encoded rather than
// anything the ConfigFile grammar can read.
func Lex(source []byte) ([]token.Token, error) {
	l := &lexer{source: source, offset: encoding.SkipByteOrderMark(source), line: 1, column: 1}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

type lexer struct {
	source []byte
	offset int
	line   int
	column int
	tokens []token.Token
}

func (l *lexer) run() error {
	for !l.done() {
		start := l.position()
		c := l.peek()
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			l.advance()
		case c == '\n':
			l.advance()
			l.emit(token.Newline, "", start)
		case c == ';' || c == '#':
			l.scanComment(start)
		case c == '"' || c == '\'':
			if err := l.scanString(start); err != nil {
				return err
			}
		case isDigit(c) || (c == '.' && isDigit(l.peekN(1))):
			l.scanNumber(start)
		case isIdentifierStart(c) || c == '\\':
			l.scanIdentifier(start)
		default:
			if err := l.scanPunctuation(start); err != nil {
				return err
			}
		}
	}
	p := l.position()
	l.emit(token.EOF, "", p)
	return nil
}

func (l *lexer) scanComment(start token.Position) {
	begin := l.offset
	for !l.done() && l.peek() != '\n' && l.peek() != '\r' {
		l.advance()
	}
	l.emit(token.Comment, string(l.source[begin:l.offset]), start)
}

func (l *lexer) scanString(start token.Position) error {
	begin := l.offset
	quote := l.advance()
	triple := l.peek() == quote && l.peekN(1) == quote
	if triple {
		l.advance()
		l.advance()
	}
	for !l.done() {
		if l.peek() == '\\' {
			l.advance()
			if !l.done() {
				l.advance()
			}
			continue
		}
		if l.peek() == quote {
			l.advance()
			if !triple || (l.peek() == quote && l.peekN(1) == quote) {
				if triple {
					l.advance()
					l.advance()
				}
				l.emit(token.String, string(l.source[begin:l.offset]), start)
				return nil
			}
			continue
		}
		l.advance()
	}
	return &Error{Position: start, Message: "unterminated string literal"}
}

func (l *lexer) scanNumber(start token.Position) {
	begin := l.offset
	typ := token.Integer
	if l.peek() == '.' {
		typ = token.Float
		l.advance()
	}
	if l.peek() == '0' && strings.ContainsRune("xXbB", rune(l.peekN(1))) {
		l.advance()
		l.advance()
		for !l.done() && (isDigit(l.peek()) || isHexLetter(l.peek()) || l.peek() == '_') {
			l.advance()
		}
		for !l.done() && strings.ContainsRune("UL", rune(l.peek())) {
			l.advance()
		}
		l.emit(token.Integer, string(l.source[begin:l.offset]), start)
		return
	}
	for !l.done() && (isDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}
	if l.peek() == '.' {
		typ = token.Float
		l.advance()
		for !l.done() && (isDigit(l.peek()) || l.peek() == '_') {
			l.advance()
		}
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		typ = token.Float
		l.advance()
		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}
		for !l.done() && (isDigit(l.peek()) || l.peek() == '_') {
			l.advance()
		}
	}
	if typ == token.Integer {
		for !l.done() && strings.ContainsRune("UL", rune(l.peek())) {
			l.advance()
		}
	}
	l.emit(typ, string(l.source[begin:l.offset]), start)
}

func (l *lexer) scanIdentifier(start token.Position) {
	begin := l.offset
	for !l.done() {
		c := l.peek()
		if isIdentifierPart(c) {
			l.advance()
			continue
		}
		// A simple section tag escapes a literal closing bracket as \]. Keep
		// both bytes in the identifier; the parser decodes it in the name.
		if c == '\\' && l.peekN(1) == ']' {
			l.advance()
			l.advance()
			continue
		}
		if c == '\\' {
			l.advance()
			continue
		}
		break
	}
	l.emit(token.Identifier, string(l.source[begin:l.offset]), start)
}

func (l *lexer) scanPunctuation(start token.Position) error {
	punctuation := map[byte]token.Type{
		'(': token.LParen, ')': token.RParen,
		'[': token.LBracket, ']': token.RBracket,
		'{': token.LBrace, '}': token.RBrace,
		',': token.Comma, ':': token.Colon, '=': token.Assign,
		'+': token.Plus, '-': token.Minus, '&': token.Ampersand, '^': token.Caret,
	}
	c := l.advance()
	if typ, ok := punctuation[c]; ok {
		l.emit(typ, string(c), start)
		return nil
	}
	return &Error{Position: start, Message: fmt.Sprintf("unexpected character %q", c)}
}

func (l *lexer) emit(typ token.Type, lexeme string, start token.Position) {
	l.tokens = append(l.tokens, token.Token{Type: typ, Lexeme: lexeme, Span: token.Span{Start: start, End: l.position()}})
}

func (l *lexer) done() bool { return l.offset >= len(l.source) }

func (l *lexer) peek() byte {
	if l.done() {
		return 0
	}
	return l.source[l.offset]
}

func (l *lexer) peekN(n int) byte {
	if l.offset+n >= len(l.source) {
		return 0
	}
	return l.source[l.offset+n]
}

func (l *lexer) position() token.Position {
	return token.Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func (l *lexer) advance() byte {
	c := l.source[l.offset]
	if c == '\n' {
		l.line++
		l.column = 1
		l.offset++
		return c
	}
	if c < utf8.RuneSelf {
		l.offset++
		l.column++
		return c
	}
	_, size := utf8.DecodeRune(l.source[l.offset:])
	l.offset += size
	l.column++
	return c
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexLetter(c byte) bool {
	return c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func isIdentifierStart(c byte) bool {
	if c >= utf8.RuneSelf {
		return true
	}
	return c == '_' || c == '@' || c == '$' || c == '*' || unicode.IsLetter(rune(c))
}

func isIdentifierPart(c byte) bool {
	if c >= utf8.RuneSelf {
		return true
	}
	if c <= ' ' {
		return false
	}
	return !strings.ContainsRune("()[]{},:=+-&#^;'\"\\", rune(c))
}
