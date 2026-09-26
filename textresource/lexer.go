package textresource

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cafecito-games/gdparser/token"
)

type tokenKind uint8

const (
	tEOF tokenKind = iota
	tNewline
	tIdentifier
	tInteger
	tFloat
	tString
	tStringName
	tNodePath
	tComment
	tLBracket
	tRBracket
	tLBrace
	tRBrace
	tLParen
	tRParen
	tComma
	tColon
	tEqual
	tSlash
)

type lexToken struct {
	kind  tokenKind
	text  string
	value string
	span  token.Span
}

// LexError reports malformed input at a source position.
type LexError struct {
	Position token.Position
	Message  string
}

func (e *LexError) Error() string { return fmt.Sprintf("%s: %s", e.Position, e.Message) }

type scanner struct {
	source               []byte
	offset, line, column int
}

func lex(source []byte) ([]lexToken, error) {
	s := scanner{source: source, line: 1, column: 1}
	var result []lexToken
	for {
		s.skipHorizontal()
		start := s.position()
		if s.offset >= len(s.source) {
			result = append(result, lexToken{kind: tEOF, span: token.Span{Start: start, End: start}})
			return result, nil
		}
		b := s.source[s.offset]
		switch b {
		case '\n', '\r':
			s.newline()
			result = append(result, s.tok(tNewline, start, "", ""))
		case ';':
			s.advanceByte()
			text := s.readToNewline()
			result = append(result, s.tok(tComment, start, text, text))
		case '#':
			// Semicolon is the native text-resource comment marker. Accept #
			// comments as a convenience unless the token is a hexadecimal Color.
			if s.isHexColor() {
				text := s.readBare()
				result = append(result, s.tok(tIdentifier, start, text, text))
			} else {
				s.advanceByte()
				text := s.readToNewline()
				result = append(result, s.tok(tComment, start, text, text))
			}
		case '[':
			s.advanceByte()
			result = append(result, s.tok(tLBracket, start, "[", ""))
		case ']':
			s.advanceByte()
			result = append(result, s.tok(tRBracket, start, "]", ""))
		case '{':
			s.advanceByte()
			result = append(result, s.tok(tLBrace, start, "{", ""))
		case '}':
			s.advanceByte()
			result = append(result, s.tok(tRBrace, start, "}", ""))
		case '(':
			s.advanceByte()
			result = append(result, s.tok(tLParen, start, "(", ""))
		case ')':
			s.advanceByte()
			result = append(result, s.tok(tRParen, start, ")", ""))
		case ',':
			s.advanceByte()
			result = append(result, s.tok(tComma, start, ",", ""))
		case ':':
			s.advanceByte()
			result = append(result, s.tok(tColon, start, ":", ""))
		case '=':
			s.advanceByte()
			result = append(result, s.tok(tEqual, start, "=", ""))
		case '/':
			s.advanceByte()
			result = append(result, s.tok(tSlash, start, "/", ""))
		case '"':
			tok, err := s.scanString(start, tString, 0)
			if err != nil {
				return nil, err
			}
			result = append(result, tok)
		case '&', '^':
			prefix := b
			s.advanceByte()
			if s.offset >= len(s.source) || s.source[s.offset] != '"' {
				return nil, &LexError{start, fmt.Sprintf("expected quoted string after %q", prefix)}
			}
			kind := tStringName
			if prefix == '^' {
				kind = tNodePath
			}
			tok, err := s.scanString(start, kind, prefix)
			if err != nil {
				return nil, err
			}
			result = append(result, tok)
		default:
			if isNumberStart(s.source[s.offset:]) {
				tok, err := s.scanNumber(start)
				if err != nil {
					return nil, err
				}
				result = append(result, tok)
				continue
			}
			if (b == '-' || b == '+') && s.offset+1 < len(s.source) && isBareStart(s.source[s.offset+1]) {
				s.advanceByte()
				text := string(b) + s.readBare()
				result = append(result, s.tok(tIdentifier, start, text, text))
				continue
			}
			if isBareStart(b) {
				text := s.readBare()
				result = append(result, s.tok(tIdentifier, start, text, text))
				continue
			}
			return nil, &LexError{start, fmt.Sprintf("unexpected character %q", b)}
		}
	}
}
func (s *scanner) position() token.Position {
	return token.Position{Offset: s.offset, Line: s.line, Column: s.column}
}
func (s *scanner) tok(k tokenKind, start token.Position, text, value string) lexToken {
	return lexToken{kind: k, text: text, value: value, span: token.Span{Start: start, End: s.position()}}
}
func (s *scanner) advanceByte() { s.offset++; s.column++ }
func (s *scanner) newline() {
	if s.source[s.offset] == '\r' {
		s.offset++
		if s.offset < len(s.source) && s.source[s.offset] == '\n' {
			s.offset++
		}
	} else {
		s.offset++
	}
	s.line++
	s.column = 1
}
func (s *scanner) skipHorizontal() {
	for s.offset < len(s.source) {
		switch s.source[s.offset] {
		case ' ', '\t', '\f':
			s.advanceByte()
		default:
			return
		}
	}
}
func (s *scanner) readToNewline() string {
	begin := s.offset
	for s.offset < len(s.source) && s.source[s.offset] != '\n' && s.source[s.offset] != '\r' {
		s.advanceRune()
	}
	return string(s.source[begin:s.offset])
}
func (s *scanner) advanceRune() {
	_, n := utf8.DecodeRune(s.source[s.offset:])
	if n < 1 {
		n = 1
	}
	s.offset += n
	s.column++
}
func isBareStart(b byte) bool {
	return b == '_' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= 0x80
}
func isBareContinue(r rune) bool {
	return r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
func (s *scanner) readBare() string {
	begin := s.offset
	if s.source[s.offset] == '#' {
		s.advanceByte()
		for s.offset < len(s.source) && isHex(s.source[s.offset]) {
			s.advanceByte()
		}
		return string(s.source[begin:s.offset])
	}
	for s.offset < len(s.source) {
		r, n := utf8.DecodeRune(s.source[s.offset:])
		if !isBareContinue(r) {
			break
		}
		s.offset += n
		s.column++
	}
	return string(s.source[begin:s.offset])
}
func isHex(b byte) bool { return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F' }
func (s *scanner) isHexColor() bool {
	i := s.offset + 1
	for i < len(s.source) && isHex(s.source[i]) {
		i++
	}
	n := i - (s.offset + 1)
	return (n == 3 || n == 4 || n == 6 || n == 8) && (i == len(s.source) || s.source[i] <= 32 || s.source[i] == ',' || s.source[i] == ')' || s.source[i] == ']' || s.source[i] == '}')
}
func isNumberStart(src []byte) bool {
	if len(src) == 0 {
		return false
	}
	if src[0] >= '0' && src[0] <= '9' {
		return true
	}
	return (src[0] == '-' || src[0] == '+') && len(src) > 1 && src[1] >= '0' && src[1] <= '9'
}
func (s *scanner) scanNumber(start token.Position) (lexToken, error) {
	begin := s.offset
	if s.source[s.offset] == '-' || s.source[s.offset] == '+' {
		s.advanceByte()
	}
	for s.offset < len(s.source) && s.source[s.offset] >= '0' && s.source[s.offset] <= '9' {
		s.advanceByte()
	}
	kind := tInteger
	if s.offset < len(s.source) && s.source[s.offset] == '.' {
		kind = tFloat
		s.advanceByte()
		for s.offset < len(s.source) && s.source[s.offset] >= '0' && s.source[s.offset] <= '9' {
			s.advanceByte()
		}
	}
	if s.offset < len(s.source) && (s.source[s.offset] == 'e' || s.source[s.offset] == 'E') {
		kind = tFloat
		s.advanceByte()
		if s.offset < len(s.source) && (s.source[s.offset] == '+' || s.source[s.offset] == '-') {
			s.advanceByte()
		}
		exp := s.offset
		for s.offset < len(s.source) && s.source[s.offset] >= '0' && s.source[s.offset] <= '9' {
			s.advanceByte()
		}
		if exp == s.offset {
			return lexToken{}, &LexError{start, "malformed numeric exponent"}
		}
	}
	if kind == tInteger && s.offset < len(s.source) {
		if s.source[s.offset] == 'U' {
			s.advanceByte()
			if s.offset < len(s.source) && s.source[s.offset] == 'L' {
				s.advanceByte()
			}
		} else if s.source[s.offset] == 'L' {
			s.advanceByte()
		}
	}
	text := string(s.source[begin:s.offset])
	if kind == tInteger {
		digits := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(text, "UL"), "U"), "L")
		if strings.HasSuffix(text, "U") || strings.HasSuffix(text, "UL") {
			if strings.HasPrefix(digits, "-") {
				return lexToken{}, &LexError{start, "unsigned integer cannot be negative"}
			}
			if _, err := strconv.ParseUint(strings.TrimPrefix(digits, "+"), 10, 64); err != nil {
				return lexToken{}, &LexError{start, "integer is out of range"}
			}
		} else if _, err := strconv.ParseInt(digits, 10, 64); err != nil {
			return lexToken{}, &LexError{start, "integer is out of range"}
		}
	} else if _, err := strconv.ParseFloat(text, 64); err != nil {
		return lexToken{}, &LexError{start, "invalid floating-point value"}
	}
	return s.tok(kind, start, text, text), nil
}
func (s *scanner) scanString(start token.Position, kind tokenKind, prefix byte) (lexToken, error) {
	begin := s.offset
	s.advanceByte()
	var value []rune
	for s.offset < len(s.source) {
		b := s.source[s.offset]
		if b == '"' {
			s.advanceByte()
			textBegin := begin
			if prefix != 0 {
				textBegin = start.Offset
			}
			return s.tok(kind, start, string(s.source[textBegin:s.offset]), string(value)), nil
		}
		if b == '\n' || b == '\r' {
			s.newline()
			value = append(value, '\n')
			continue
		}
		if b != '\\' {
			r, n := utf8.DecodeRune(s.source[s.offset:])
			if r == utf8.RuneError && n == 1 {
				return lexToken{}, &LexError{s.position(), "invalid UTF-8 in string"}
			}
			value = append(value, r)
			s.offset += n
			s.column++
			continue
		}
		escapePos := s.position()
		s.advanceByte()
		if s.offset >= len(s.source) {
			return lexToken{}, &LexError{escapePos, "unterminated string escape"}
		}
		esc := s.source[s.offset]
		s.advanceByte()
		switch esc {
		case 'b':
			value = append(value, '\b')
		case 't':
			value = append(value, '\t')
		case 'n':
			value = append(value, '\n')
		case 'f':
			value = append(value, '\f')
		case 'r':
			value = append(value, '\r')
		case 'u', 'U':
			digits := 4
			if esc == 'U' {
				digits = 6
			}
			if s.offset+digits > len(s.source) {
				return lexToken{}, &LexError{escapePos, "short Unicode escape"}
			}
			v, err := strconv.ParseUint(string(s.source[s.offset:s.offset+digits]), 16, 32)
			if err != nil {
				return lexToken{}, &LexError{escapePos, "malformed Unicode escape"}
			}
			for range digits {
				s.advanceByte()
			}
			value = append(value, rune(v))
		default:
			value = append(value, rune(esc))
		}
	}
	return lexToken{}, &LexError{start, "unterminated string"}
}
