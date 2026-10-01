// Package lexer tokenizes GDScript source, including its indentation structure.
package lexer

import (
	"bytes"
	"fmt"
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

// Lex returns all tokens in source, with indentation emitted as INDENT and
// DEDENT. Nothing drives the scan, so a line break is taken to carry no meaning
// wherever a bracket is open, which is what Godot's parser asks for everywhere
// except inside a lambda body and a type argument list. Use a Scanner, as the
// parser does, where those matter.
func Lex(source []byte) ([]token.Token, error) {
	l := newLexer(source)
	l.tracksBrackets = true
	var tokens []token.Token
	for !l.finished {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

type Scanner struct {
	lex *lexer
	// tokens holds every token handed out so far, so the parser may look back
	// and ahead by index.
	tokens []token.Token
	err    error
}

// NewScanner returns a Scanner reading source. Line breaks are significant until
// the caller says otherwise, which is the state Godot's parser starts in.
func NewScanner(source []byte) *Scanner {
	return &Scanner{lex: newLexer(source)}
}

// At returns the token at index, reading more source when it has to. Asking past
// the end of the source returns its EOF token, so a caller may look ahead
// without bounds checking.
func (s *Scanner) At(index int) (token.Token, error) {
	for len(s.tokens) <= index && s.err == nil && !s.lex.finished {
		tok, err := s.lex.next()
		if err != nil {
			s.err = err
			break
		}
		s.tokens = append(s.tokens, tok)
	}
	if index < len(s.tokens) {
		return s.tokens[index], nil
	}
	if s.err != nil {
		return token.Token{}, s.err
	}
	return s.tokens[len(s.tokens)-1], nil
}

// SetMultilineMode says whether a line break between the tokens still to be read
// carries meaning. Turning line breaks off also drains the ones already read at
// index, because the construct they would have ended is one the caller has just
// decided may span lines.
func (s *Scanner) SetMultilineMode(index int, multiline bool) {
	s.lex.multiline = multiline
	if multiline {
		s.DropLayout(index)
	}
}

// DropLayout discards the run of line break and indentation tokens read at index,
// which say nothing a construct spanning lines can use.
func (s *Scanner) DropLayout(index int) {
	end := index
	for end < len(s.tokens) && isLayout(s.tokens[end].Type) {
		end++
	}
	if end > index {
		s.tokens = append(s.tokens[:index], s.tokens[end:]...)
	}
}

// isLayout reports whether typ describes the shape of the source rather than
// anything written in it.
func isLayout(typ token.Type) bool {
	return typ == token.Newline || typ == token.Indent || typ == token.Dedent
}

// PushExpressionIndentedBlock sets aside the indentation built up so far, so that
// a block written inside an expression, which is to say a lambda body, may start
// its own.
func (s *Scanner) PushExpressionIndentedBlock() {
	s.lex.blocks = append(s.lex.blocks, s.lex.indents)
}

// PopExpressionIndentedBlock restores the indentation that the matching
// PushExpressionIndentedBlock set aside.
func (s *Scanner) PopExpressionIndentedBlock() {
	if len(s.lex.blocks) == 0 {
		return
	}
	s.lex.indents = s.lex.blocks[len(s.lex.blocks)-1]
	s.lex.blocks = s.lex.blocks[:len(s.lex.blocks)-1]
}

type lexer struct {
	source  []byte
	offset  int
	line    int
	column  int
	atStart bool
	// multiline says that a line break between tokens carries no meaning, so no
	// NEWLINE, INDENT or DEDENT is produced while it holds. The parser sets it;
	// Lex derives it from depth instead.
	multiline bool
	// tracksBrackets makes the lexer set multiline from the bracket depth on its
	// own, which is what Lex does in the absence of a parser.
	tracksBrackets bool
	depth          int
	indents        []int
	// blocks holds the indentation stacks set aside for the lambda bodies that
	// are open, innermost last.
	blocks [][]int
	// pending holds tokens produced but not yet handed out, such as the rest of
	// a run of dedents.
	pending []token.Token
	// lastType is the type of the token produced most recently, which is what
	// says whether the source already ended with a line break.
	lastType token.Type
	// ended records that the tokens closing the source have been produced, and
	// finished that its EOF has been handed out.
	ended    bool
	finished bool
	// codeLineIndent, codeLineStart and codeLineFound cache the lookahead that
	// nextCodeIndent performs. The cache holds while the scan stays before
	// codeLineStart, which is the end of source when no code line follows.
	codeLineIndent int
	codeLineStart  int
	codeLineFound  bool
}

func newLexer(source []byte) *lexer {
	l := &lexer{source: source, line: 1, column: 1, atStart: true, indents: []int{0}}
	// A UTF-8 byte order mark carries no syntax. Godot's style guide asks for
	// files without one, so it is skipped rather than rejected, which lets a
	// formatter rewrite such a file cleanly.
	if bytes.HasPrefix(source, byteOrderMark) {
		l.offset = len(byteOrderMark)
	}
	return l
}

// setIndents replaces the indentation stack without writing through a slice an
// earlier state still refers to.
func (l *lexer) setIndents(indents []int) {
	l.indents = append([]int(nil), indents...)
}

func (l *lexer) pushIndent(columns int) {
	l.setIndents(append(l.indents, columns))
}

func (l *lexer) popIndent() {
	l.setIndents(l.indents[:len(l.indents)-1])
}

// next returns the next token, reading source until one is produced.
func (l *lexer) next() (token.Token, error) {
	for len(l.pending) == 0 {
		if err := l.step(); err != nil {
			return token.Token{}, err
		}
	}
	tok := l.pending[0]
	l.pending = l.pending[1:]
	if tok.Type == token.EOF {
		l.finished = true
	}
	return tok, nil
}

// step reads the next piece of source, queueing whatever tokens it produces. It
// queues nothing when the source only held whitespace, so next calls it again.
func (l *lexer) step() error {
	if l.done() {
		if !l.ended {
			l.ended = true
			position := l.position()
			if l.lastType != token.Newline {
				l.emit(token.Newline, "", position)
			}
			for len(l.indents) > 1 {
				l.popIndent()
				l.emit(token.Dedent, "", position)
			}
			l.emit(token.EOF, "", position)
		}
		return nil
	}
	if l.atStart && !l.multiline {
		// Indentation is measured before the line's first token, and the two are
		// read in one step so that a blank or comment-only line cannot leave the
		// scan where it started.
		if err := l.scanIndent(); err != nil {
			return err
		}
		if l.done() {
			return nil
		}
	}

	start := l.position()
	c := l.peek()
	switch {
	case c == ' ' || c == '\t' || c == '\r':
		l.advance()
	case c == '\n':
		l.advance()
		if !l.multiline {
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
		for len(l.indents) > 1 && columns < l.indents[len(l.indents)-1] {
			l.popIndent()
			l.emit(token.Dedent, "", p)
		}
		return nil
	}
	if columns > top {
		l.pushIndent(columns)
		l.emit(token.Indent, "", p)
		return nil
	}
	if columns < top {
		for len(l.indents) > 1 && columns < l.indents[len(l.indents)-1] {
			l.popIndent()
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

func (l *lexer) scanComment(start token.Position) {
	l.atStart = false
	begin := l.offset
	for !l.done() && l.peek() != '\n' {
		if l.peek() == '\r' {
			// Carriage returns that run to the end of the line end the
			// comment rather than belonging to its text, whether the line ends
			// with a line feed or with the file. Emitting them inside the text
			// would carry them into output whose other lines end with a line
			// feed alone. The whole run is measured at once so that a comment
			// holding carriage returns is still scanned in one pass.
			run := 0
			for l.peekN(run) == '\r' {
				run++
			}
			if l.offset+run == len(l.source) || l.peekN(run) == '\n' {
				break
			}
			for range run {
				l.advance()
			}
			continue
		}
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
		{"**=", token.DoubleStarAssign},
		{"&&", token.And}, {"||", token.Or},
		{"...", token.Ellipsis}, {"..", token.Range},
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
	remaining := l.source[l.offset:]
	for _, op := range operators {
		if bytes.HasPrefix(remaining, []byte(op.text)) {
			for range len(op.text) {
				l.advance()
			}
			if l.tracksBrackets {
				switch op.typ {
				case token.LParen, token.LBracket, token.LBrace:
					l.depth++
				case token.RParen, token.RBracket, token.RBrace:
					if l.depth > 0 {
						l.depth--
					}
				}
				l.multiline = l.depth > 0
			}
			l.emit(op.typ, op.text, start)
			return nil
		}
	}
	r, _ := utf8.DecodeRune(l.source[l.offset:])
	return &Error{Position: start, Message: fmt.Sprintf("unexpected character %q", r)}
}

func (l *lexer) emit(typ token.Type, lexeme string, start token.Position) {
	l.pending = append(l.pending, token.Token{Type: typ, Lexeme: lexeme, Span: token.Span{Start: start, End: l.position()}})
	l.lastType = typ
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
