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
	source  []byte
	offset  int
	line    int
	column  int
	atStart bool
	depth   int
	layouts []lambdaLayout
	// lambdaHeaderDepth is the bracket depth of a lambda header being scanned,
	// or zero when none is pending. A lambda header is only tracked inside
	// brackets, so a real pending header always sits at a depth above zero.
	lambdaHeaderDepth int
	indents           []int
	tokens            []token.Token
	// codeLineIndent, codeLineStart and codeLineFound cache the lookahead that
	// nextCodeIndent performs. The cache holds while the scan stays before
	// codeLineStart, which is the end of source when no code line follows.
	codeLineIndent int
	codeLineStart  int
	codeLineFound  bool
}

// lambdaLayout records one multiline lambda body that is still open. Lambdas
// nest, so each body keeps its own state: the bracket depth its header closed
// at, and the height of the indentation stack outside the body.
type lambdaLayout struct {
	depth       int
	indentDepth int
}

// layoutDepth returns the bracket depth of the innermost open lambda body, or
// zero when no body is open.
func (l *lexer) layoutDepth() int {
	if len(l.layouts) == 0 {
		return 0
	}
	return l.layouts[len(l.layouts)-1].depth
}

func (l *lexer) run() error {
	for !l.done() {
		if l.atStart && l.depth == l.layoutDepth() {
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
			if l.depth == l.layoutDepth() {
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
	columns, offset := measureIndent(l.source, l.offset)
	for l.offset < offset {
		l.advance()
	}
	// Blank lines do not affect the indentation stack.
	if l.done() || l.peek() == '\n' || l.peek() == '\r' {
		return nil
	}
	l.atStart = false
	if len(l.layouts) > 0 && l.depth == l.layoutDepth() && closesLambdaLayout(l.peek()) {
		// A comma or closing bracket on its own line ends the lambda body
		// rather than continuing it, so endLambdaLayout unwinds the
		// indentation stack when the token itself is scanned. Measuring this
		// line here would instead reject an indentation that is allowed to sit
		// outside the body's block.
		return nil
	}
	top := l.indents[len(l.indents)-1]
	p := token.Position{Offset: startOffset, Line: l.line, Column: 1}
	if l.peek() == '#' {
		// Godot's tokenizer discards comments before measuring indentation, so
		// a comment-only line neither opens nor closes a block on its own. Its
		// indentation only says which block it belongs to, and that is settled
		// by the next line of code: when that line is deeper, the comment
		// belongs to the block the code stays in, and nothing closes here.
		if next, ok := l.nextCodeIndent(); ok && next > columns {
			columns = next
		}
		if columns >= top {
			return nil
		}
		// A comment written at an outer level keeps that outer scope, so it
		// still closes the blocks the code below it has left. An indentation
		// matching no outer block is not an error on a comment-only line: the
		// comment keeps the scope of the nearest block above it.
		floor := l.indentFloor()
		for len(l.indents) > floor && columns < l.indents[len(l.indents)-1] {
			l.indents = l.indents[:len(l.indents)-1]
			l.emit(token.Dedent, "", p)
		}
		return nil
	}
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

// nextCodeIndent returns the indentation of the next line of code, caching the
// result until that line is reached. Every comment-only line in one run shares
// the same following line of code, so without the cache a run of them would
// rescan the source ahead of it once per line.
func (l *lexer) nextCodeIndent() (int, bool) {
	if l.offset < l.codeLineStart {
		return l.codeLineIndent, l.codeLineFound
	}
	columns, start, ok := nextCodeIndent(l.source, l.offset)
	l.codeLineIndent, l.codeLineStart, l.codeLineFound = columns, start, ok
	return columns, ok
}

// indentFloor returns the smallest indentation stack height that indentation
// alone may unwind to. Inside a multiline lambda body only the comma or closing
// bracket that ends the body may leave it, so the body's own level is a floor.
func (l *lexer) indentFloor() int {
	if len(l.layouts) == 0 {
		return 1
	}
	return l.layouts[len(l.layouts)-1].indentDepth + 1
}

// measureIndent returns the indentation width of the line starting at offset,
// and the offset of its first non-indentation byte. A tab advances to the next
// multiple of four columns, as Godot's tokenizer measures it.
func measureIndent(source []byte, offset int) (columns, next int) {
	for offset < len(source) {
		switch source[offset] {
		case ' ':
			columns++
		case '\t':
			columns += 4 - columns%4
		default:
			return columns, offset
		}
		offset++
	}
	return columns, offset
}

// nextCodeIndent returns the indentation width of the next line that holds
// code and the offset that line's code starts at, skipping the rest of the line
// at offset along with any blank and comment-only lines. It reports whether
// such a line exists; when none does, the returned offset is the end of source.
func nextCodeIndent(source []byte, offset int) (columns, start int, ok bool) {
	for {
		for offset < len(source) && source[offset] != '\n' {
			offset++
		}
		if offset >= len(source) {
			return 0, len(source), false
		}
		offset++
		columns, next := measureIndent(source, offset)
		if next >= len(source) {
			return 0, len(source), false
		}
		offset = next
		if source[offset] == '\n' || source[offset] == '\r' || source[offset] == '#' {
			continue
		}
		return columns, offset, true
	}
}

// closesLambdaLayout reports whether c, as the first character of a line,
// terminates an enclosing multiline lambda body.
func closesLambdaLayout(c byte) bool {
	return c == ',' || c == ')' || c == ']' || c == '}'
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
	if typ == token.Func && l.depth > l.layoutDepth() {
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
			if op.typ == token.Comma && len(l.layouts) > 0 && l.depth == l.layoutDepth() {
				l.endLambdaLayout(start)
			}
			if (op.typ == token.RParen || op.typ == token.RBracket || op.typ == token.RBrace) && len(l.layouts) > 0 && l.depth == l.layoutDepth() {
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
				for len(l.layouts) > 0 && l.depth < l.layoutDepth() {
					l.layouts = l.layouts[:len(l.layouts)-1]
					l.lambdaHeaderDepth = 0
				}
			}
			l.emit(op.typ, op.text, start)
			if op.typ == token.Colon && l.lambdaHeaderDepth > 0 && l.lambdaHeaderDepth == l.depth {
				if l.blockFollows() {
					l.layouts = append(l.layouts, lambdaLayout{depth: l.depth, indentDepth: len(l.indents)})
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
	layout := l.layouts[len(l.layouts)-1]
	for len(l.indents) > layout.indentDepth {
		l.indents = l.indents[:len(l.indents)-1]
		l.emit(token.Dedent, "", position)
	}
	l.layouts = l.layouts[:len(l.layouts)-1]
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
