// Package lexer tokenizes GDScript source, including its indentation structure.
package lexer

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cafecito-games/gdparser/internal/encoding"
	"github.com/cafecito-games/gdparser/token"
)

// Error describes invalid source encountered by the lexer.
type Error struct {
	Position token.Position
	Message  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Position, e.Message) }

// Lex returns all tokens in source, with indentation emitted as INDENT and
// DEDENT. Nothing drives the scan, so a line break is taken to carry no meaning
// wherever a bracket is open, which is what Godot's parser asks for everywhere
// except inside a lambda body and a type argument list. Use a Scanner, as the
// parser does, where those matter.
func Lex(source []byte) ([]token.Token, error) {
	l := newLexer(source)
	l.tracksBrackets = true
	// Source averages a handful of bytes per token, so this is close enough to
	// scan a file without growing the slice more than once or twice.
	tokens := make([]token.Token, 0, len(source)/4+8)
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
	// indentChar is the character the file indents with, chosen by the first
	// line that has any indentation, and zero until then.
	indentChar byte
	indents    []int
	// blocks holds the indentation stacks set aside for the lambda bodies that
	// are open, innermost last.
	blocks [][]int
	// pending holds tokens produced but not yet handed out, such as the rest of
	// a run of dedents, and pendingAt is how many of them have been. The buffer is
	// reused rather than resliced, so a file's scan allocates it once.
	pending   []token.Token
	pendingAt int
	// continued holds the comments written on the continuation lines of the
	// logical line being scanned. They sit in the middle of a statement, where
	// nothing can carry them, so they are held until the line ends.
	continued []token.Token
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
	// A UTF-8 byte order mark carries no syntax. Godot's style guide asks for
	// files without one, so it is skipped rather than rejected, which lets a
	// formatter rewrite such a file cleanly.
	return &lexer{
		source:  source,
		offset:  encoding.SkipByteOrderMark(source),
		line:    1,
		column:  1,
		atStart: true,
		indents: []int{0},
	}
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
	for l.pendingAt == len(l.pending) {
		l.pending, l.pendingAt = l.pending[:0], 0
		if err := l.step(); err != nil {
			return token.Token{}, err
		}
	}
	tok := l.pending[l.pendingAt]
	l.pendingAt++
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
			l.flushContinuationComments()
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
			l.flushContinuationComments()
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
		if err := l.scanString(start, true); err != nil {
			return err
		}
	case isIdentifierStart(c):
		l.scanIdentifier(start)
	case isDigit(c):
		if err := l.scanNumber(start); err != nil {
			return err
		}
	case c == '.' && isDigit(l.peekN(1)):
		// A float with its leading zero omitted, as in .5.
		if err := l.scanNumber(start); err != nil {
			return err
		}
	case c == '\'' || c == '"':
		if err := l.scanString(start, false); err != nil {
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
	columns, offset, indentChar, mixed := measureIndent(l.source, l.offset)
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
	// Indentation is written one way throughout a file, and one way within a
	// line. Godot settles both here, where a line of code begins.
	if mixed {
		return &Error{Position: p, Message: "a line's indentation mixes tabs and spaces"}
	}
	if indentChar != 0 {
		if l.indentChar == 0 {
			l.indentChar = indentChar
		} else if indentChar != l.indentChar {
			return &Error{
				Position: p,
				Message:  "indented with a " + indentCharName(indentChar) + " where the file indents with a " + indentCharName(l.indentChar),
			}
		}
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

// tabSize is the width Godot's tokenizer gives a tab when it measures
// indentation. It counts a flat tab_size columns for each one rather than
// advancing to the next multiple, so that a line mixing the two is measured the
// same wherever the tab falls.
const tabSize = 4

// measureIndent returns the indentation width of the line starting at offset, the
// offset of its first non-indentation byte, the character the line indents with,
// and whether the line mixed that character with the other one.
func measureIndent(source []byte, offset int) (columns, next int, indentChar byte, mixed bool) {
	for offset < len(source) {
		space := source[offset]
		switch space {
		case ' ':
			columns++
		case '\t':
			columns += tabSize
		default:
			return columns, offset, indentChar, mixed
		}
		if indentChar == 0 {
			indentChar = space
		}
		mixed = mixed || space != indentChar
		offset++
	}
	return columns, offset, indentChar, mixed
}

// indentCharName names an indentation character for an error message.
func indentCharName(c byte) string {
	if c == '\t' {
		return "tab"
	}
	return "space"
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
		columns, next, _, _ := measureIndent(source, offset)
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

// scanContinuation consumes a backslash that joins the next source line to this
// one, along with the whitespace and the comment lines that follow it. Godot
// skips both and discards the comments; they are held here instead and given back
// at the end of the logical line, which is the nearest place a statement can
// carry them.
func (l *lexer) scanContinuation() {
	l.advance() // backslash
	l.advance() // newline
	for !l.done() {
		for !l.done() && (l.peek() == ' ' || l.peek() == '\t' || l.peek() == '\r') {
			l.advance()
		}
		if l.peek() != '#' {
			break
		}
		start := l.position()
		l.scanComment(start)
		l.continued = append(l.continued, l.pending[len(l.pending)-1])
		l.pending = l.pending[:len(l.pending)-1]
		l.lastType = token.Newline
		if l.peek() == '\n' {
			l.advance()
		}
	}
	l.atStart = false
}

// flushContinuationComments gives back the comments held from this logical line's
// continuation lines, so that they come just before the line ends.
func (l *lexer) flushContinuationComments() {
	if len(l.continued) == 0 {
		return
	}
	l.pending = append(l.pending, l.continued...)
	l.lastType = l.continued[len(l.continued)-1].Type
	l.continued = l.continued[:0]
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

// scanNumber reads a numeric literal, holding to the rules Godot's number()
// states in modules/gdscript/gdscript_tokenizer.cpp: an underscore may separate
// digits but not stand beside another one, nor open a literal after its base
// prefix or its decimal point; a base prefix needs a digit of that base; a
// decimal point belongs to no hexadecimal or binary literal, and to a decimal one
// only once; and an exponent needs a value.
func (l *lexer) scanNumber(start token.Position) error {
	begin := l.offset
	digit := isDigit
	base := 10
	if l.peek() == '0' && (l.peekN(1) == 'x' || l.peekN(1) == 'X') {
		base, digit = 16, isHexDigit
	} else if l.peek() == '0' && (l.peekN(1) == 'b' || l.peekN(1) == 'B') {
		base, digit = 2, isBinaryDigit
	}
	if base != 10 {
		l.advance()
		prefix := l.advance()
		if l.peek() == '_' {
			return l.numberError("an underscore may not open a literal after \"0" + string(prefix) + "\"")
		}
		digits, err := l.scanDigits(digit)
		if err != nil {
			return err
		}
		if digits == 0 {
			return l.numberError("expected a digit after \"0" + string(prefix) + "\"")
		}
		if l.peek() == '.' && l.peekN(1) != '.' {
			if base == 16 {
				return l.numberError("a hexadecimal literal has no decimal point")
			}
			return l.numberError("a binary literal has no decimal point")
		}
		l.emit(token.Integer, string(l.source[begin:l.offset]), start)
		return nil
	}
	if _, err := l.scanDigits(digit); err != nil {
		return err
	}
	typ := token.Integer
	if l.peek() == '.' && l.peekN(1) != '.' {
		typ = token.Float
		l.advance()
		if l.peek() == '_' {
			return l.numberError("an underscore may not follow a decimal point")
		}
		if _, err := l.scanDigits(digit); err != nil {
			return err
		}
		if l.peek() == '.' && l.peekN(1) != '.' {
			return l.numberError("a literal has at most one decimal point")
		}
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		typ = token.Float
		l.advance()
		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}
		digits, err := l.scanDigits(digit)
		if err != nil {
			return err
		}
		if digits == 0 {
			return l.numberError("expected an exponent value after \"e\"")
		}
	}
	l.emit(typ, string(l.source[begin:l.offset]), start)
	return nil
}

// scanDigits consumes a run of digits of one base, along with the underscores
// that separate them, and returns how many digits it read.
func (l *lexer) scanDigits(digit func(byte) bool) (int, error) {
	count := 0
	previousWasUnderscore := false
	for !l.done() && (digit(l.peek()) || l.peek() == '_') {
		if l.peek() == '_' {
			if previousWasUnderscore {
				return 0, l.numberError("underscores may not be adjacent in a numeric literal")
			}
			previousWasUnderscore = true
		} else {
			previousWasUnderscore = false
			count++
		}
		l.advance()
	}
	return count, nil
}

// numberError reports a malformed numeric literal at the scan position.
func (l *lexer) numberError(message string) error {
	return &Error{Position: l.position(), Message: message}
}

// scanString reads a string literal. raw marks the r-prefixed form, in which a
// backslash is part of the text rather than opening an escape. Only the end of
// the source leaves a string unterminated: Godot's tokenizer takes a line break
// inside a quoted string as part of its text, whether the string is
// triple-quoted or not, and reports nothing until it runs out of source.
func (l *lexer) scanString(start token.Position, raw bool) error {
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
			if l.done() {
				break
			}
			if raw {
				// A backslash escapes only the quote and itself here, which is
				// what lets an r-string hold a quote at all.
				l.advance()
				continue
			}
			if err := l.scanEscape(); err != nil {
				return err
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

// escapes lists the characters a backslash may introduce in a string, as Godot's
// string() accepts them. A backslash before a line break escapes the break.
const escapes = "abfnrtv'\"\\\r\n"

// scanEscape reads the escape whose backslash has been consumed.
func (l *lexer) scanEscape() error {
	code := l.peek()
	if code == 'u' || code == 'U' {
		digits := 4
		if code == 'U' {
			digits = 6
		}
		l.advance()
		for range digits {
			if l.done() {
				return &Error{Position: l.position(), Message: "unterminated string literal"}
			}
			if !isHexDigit(l.peek()) {
				return &Error{Position: l.position(), Message: "expected a hexadecimal digit in the unicode escape"}
			}
			l.advance()
		}
		return nil
	}
	if !strings.ContainsRune(escapes, rune(code)) {
		return &Error{Position: l.position(), Message: fmt.Sprintf(`invalid escape "\%c" in string`, code)}
	}
	l.advance()
	return nil
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
		{"...", token.Ellipsis}, {"..", token.DotDot},
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
func isBinaryDigit(c byte) bool    { return c == '0' || c == '1' }
func isHexDigit(c byte) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
