// Package lexer tokenizes Godot 4 shading-language source.
package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/cafecito-games/gdparser/internal/encoding"
	"github.com/cafecito-games/gdparser/shader/token"
)

// Lex tokenizes source. Comments and preprocessor lines are retained. A leading
// byte order mark is skipped, since it says how the file is encoded rather than
// anything the shading language can read.
func Lex(filename string, source []byte) ([]token.Token, error) {
	l := lexer{
		filename:  filename,
		source:    source,
		offset:    encoding.SkipByteOrderMark(source),
		line:      1,
		column:    1,
		lineStart: true,
	}
	return l.lex()
}

type lexer struct {
	filename  string
	source    []byte
	offset    int
	line      int
	column    int
	lineStart bool
	tokens    []token.Token
}

func (l *lexer) pos() token.Position {
	return token.Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func (l *lexer) advance() rune {
	r, size := utf8.DecodeRune(l.source[l.offset:])
	l.offset += size
	if r == '\n' {
		l.line++
		l.column = 1
		l.lineStart = true
	} else {
		l.column++
		if r != ' ' && r != '\t' && r != '\r' {
			l.lineStart = false
		}
	}
	return r
}

func (l *lexer) peek(n int) byte {
	if l.offset+n >= len(l.source) {
		return 0
	}
	return l.source[l.offset+n]
}

func (l *lexer) emit(kind token.Kind, start token.Position) {
	l.tokens = append(l.tokens, token.Token{Kind: kind, Text: string(l.source[start.Offset:l.offset]), Span: token.Span{Start: start, End: l.pos()}})
}

func (l *lexer) fail(pos token.Position, format string, args ...any) error {
	return &token.Error{Filename: l.filename, Position: pos, Message: fmt.Sprintf(format, args...)}
}

// scanToLineEnd advances to the end of the line, stopping before the carriage
// returns that run to it. A carriage return there ends the line rather than
// belonging to the text, whether the line ends with a line feed or with the file;
// keeping one would carry it into output whose other lines end with a line feed
// alone.
func (l *lexer) scanToLineEnd() {
	for l.offset < len(l.source) && l.peek(0) != '\n' {
		if l.peek(0) == '\r' {
			run := 0
			for l.peek(run) == '\r' {
				run++
			}
			if l.offset+run >= len(l.source) || l.peek(run) == '\n' {
				return
			}
			for range run {
				l.advance()
			}
			continue
		}
		l.advance()
	}
}

func (l *lexer) lex() ([]token.Token, error) {
	for l.offset < len(l.source) {
		b := l.peek(0)
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			l.advance()
			continue
		}
		start := l.pos()
		if b == '#' && l.lineStart {
			for {
				l.scanToLineEnd()
				if l.offset >= len(l.source) {
					break
				}
				i := l.offset - 1
				for i >= start.Offset && (l.source[i] == ' ' || l.source[i] == '\t' || l.source[i] == '\r') {
					i--
				}
				if i < start.Offset || l.source[i] != '\\' {
					break
				}
				for l.offset < len(l.source) && l.peek(0) != '\n' {
					l.advance()
				}
				l.advance()
			}
			l.emit(token.Directive, start)
			continue
		}
		if b == '/' && l.peek(1) == '/' {
			l.advance()
			l.advance()
			l.scanToLineEnd()
			l.emit(token.Comment, start)
			continue
		}
		if b == '/' && l.peek(1) == '*' {
			l.advance()
			l.advance()
			closed := false
			for l.offset < len(l.source) {
				if l.peek(0) == '*' && l.peek(1) == '/' {
					l.advance()
					l.advance()
					closed = true
					break
				}
				l.advance()
			}
			if !closed {
				return nil, l.fail(start, "unterminated block comment")
			}
			l.emit(token.Comment, start)
			continue
		}
		if isIdentStart(b) {
			l.advance()
			for isIdentContinue(l.peek(0)) {
				l.advance()
			}
			l.emit(token.Identifier, start)
			continue
		}
		if isDigit(b) || (b == '.' && isDigit(l.peek(1))) {
			l.scanNumber()
			l.emit(token.Number, start)
			continue
		}
		if b == '"' || b == '\'' {
			quote := b
			l.advance()
			closed := false
			for l.offset < len(l.source) {
				if l.peek(0) == '\\' {
					l.advance()
					if l.offset < len(l.source) {
						l.advance()
					}
					continue
				}
				if l.peek(0) == quote {
					l.advance()
					closed = true
					break
				}
				if l.peek(0) == '\n' {
					return nil, l.fail(start, "newline in string literal")
				}
				l.advance()
			}
			if !closed {
				return nil, l.fail(start, "unterminated string literal")
			}
			l.emit(token.String, start)
			continue
		}
		matched := ""
		for _, op := range []string{"<<=", ">>=", "++", "--", "==", "!=", "<=", ">=", "&&", "||", "^^", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>"} {
			if l.has(op) {
				matched = op
				break
			}
		}
		if matched != "" {
			for range len(matched) {
				l.advance()
			}
			l.emit(token.Symbol, start)
			continue
		}
		if contains("{}()[];,.?:+-*/%<>=!~&|^", b) {
			l.advance()
			l.emit(token.Symbol, start)
			continue
		}
		_, size := utf8.DecodeRune(l.source[l.offset:])
		return nil, l.fail(start, "unexpected character %q", string(l.source[l.offset:l.offset+size]))
	}
	p := l.pos()
	l.tokens = append(l.tokens, token.Token{Kind: token.EOF, Span: token.Span{Start: p, End: p}})
	return l.tokens, nil
}

func (l *lexer) has(s string) bool {
	if l.offset+len(s) > len(l.source) {
		return false
	}
	return string(l.source[l.offset:l.offset+len(s)]) == s
}

func (l *lexer) scanNumber() {
	if l.peek(0) == '0' && (l.peek(1) == 'x' || l.peek(1) == 'X') {
		l.advance()
		l.advance()
		for isHex(l.peek(0)) || l.peek(0) == '_' {
			l.advance()
		}
		return
	}
	if l.peek(0) == '.' {
		l.advance()
	}
	for isDigit(l.peek(0)) || l.peek(0) == '_' {
		l.advance()
	}
	if l.peek(0) == '.' {
		l.advance()
		for isDigit(l.peek(0)) || l.peek(0) == '_' {
			l.advance()
		}
	}
	if l.peek(0) == 'e' || l.peek(0) == 'E' {
		l.advance()
		if l.peek(0) == '+' || l.peek(0) == '-' {
			l.advance()
		}
		for isDigit(l.peek(0)) || l.peek(0) == '_' {
			l.advance()
		}
	}
	for l.peek(0) == 'u' || l.peek(0) == 'U' || l.peek(0) == 'f' || l.peek(0) == 'F' {
		l.advance()
	}
}

func isIdentStart(b byte) bool    { return b == '_' || b >= utf8.RuneSelf || unicode.IsLetter(rune(b)) }
func isIdentContinue(b byte) bool { return isIdentStart(b) || isDigit(b) }
func isDigit(b byte) bool         { return b >= '0' && b <= '9' }
func isHex(b byte) bool           { return isDigit(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F' }
func contains(s string, b byte) bool {
	for i := range len(s) {
		if s[i] == b {
			return true
		}
	}
	return false
}
