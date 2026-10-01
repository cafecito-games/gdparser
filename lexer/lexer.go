// Package lexer tokenizes GDScript source, including its indentation structure.
package lexer

import (
	"bytes"
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

// byteOrderMark is the UTF-8 encoding of U+FEFF.
var byteOrderMark = []byte{0xef, 0xbb, 0xbf}

// Lex returns all tokens in source. Indentation is emitted as INDENT and DEDENT.
func Lex(source []byte) ([]token.Token, error) {
	l := &lexer{
		source:  source,
		line:    1,
		column:  1,
		atStart: true,
		indents: []int{0},
	}
	// A UTF-8 byte order mark carries no syntax. Godot's style guide asks for
	// files without one, so it is skipped rather than rejected, which lets a
	// formatter rewrite such a file cleanly.
	if bytes.HasPrefix(source, byteOrderMark) {
		l.offset = len(byteOrderMark)
	}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

type lexer struct {
	source            []byte
	offset            int
	line              int
	column            int
	atStart           bool
	depth             int
	layoutDepth       int
	layoutIndentDepth int
	lambdaHeaderDepth int
	indents           []int
	tokens            []token.Token
}

func (l *lexer) run() error {
	for !l.done() {
		if l.atStart && l.depth == l.layoutDepth {
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
			if l.depth == l.layoutDepth {
				l.emit(token.Newline, "", start)
			}
			l.atStart = true
		case c == '\\' && l.peekN(1) == '\n':
			l.scanContinuation()
		case c == '#':
			l.scanComment(start)
		case c == 'r' && (l.peekN(1) == '"' || l.peekN(1) == '\''):
			// A raw string literal, r"...", where backslashes are literal.
			l.advance()
			if err := l.scanString(start); err != nil {
				return err
			}
		case isIdentifierStart(c):
			l.scanIdentifier(start)
		case isDigit(c):
			l.scanNumber(start)
		case c == '.' && isDigit(l.peekN(1)):
			// A float with its leading zero omitted, as in .5.
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
	// Blank lines do not affect the indentation stack. Comment indentation is
	// significant to the AST even though Godot ignores it syntactically.
	if l.done() || l.peek() == '\n' || l.peek() == '\r' {
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

func (l *lexer) scanContinuation() {
	l.advance() // backslash
	l.advance() // newline
	for !l.done() && (l.peek() == ' ' || l.peek() == '\t' || l.peek() == '\r') {
		l.advance()
	}
	l.atStart = false
}

func (l *lexer) scanIdentifier(start token.Position) {
	l.atStart = false
	begin := l.offset
	for !l.done() && isIdentifierPart(l.peek()) {
		l.advance()
	}
	text := string(l.source[begin:l.offset])
	typ := token.LookupIdentifier(text)
	l.emit(typ, text, start)
	if typ == token.Func && l.depth > l.layoutDepth {
		l.lambdaHeaderDepth = l.depth
	}
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
	if l.peek() == '.' && l.peekN(1) != '.' {
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
	begin := start.Offset
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
		{"&&", token.And}, {"||", token.Or},
		{"**", token.DoubleStar}, {"->", token.Arrow}, {":=", token.InferAssign},
		{"==", token.Equal}, {"!=", token.NotEqual}, {"<=", token.LessEqual},
		{">=", token.GreaterEqual}, {"<<", token.ShiftLeft}, {">>", token.ShiftRight},
		{"+=", token.PlusAssign}, {"-=", token.MinusAssign}, {"*=", token.StarAssign},
		{"/=", token.SlashAssign}, {"%=", token.PercentAssign}, {"&=", token.AmpAssign},
		{"|=", token.PipeAssign}, {"^=", token.CaretAssign},
		{"(", token.LParen}, {")", token.RParen}, {"[", token.LBracket}, {"]", token.RBracket},
		{"{", token.LBrace}, {"}", token.RBrace}, {",", token.Comma}, {";", token.Semicolon}, {":", token.Colon},
		{".", token.Dot}, {"@", token.At}, {"$", token.Dollar}, {"%", token.Percent},
		{"=", token.Assign}, {"+", token.Plus}, {"-", token.Minus}, {"*", token.Star},
		{"/", token.Slash}, {"<", token.Less}, {">", token.Greater}, {"&", token.Ampersand},
		{"|", token.Pipe}, {"^", token.Caret}, {"~", token.Tilde}, {"!", token.Bang},
	}
	remaining := string(l.source[l.offset:])
	for _, op := range operators {
		if strings.HasPrefix(remaining, op.text) {
			if op.typ == token.Comma && l.layoutDepth > 0 && l.depth == l.layoutDepth {
				l.endLambdaLayout(start)
			}
			if (op.typ == token.RParen || op.typ == token.RBracket || op.typ == token.RBrace) && l.layoutDepth > 0 && l.depth == l.layoutDepth {
				l.endLambdaLayout(start)
			}
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
				if l.layoutDepth > 0 && l.depth < l.layoutDepth {
					l.layoutDepth = 0
					l.layoutIndentDepth = 0
					l.lambdaHeaderDepth = 0
				}
			}
			l.emit(op.typ, op.text, start)
			if op.typ == token.Colon && l.lambdaHeaderDepth == l.depth {
				if l.blockFollows() {
					l.layoutDepth = l.depth
					l.layoutIndentDepth = len(l.indents)
				}
				l.lambdaHeaderDepth = 0
			}
			return nil
		}
	}
	r, _ := utf8.DecodeRune(l.source[l.offset:])
	return &Error{Position: start, Message: fmt.Sprintf("unexpected character %q", r)}
}

func (l *lexer) endLambdaLayout(position token.Position) {
	if len(l.tokens) > 0 && l.tokens[len(l.tokens)-1].Type != token.Newline && l.tokens[len(l.tokens)-1].Type != token.Dedent {
		l.emit(token.Newline, "", position)
	}
	for len(l.indents) > l.layoutIndentDepth {
		l.indents = l.indents[:len(l.indents)-1]
		l.emit(token.Dedent, "", position)
	}
	l.layoutDepth = 0
	l.layoutIndentDepth = 0
	l.lambdaHeaderDepth = 0
}

func (l *lexer) blockFollows() bool {
	for offset := l.offset; offset < len(l.source); offset++ {
		switch l.source[offset] {
		case ' ', '\t', '\r':
			continue
		case '\n', '#':
			return true
		default:
			return false
		}
	}
	return false
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
