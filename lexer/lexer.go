// Package lexer tokenizes GDScript source, including its indentation structure.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cafecito-games/gdparser/token"
)

// Error describes invalid source encountered by the lexer.
type Error struct {
	Position token.Position
	Message  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Position, e.Message) }

// Lex returns all tokens in source. Indentation is emitted as INDENT and DEDENT.
func Lex(source []byte) ([]token.Token, error) {
	l := &lexer{
		source:  source,
		line:    1,
		column:  1,
		atStart: true,
		indents: []int{0},
	}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

type lexer struct {
	source  []byte
	offset  int
	line    int
	column  int
	atStart bool
	depth   int
	indents []int
	tokens  []token.Token
}

func (l *lexer) run() error {
	for !l.done() {
		if l.atStart && l.depth == 0 {
			if err := l.scanIndent(); err != nil {
				return err
			}
			if l.done() {
				break
			}
		}

		start := l.position()
		c := l.peek()
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			l.advance()
		case c == '\n':
			l.advance()
			if l.depth == 0 {
				l.emit(token.Newline, "", start)
			}
			l.atStart = true
		case c == '#':
			l.scanComment(start)
		case isIdentifierStart(c):
			l.scanIdentifier(start)
		case isDigit(c):
			l.scanNumber(start)
		case c == '\'' || c == '"':
			if err := l.scanString(start); err != nil {
				return err
			}
		default:
			if err := l.scanOperator(start); err != nil {
				return err
			}
		}
	}

	if len(l.tokens) == 0 || l.tokens[len(l.tokens)-1].Type != token.Newline {
		p := l.position()
		l.emit(token.Newline, "", p)
	}
	for len(l.indents) > 1 {
		l.indents = l.indents[:len(l.indents)-1]
		p := l.position()
		l.emit(token.Dedent, "", p)
	}
	p := l.position()
	l.emit(token.EOF, "", p)
	return nil
}

func (l *lexer) scanIndent() error {
	startOffset := l.offset
	columns := 0
	for !l.done() {
		switch l.peek() {
		case ' ':
			columns++
			l.advance()
		case '\t':
			columns += 4 - columns%4
			l.advance()
		default:
			goto measured
		}
	}

measured:
	// Blank and comment-only lines do not affect the indentation stack.
	if l.done() || l.peek() == '\n' || l.peek() == '\r' || l.peek() == '#' {
		return nil
	}
	l.atStart = false
	top := l.indents[len(l.indents)-1]
	p := token.Position{Offset: startOffset, Line: l.line, Column: 1}
	if columns > top {
		l.indents = append(l.indents, columns)
		l.emit(token.Indent, "", p)
		return nil
	}
	if columns < top {
		for len(l.indents) > 1 && columns < l.indents[len(l.indents)-1] {
			l.indents = l.indents[:len(l.indents)-1]
			l.emit(token.Dedent, "", p)
		}
		if columns != l.indents[len(l.indents)-1] {
			return &Error{Position: p, Message: "indentation does not match an outer block"}
		}
	}
	return nil
}

func (l *lexer) scanComment(start token.Position) {
	l.atStart = false
	begin := l.offset
	for !l.done() && l.peek() != '\n' {
		l.advance()
	}
	l.emit(token.Comment, string(l.source[begin:l.offset]), start)
}

func (l *lexer) scanIdentifier(start token.Position) {
	l.atStart = false
	begin := l.offset
	for !l.done() && isIdentifierPart(l.peek()) {
		l.advance()
	}
	text := string(l.source[begin:l.offset])
	l.emit(token.LookupIdentifier(text), text, start)
}

func (l *lexer) scanNumber(start token.Position) {
	l.atStart = false
	begin := l.offset
	if l.peek() == '0' && (l.peekN(1) == 'x' || l.peekN(1) == 'X' || l.peekN(1) == 'b' || l.peekN(1) == 'B') {
		l.advance()
		l.advance()
		for !l.done() && (isDigit(l.peek()) || isHexLetter(l.peek()) || l.peek() == '_') {
			l.advance()
		}
		l.emit(token.Integer, string(l.source[begin:l.offset]), start)
		return
	}
	for !l.done() && (isDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}
	typ := token.Integer
	if l.peek() == '.' && isDigit(l.peekN(1)) {
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
	l.emit(typ, string(l.source[begin:l.offset]), start)
}

func (l *lexer) scanString(start token.Position) error {
	l.atStart = false
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
		if l.peek() == '\n' && !triple {
			return &Error{Position: start, Message: "unterminated string literal"}
		}
		l.advance()
	}
	return &Error{Position: start, Message: "unterminated string literal"}
}

func (l *lexer) scanOperator(start token.Position) error {
	l.atStart = false
	operators := []struct {
		text string
		typ  token.Type
	}{
		{"<<=", token.ShiftLeftAssign}, {">>=", token.ShiftRightAssign},
		{"**", token.DoubleStar}, {"->", token.Arrow}, {":=", token.InferAssign},
		{"==", token.Equal}, {"!=", token.NotEqual}, {"<=", token.LessEqual},
		{">=", token.GreaterEqual}, {"<<", token.ShiftLeft}, {">>", token.ShiftRight},
		{"+=", token.PlusAssign}, {"-=", token.MinusAssign}, {"*=", token.StarAssign},
		{"/=", token.SlashAssign}, {"%=", token.PercentAssign}, {"&=", token.AmpAssign},
		{"|=", token.PipeAssign}, {"^=", token.CaretAssign},
		{"(", token.LParen}, {")", token.RParen}, {"[", token.LBracket}, {"]", token.RBracket},
		{"{", token.LBrace}, {"}", token.RBrace}, {",", token.Comma}, {":", token.Colon},
		{".", token.Dot}, {"@", token.At}, {"$", token.Dollar}, {"%", token.Percent},
		{"=", token.Assign}, {"+", token.Plus}, {"-", token.Minus}, {"*", token.Star},
		{"/", token.Slash}, {"<", token.Less}, {">", token.Greater}, {"&", token.Ampersand},
		{"|", token.Pipe}, {"^", token.Caret}, {"~", token.Tilde}, {"!", token.Bang},
	}
	remaining := string(l.source[l.offset:])
	for _, op := range operators {
		if strings.HasPrefix(remaining, op.text) {
			for range len(op.text) {
				l.advance()
			}
			switch op.typ {
			case token.LParen, token.LBracket, token.LBrace:
				l.depth++
			case token.RParen, token.RBracket, token.RBrace:
				if l.depth > 0 {
					l.depth--
				}
			}
			l.emit(op.typ, op.text, start)
			return nil
		}
	}
	r, _ := utf8.DecodeRune(l.source[l.offset:])
	return &Error{Position: start, Message: fmt.Sprintf("unexpected character %q", r)}
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

func (l *lexer) advance() byte {
	c := l.source[l.offset]
	l.offset++
	if c == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return c
}

func (l *lexer) position() token.Position {
	return token.Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func isIdentifierStart(c byte) bool {
	return c == '_' || c >= utf8.RuneSelf || unicode.IsLetter(rune(c))
}
func isIdentifierPart(c byte) bool { return isIdentifierStart(c) || isDigit(c) }
func isDigit(c byte) bool          { return c >= '0' && c <= '9' }
func isHexLetter(c byte) bool      { return c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
