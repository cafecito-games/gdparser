package textresource

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cafecito-games/gdparser/token"
)

// Error is a positioned text-resource syntax error.
type Error struct {
	Filename string
	Position token.Position
	Message  string
	Found    string
}

func (e *Error) Error() string {
	location := e.Position.String()
	if e.Filename != "" {
		location = e.Filename + ":" + location
	}
	if e.Found != "" {
		return fmt.Sprintf("%s: %s (found %s)", location, e.Message, e.Found)
	}
	return fmt.Sprintf("%s: %s", location, e.Message)
}

// Parse parses source without attaching a filename to diagnostics.
func Parse(source []byte) (*Document, error) { return ParseFile("", source) }

// ParseString parses string source without attaching a filename.
func ParseString(source string) (*Document, error) { return Parse([]byte(source)) }

// ParseFile parses source and includes filename in diagnostics and the tree.
func ParseFile(filename string, source []byte) (*Document, error) {
	tokens, err := lex(source)
	if err != nil {
		if filename != "" {
			return nil, fmt.Errorf("%s:%w", filename, err)
		}
		return nil, err
	}
	p := parser{filename: filename, source: source, tokens: tokens}
	doc, err := p.parseDocument()
	if err != nil {
		return nil, err
	}
	return doc, nil
}

type parser struct {
	filename string
	source   []byte
	tokens   []lexToken
	current  int
}

func (p *parser) parseDocument() (*Document, error) {
	doc := &Document{Name: p.filename}
	for !p.at(tEOF) {
		if p.match(tNewline) {
			continue
		}
		if p.at(tComment) {
			c := p.comment(p.advance())
			doc.Items = append(doc.Items, c)
			if err := p.lineEnd(); err != nil {
				return nil, err
			}
			continue
		}
		if !p.at(tLBracket) {
			return nil, p.error(p.peek(), "expected a section header")
		}
		section, err := p.parseSection()
		if err != nil {
			return nil, err
		}
		doc.Items = append(doc.Items, section)
	}
	if len(p.tokens) > 0 {
		doc.SourceSpan = token.Span{Start: p.tokens[0].span.Start, End: p.peek().span.End}
	}
	return doc, nil
}
func (p *parser) parseSection() (*Section, error) {
	start := p.advance()
	name, err := p.expect(tIdentifier, "expected section type after '['")
	if err != nil {
		return nil, err
	}
	section := &Section{Type: name.text}
	for !p.at(tRBracket) {
		if p.at(tEOF) || p.at(tNewline) || p.at(tComment) {
			return nil, p.error(p.peek(), "expected ']' after section header")
		}
		attrName, err := p.expect(tIdentifier, "expected attribute name")
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(tEqual, "expected '=' after attribute name"); err != nil {
			return nil, err
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		section.Attributes = append(section.Attributes, &Attribute{Base: Base{SourceSpan: token.Span{Start: attrName.span.Start, End: value.Span().End}}, Name: attrName.text, Value: value})
	}
	close := p.advance()
	section.SourceSpan = token.Span{Start: start.span.Start, End: close.span.End}
	if p.at(tComment) {
		section.HeaderComment = p.comment(p.advance())
		section.SourceSpan.End = section.HeaderComment.Span().End
	}
	if err := p.lineEnd(); err != nil {
		return nil, err
	}
	for !p.at(tEOF) && !p.at(tLBracket) {
		if p.match(tNewline) {
			continue
		}
		if p.at(tComment) {
			c := p.comment(p.advance())
			section.Body = append(section.Body, c)
			section.SourceSpan.End = c.Span().End
			if err := p.lineEnd(); err != nil {
				return nil, err
			}
			continue
		}
		assignment, err := p.parseAssignment()
		if err != nil {
			return nil, err
		}
		section.Body = append(section.Body, assignment)
		section.SourceSpan.End = assignment.Span().End
	}
	return section, nil
}
func (p *parser) parseAssignment() (*Assignment, error) {
	startIndex := p.current
	start := p.peek()
	for !p.at(tEqual) && !p.at(tNewline) && !p.at(tComment) && !p.at(tEOF) {
		p.advance()
	}
	if startIndex == p.current {
		return nil, p.error(p.peek(), "expected property name")
	}
	if !p.at(tEqual) {
		return nil, p.error(p.peek(), "expected '=' after property name")
	}
	equal := p.advance()
	raw := strings.TrimSpace(string(p.source[start.span.Start.Offset:equal.span.Start.Offset]))
	if raw == "" {
		return nil, p.error(start, "expected property name")
	}
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	result := &Assignment{Property: raw, Value: value}
	end := value.Span().End
	if p.at(tComment) {
		result.TrailingComment = p.comment(p.advance())
		end = result.TrailingComment.Span().End
	}
	result.SourceSpan = token.Span{Start: start.span.Start, End: end}
	if err := p.lineEnd(); err != nil {
		return nil, err
	}
	return result, nil
}
func (p *parser) parseValue() (Value, error) {
	tok := p.peek()
	switch tok.kind {
	case tIdentifier:
		p.advance()
		switch tok.text {
		case "null":
			return &NullValue{Base{tok.span}}, nil
		case "true":
			return &BoolValue{Base: Base{tok.span}, Value: true}, nil
		case "false":
			return &BoolValue{Base: Base{tok.span}, Value: false}, nil
		case "nan", "+nan", "-nan":
			return &FloatValue{Base: Base{tok.span}, Value: math.NaN()}, nil
		case "inf", "+inf":
			return &FloatValue{Base: Base{tok.span}, Value: math.Inf(1)}, nil
		case "-inf":
			return &FloatValue{Base: Base{tok.span}, Value: math.Inf(-1)}, nil
		}
		if (tok.text == "Array" || tok.text == "Dictionary") && p.at(tLBracket) {
			return p.parseTyped(tok)
		}
		if p.at(tLParen) {
			return p.parseCall(tok)
		}
		return &IdentifierValue{Base: Base{tok.span}, Name: tok.text}, nil
	case tInteger:
		p.advance()
		suffix := ""
		for _, candidate := range []string{"UL", "U", "L"} {
			if strings.HasSuffix(tok.text, candidate) {
				suffix = candidate
				break
			}
		}
		digits := strings.TrimSuffix(tok.text, suffix)
		integer := &IntegerValue{Base: Base{tok.span}, Suffix: suffix}
		if suffix == "U" || suffix == "UL" {
			integer.UnsignedValue, _ = strconv.ParseUint(strings.TrimPrefix(digits, "+"), 10, 64)
		} else {
			integer.Value, _ = strconv.ParseInt(digits, 10, 64)
		}
		return integer, nil
	case tFloat:
		p.advance()
		value, _ := strconv.ParseFloat(tok.text, 64)
		return &FloatValue{Base: Base{tok.span}, Value: value}, nil
	case tString, tStringName, tNodePath:
		p.advance()
		kind := String
		if tok.kind == tStringName {
			kind = StringName
		} else if tok.kind == tNodePath {
			kind = NodePath
		}
		return &StringValue{Base: Base{tok.span}, Value: tok.value, Kind: kind}, nil
	case tLBracket:
		return p.parseArray()
	case tLBrace:
		return p.parseDictionary()
	default:
		return nil, p.error(tok, "expected a Variant value")
	}
}
func (p *parser) parseArray() (*ArrayValue, error) {
	start := p.advance()
	result := &ArrayValue{}
	for {
		p.skipNewlines()
		if p.at(tRBracket) {
			end := p.advance()
			result.SourceSpan = token.Span{Start: start.span.Start, End: end.span.End}
			return result, nil
		}
		if p.at(tEOF) {
			return nil, p.error(p.peek(), "expected ']' after array")
		}
		if p.at(tComment) {
			result.Items = append(result.Items, p.comment(p.advance()))
			p.consumeCommentLine()
			continue
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, value.(CompositeItem))
		if p.match(tComma) {
			continue
		}
		p.skipNewlines()
		if !p.at(tRBracket) && !p.at(tComment) {
			return nil, p.error(p.peek(), "expected ',' or ']' in array")
		}
	}
}
func (p *parser) parseDictionary() (*DictionaryValue, error) {
	start := p.advance()
	result := &DictionaryValue{}
	for {
		p.skipNewlines()
		if p.at(tRBrace) {
			end := p.advance()
			result.SourceSpan = token.Span{Start: start.span.Start, End: end.span.End}
			return result, nil
		}
		if p.at(tEOF) {
			return nil, p.error(p.peek(), "expected '}' after dictionary")
		}
		if p.at(tComment) {
			result.Items = append(result.Items, p.comment(p.advance()))
			p.consumeCommentLine()
			continue
		}
		key, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(tColon, "expected ':' after dictionary key"); err != nil {
			return nil, err
		}
		p.skipNewlines()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		entry := &DictionaryEntry{Base: Base{SourceSpan: token.Span{Start: key.Span().Start, End: value.Span().End}}, Key: key, Value: value}
		result.Items = append(result.Items, entry)
		if p.match(tComma) {
			continue
		}
		p.skipNewlines()
		if !p.at(tRBrace) && !p.at(tComment) {
			return nil, p.error(p.peek(), "expected ',' or '}' in dictionary")
		}
	}
}
func (p *parser) parseCall(name lexToken) (*CallValue, error) {
	p.advance()
	result := &CallValue{Name: name.text}
	for {
		p.skipNewlines()
		if p.at(tRParen) {
			end := p.advance()
			result.SourceSpan = token.Span{Start: name.span.Start, End: end.span.End}
			return result, nil
		}
		if p.at(tEOF) {
			return nil, p.error(p.peek(), "expected ')' after arguments")
		}
		if p.at(tComment) {
			result.Arguments = append(result.Arguments, p.comment(p.advance()))
			p.consumeCommentLine()
			continue
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		result.Arguments = append(result.Arguments, value.(CompositeItem))
		if p.match(tComma) {
			continue
		}
		p.skipNewlines()
		if !p.at(tRParen) && !p.at(tComment) {
			return nil, p.error(p.peek(), "expected ',' or ')' in arguments")
		}
	}
}
func (p *parser) parseTyped(name lexToken) (Value, error) {
	p.advance()
	first, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	var second *TypeRef
	if name.text == "Dictionary" {
		if _, err = p.expect(tComma, "expected ',' between dictionary types"); err != nil {
			return nil, err
		}
		second, err = p.parseTypeRef()
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.expect(tRBracket, "expected ']' after type arguments"); err != nil {
		return nil, err
	}
	if _, err = p.expect(tLParen, "expected '(' after typed container"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	if name.text == "Array" {
		array, err := p.parseArray()
		if err != nil {
			return nil, err
		}
		p.skipNewlines()
		end, err := p.expect(tRParen, "expected ')' after typed array")
		if err != nil {
			return nil, err
		}
		return &TypedArrayValue{Base: Base{SourceSpan: token.Span{Start: name.span.Start, End: end.span.End}}, ElementType: first, Array: array}, nil
	}
	dictionary, err := p.parseDictionary()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	end, err := p.expect(tRParen, "expected ')' after typed dictionary")
	if err != nil {
		return nil, err
	}
	return &TypedDictionaryValue{Base: Base{SourceSpan: token.Span{Start: name.span.Start, End: end.span.End}}, KeyType: first, ValueType: second, Dictionary: dictionary}, nil
}
func (p *parser) parseTypeRef() (*TypeRef, error) {
	name, err := p.expect(tIdentifier, "expected type name")
	if err != nil {
		return nil, err
	}
	result := &TypeRef{Name: name.text}
	end := name.span.End
	if p.at(tLParen) {
		call, err := p.parseCall(name)
		if err != nil {
			return nil, err
		}
		result.Constructor = call
		result.SourceSpan = call.SourceSpan
		return result, nil
	}
	if p.match(tLBracket) {
		for {
			arg, err := p.parseTypeRef()
			if err != nil {
				return nil, err
			}
			result.Arguments = append(result.Arguments, arg)
			end = arg.Span().End
			if p.match(tComma) {
				continue
			}
			close, err := p.expect(tRBracket, "expected ']' after nested type")
			if err != nil {
				return nil, err
			}
			end = close.span.End
			break
		}
	}
	result.SourceSpan = token.Span{Start: name.span.Start, End: end}
	return result, nil
}
func (p *parser) comment(tok lexToken) *Comment {
	return &Comment{Base: Base{tok.span}, Text: tok.value}
}
func (p *parser) consumeCommentLine() {
	if p.at(tComment) {
		p.advance()
	}
	if p.at(tNewline) {
		p.advance()
	}
}
func (p *parser) skipNewlines() {
	for p.match(tNewline) {
	}
}
func (p *parser) lineEnd() error {
	if p.match(tNewline) || p.at(tEOF) {
		return nil
	}
	return p.error(p.peek(), "expected end of line")
}
func (p *parser) peek() lexToken         { return p.tokens[p.current] }
func (p *parser) at(kind tokenKind) bool { return p.peek().kind == kind }
func (p *parser) advance() lexToken {
	tok := p.peek()
	if tok.kind != tEOF {
		p.current++
	}
	return tok
}
func (p *parser) match(kind tokenKind) bool {
	if !p.at(kind) {
		return false
	}
	p.advance()
	return true
}
func (p *parser) expect(kind tokenKind, message string) (lexToken, error) {
	if p.at(kind) {
		return p.advance(), nil
	}
	return lexToken{}, p.error(p.peek(), message)
}
func (p *parser) error(tok lexToken, message string) *Error {
	found := tok.text
	if tok.kind == tEOF {
		found = "end of file"
	} else if found == "" {
		found = tokenKindName(tok.kind)
	}
	return &Error{Filename: p.filename, Position: tok.span.Start, Message: message, Found: found}
}
func tokenKindName(k tokenKind) string {
	names := map[tokenKind]string{tNewline: "newline", tString: "string", tStringName: "StringName", tNodePath: "NodePath", tComment: "comment", tLBracket: "[", tRBracket: "]", tLBrace: "{", tRBrace: "}", tLParen: "(", tRParen: ")", tComma: ",", tColon: ":", tEqual: "="}
	if name := names[k]; name != "" {
		return name
	}
	return "token"
}
