// Package parser parses Godot ConfigFile/project.godot tokens into a typed AST.
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/cafecito-games/gdparser/projectconfig/ast"
	"github.com/cafecito-games/gdparser/projectconfig/lexer"
	"github.com/cafecito-games/gdparser/projectconfig/token"
)

// Error is a syntax error with a source position.
type Error struct {
	Filename string
	Token    token.Token
	Message  string
}

func (e *Error) Error() string {
	location := e.Token.Span.Start.String()
	if e.Filename != "" {
		location = e.Filename + ":" + location
	}
	return fmt.Sprintf("%s: %s (found %s)", location, e.Message, e.Token)
}

// Parse parses one Godot project configuration file.
func Parse(filename string, source []byte) (*ast.File, error) {
	tokens, err := lexer.Lex(source)
	if err != nil {
		if filename != "" {
			return nil, fmt.Errorf("%s:%w", filename, err)
		}
		return nil, err
	}
	p := &parser{filename: filename, source: source, tokens: tokens}
	file, err := p.parseFile()
	if err != nil {
		return nil, err
	}
	file.Name = filename
	if len(source) > 0 {
		file.SourceSpan = token.Span{
			Start: token.Position{Offset: 0, Line: 1, Column: 1},
			End:   tokens[len(tokens)-1].Span.End,
		}
	} else {
		file.SourceSpan = tokens[0].Span
	}
	return file, nil
}

type parser struct {
	filename string
	source   []byte
	tokens   []token.Token
	current  int
}

func (p *parser) parseFile() (*ast.File, error) {
	file := &ast.File{}
	var statements *[]ast.Statement = &file.Preamble
	var currentSection *ast.Section
	for !p.at(token.EOF) {
		for p.match(token.Newline) {
		}
		if p.at(token.EOF) {
			break
		}
		if p.at(token.LBracket) {
			section, err := p.parseSection()
			if err != nil {
				return nil, err
			}
			file.Sections = append(file.Sections, section)
			currentSection = section
			statements = &section.Statements
			continue
		}
		if p.at(token.Comment) {
			comment := p.advance()
			*statements = append(*statements, &ast.Comment{Base: base(comment.Span), Text: comment.Lexeme})
			if currentSection != nil {
				currentSection.SourceSpan.End = comment.Span.End
			}
			if !p.at(token.EOF) {
				if _, err := p.expect(token.Newline, "expected end of comment line"); err != nil {
					return nil, err
				}
			}
			continue
		}
		assignment, err := p.parseAssignment()
		if err != nil {
			return nil, err
		}
		*statements = append(*statements, assignment)
		if currentSection != nil {
			currentSection.SourceSpan.End = assignment.Span().End
		}
		if p.match(token.Comment) {
			comment := p.previous()
			*statements = append(*statements, &ast.Comment{Base: base(comment.Span), Text: comment.Lexeme})
			if currentSection != nil {
				currentSection.SourceSpan.End = comment.Span.End
			}
		}
		if !p.at(token.EOF) {
			if _, err := p.expect(token.Newline, "expected end of assignment line"); err != nil {
				return nil, err
			}
		}
	}
	return file, nil
}

func (p *parser) parseSection() (*ast.Section, error) {
	start := p.advance()
	for !p.at(token.RBracket) && !p.at(token.Newline) && !p.at(token.EOF) {
		p.advance()
	}
	if p.previous().Type == token.LBracket {
		return nil, p.error(p.peek(), "section name must not be empty")
	}
	end, err := p.expect(token.RBracket, "expected ']' after section name")
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(string(p.source[start.Span.End.Offset:end.Span.Start.Offset]))
	name = strings.ReplaceAll(name, `\]`, `]`)
	section := &ast.Section{Base: between(start.Span.Start, end.Span.End), Name: name}
	if p.match(token.Comment) {
		comment := p.previous()
		section.Statements = append(section.Statements, &ast.Comment{Base: base(comment.Span), Text: comment.Lexeme})
	}
	if !p.at(token.EOF) {
		if _, err := p.expect(token.Newline, "expected end of section line"); err != nil {
			return nil, err
		}
	}
	return section, nil
}

func (p *parser) parseAssignment() (*ast.Assignment, error) {
	keyToken := p.peek()
	if p.at(token.Assign, token.Newline, token.Comment, token.EOF) {
		return nil, p.error(p.peek(), "expected configuration key")
	}
	var keyTokens []token.Token
	for !p.at(token.Assign, token.Newline, token.Comment, token.EOF) {
		keyTokens = append(keyTokens, p.advance())
	}
	var key strings.Builder
	for _, part := range keyTokens {
		key.WriteString(part.Lexeme)
	}
	keyText := key.String()
	if len(keyTokens) == 1 && keyToken.Type == token.String {
		decoded, err := decodeString(keyText)
		if err != nil {
			return nil, p.error(keyToken, err.Error())
		}
		keyText = decoded
	}
	if keyText == "" {
		return nil, p.error(keyToken, "configuration key must not be empty")
	}
	if _, err := p.expect(token.Assign, "expected '=' after configuration key"); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &ast.Assignment{
		Base:  between(keyToken.Span.Start, value.Span().End),
		Key:   keyText,
		Value: value,
	}, nil
}

func (p *parser) parseExpression() (ast.Expression, error) {
	if p.match(token.Plus, token.Minus) {
		operator := p.previous()
		operand, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		switch operand.(type) {
		case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.Identifier:
		default:
			return nil, p.error(operator, "sign may only prefix a numeric value")
		}
		return &ast.UnaryExpression{
			Base:     between(operator.Span.Start, operand.Span().End),
			Operator: operator.Lexeme,
			Operand:  operand,
		}, nil
	}

	current := p.advance()
	switch current.Type {
	case token.Integer:
		if err := validateInteger(current.Lexeme); err != nil {
			return nil, p.error(current, err.Error())
		}
		return &ast.IntegerLiteral{Base: base(current.Span), Raw: current.Lexeme}, nil
	case token.Float:
		if _, err := strconv.ParseFloat(strings.ReplaceAll(current.Lexeme, "_", ""), 64); err != nil {
			return nil, p.error(current, "invalid floating-point literal")
		}
		return &ast.FloatLiteral{Base: base(current.Span), Raw: current.Lexeme}, nil
	case token.String:
		value, err := decodeString(current.Lexeme)
		if err != nil {
			return nil, p.error(current, err.Error())
		}
		return &ast.StringLiteral{Base: base(current.Span), Value: value}, nil
	case token.Ampersand, token.Caret:
		literal, err := p.expect(token.String, "expected string after prefix")
		if err != nil {
			return nil, err
		}
		value, decodeErr := decodeString(literal.Lexeme)
		if decodeErr != nil {
			return nil, p.error(literal, decodeErr.Error())
		}
		return &ast.StringLiteral{
			Base:   between(current.Span.Start, literal.Span.End),
			Value:  value,
			Prefix: current.Lexeme,
		}, nil
	case token.Identifier:
		switch strings.ToLower(current.Lexeme) {
		case "null", "nil":
			return &ast.NullLiteral{Base: base(current.Span)}, nil
		case "true", "false":
			return &ast.BoolLiteral{Base: base(current.Span), Value: strings.EqualFold(current.Lexeme, "true")}, nil
		case "nan", "inf":
			return &ast.FloatLiteral{Base: base(current.Span), Raw: strings.ToLower(current.Lexeme)}, nil
		}
		if current.Lexeme == "Array" && p.at(token.LBracket) {
			return p.parseTypedArray(current)
		}
		if current.Lexeme == "Dictionary" && p.at(token.LBracket) {
			return p.parseTypedDictionary(current)
		}
		if p.at(token.LParen) {
			return p.parseConstructor(current)
		}
		return &ast.Identifier{Base: base(current.Span), Name: current.Lexeme}, nil
	case token.LBracket:
		return p.parseArray(current)
	case token.LBrace:
		return p.parseDictionary(current)
	default:
		return nil, p.error(current, "expected Variant value")
	}
}

func (p *parser) parseArray(start token.Token) (ast.Expression, error) {
	array := &ast.ArrayLiteral{}
	p.consumeArrayLayout(array)
	if p.match(token.RBracket) {
		array.Base = between(start.Span.Start, p.previous().Span.End)
		return array, nil
	}
	for {
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		array.Items = append(array.Items, &ast.ArrayElement{Base: base(value.Span()), Value: value})
		p.consumeArrayLayout(array)
		if p.match(token.RBracket) {
			array.Base = between(start.Span.Start, p.previous().Span.End)
			return array, nil
		}
		if _, err := p.expect(token.Comma, "expected ',' or ']' in array"); err != nil {
			return nil, err
		}
		p.consumeArrayLayout(array)
		if p.match(token.RBracket) {
			array.Base = between(start.Span.Start, p.previous().Span.End)
			return array, nil
		}
	}
}

func (p *parser) parseDictionary(start token.Token) (ast.Expression, error) {
	dictionary := &ast.DictionaryLiteral{}
	p.consumeDictionaryLayout(dictionary)
	if p.match(token.RBrace) {
		dictionary.Base = between(start.Span.Start, p.previous().Span.End)
		return dictionary, nil
	}
	for {
		key, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		infixComments := p.takeValueLayout()
		if _, err := p.expect(token.Colon, "expected ':' after dictionary key"); err != nil {
			return nil, err
		}
		infixComments = append(infixComments, p.takeValueLayout()...)
		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		entry := &ast.DictionaryEntry{
			Base:          between(key.Span().Start, value.Span().End),
			Key:           key,
			InfixComments: infixComments,
			Value:         value,
		}
		dictionary.Items = append(dictionary.Items, entry)
		p.consumeDictionaryLayout(dictionary)
		if p.match(token.RBrace) {
			dictionary.Base = between(start.Span.Start, p.previous().Span.End)
			return dictionary, nil
		}
		if _, err := p.expect(token.Comma, "expected ',' or '}' in dictionary"); err != nil {
			return nil, err
		}
		p.consumeDictionaryLayout(dictionary)
		if p.match(token.RBrace) {
			dictionary.Base = between(start.Span.Start, p.previous().Span.End)
			return dictionary, nil
		}
	}
}

func (p *parser) parseConstructor(name token.Token) (ast.Expression, error) {
	p.advance() // (
	call := &ast.ConstructorCall{Name: name.Lexeme}
	p.consumeConstructorLayout(call)
	if p.match(token.RParen) {
		call.Base = between(name.Span.Start, p.previous().Span.End)
		return call, nil
	}
	for {
		first, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		argument := &ast.ConstructorArgument{Value: first, Base: base(first.Span())}
		infixComments := p.takeValueLayout()
		if p.match(token.Colon) {
			infixComments = append(infixComments, p.takeValueLayout()...)
			value, valueErr := p.parseExpression()
			if valueErr != nil {
				return nil, valueErr
			}
			argument.Key = first
			argument.InfixComments = infixComments
			argument.Value = value
			argument.Base = between(first.Span().Start, value.Span().End)
		} else {
			call.Items = append(call.Items, argument)
			for _, comment := range infixComments {
				call.Items = append(call.Items, comment)
			}
			infixComments = nil
		}
		if argument.Key != nil {
			call.Items = append(call.Items, argument)
		}
		p.consumeConstructorLayout(call)
		if p.match(token.RParen) {
			call.Base = between(name.Span.Start, p.previous().Span.End)
			return call, nil
		}
		if _, err := p.expect(token.Comma, "expected ',' or ')' in constructor"); err != nil {
			return nil, err
		}
		p.consumeConstructorLayout(call)
		if p.match(token.RParen) {
			call.Base = between(name.Span.Start, p.previous().Span.End)
			return call, nil
		}
	}
}

func (p *parser) parseTypedArray(name token.Token) (ast.Expression, error) {
	p.advance() // [
	p.skipNewlines()
	elementType, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if _, err := p.expect(token.RBracket, "expected ']' after array element type"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	if _, err := p.expect(token.LParen, "expected '(' after typed array type"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	start, err := p.expect(token.LBracket, "expected array value in typed array")
	if err != nil {
		return nil, err
	}
	valueExpression, err := p.parseArray(start)
	if err != nil {
		return nil, err
	}
	value := valueExpression.(*ast.ArrayLiteral)
	p.skipNewlines()
	end, err := p.expect(token.RParen, "expected ')' after typed array value")
	if err != nil {
		return nil, err
	}
	return &ast.TypedArrayLiteral{
		Base:        between(name.Span.Start, end.Span.End),
		ElementType: elementType,
		Value:       value,
	}, nil
}

func (p *parser) parseTypedDictionary(name token.Token) (ast.Expression, error) {
	p.advance() // [
	p.skipNewlines()
	keyType, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if _, err := p.expect(token.Comma, "expected ',' after dictionary key type"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	valueType, err := p.parseTypeRef()
	if err != nil {
		return nil, err
	}
	p.skipNewlines()
	if _, err := p.expect(token.RBracket, "expected ']' after dictionary value type"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	if _, err := p.expect(token.LParen, "expected '(' after typed dictionary types"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	start, err := p.expect(token.LBrace, "expected dictionary value in typed dictionary")
	if err != nil {
		return nil, err
	}
	valueExpression, err := p.parseDictionary(start)
	if err != nil {
		return nil, err
	}
	value := valueExpression.(*ast.DictionaryLiteral)
	p.skipNewlines()
	end, err := p.expect(token.RParen, "expected ')' after typed dictionary value")
	if err != nil {
		return nil, err
	}
	return &ast.TypedDictionaryLiteral{
		Base:      between(name.Span.Start, end.Span.End),
		KeyType:   keyType,
		ValueType: valueType,
		Value:     value,
	}, nil
}

func (p *parser) parseTypeRef() (*ast.TypeRef, error) {
	name, err := p.expect(token.Identifier, "expected Variant type name")
	if err != nil {
		return nil, err
	}
	typeRef := &ast.TypeRef{Base: base(name.Span), Name: name.Lexeme}
	if !p.match(token.LBracket) {
		return typeRef, nil
	}
	p.skipNewlines()
	for {
		argument, err := p.parseTypeRef()
		if err != nil {
			return nil, err
		}
		typeRef.Arguments = append(typeRef.Arguments, argument)
		p.skipNewlines()
		if p.match(token.RBracket) {
			typeRef.SourceSpan.End = p.previous().Span.End
			return typeRef, nil
		}
		if _, err := p.expect(token.Comma, "expected ',' or ']' in type arguments"); err != nil {
			return nil, err
		}
		p.skipNewlines()
	}
}

func (p *parser) skipNewlines() {
	for p.match(token.Newline) {
	}
}

func (p *parser) consumeArrayLayout(array *ast.ArrayLiteral) {
	for {
		if p.match(token.Newline) {
			continue
		}
		if p.match(token.Comment) {
			comment := p.previous()
			array.Items = append(array.Items, &ast.Comment{Base: base(comment.Span), Text: comment.Lexeme})
			p.match(token.Newline)
			continue
		}
		return
	}
}

func (p *parser) consumeDictionaryLayout(dictionary *ast.DictionaryLiteral) {
	for {
		if p.match(token.Newline) {
			continue
		}
		if p.match(token.Comment) {
			comment := p.previous()
			dictionary.Items = append(dictionary.Items, &ast.Comment{Base: base(comment.Span), Text: comment.Lexeme})
			p.match(token.Newline)
			continue
		}
		return
	}
}

func (p *parser) consumeConstructorLayout(call *ast.ConstructorCall) {
	for {
		if p.match(token.Newline) {
			continue
		}
		if p.match(token.Comment) {
			comment := p.previous()
			call.Items = append(call.Items, &ast.Comment{Base: base(comment.Span), Text: comment.Lexeme})
			p.match(token.Newline)
			continue
		}
		return
	}
}

func (p *parser) takeValueLayout() []*ast.Comment {
	var comments []*ast.Comment
	for {
		if p.match(token.Newline) {
			continue
		}
		if p.match(token.Comment) {
			comment := p.previous()
			comments = append(comments, &ast.Comment{Base: base(comment.Span), Text: comment.Lexeme})
			p.match(token.Newline)
			continue
		}
		return comments
	}
}

func (p *parser) at(types ...token.Type) bool {
	for _, typ := range types {
		if p.peek().Type == typ {
			return true
		}
	}
	return false
}

func (p *parser) match(types ...token.Type) bool {
	if !p.at(types...) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) expect(typ token.Type, message string) (token.Token, error) {
	if p.at(typ) {
		return p.advance(), nil
	}
	return token.Token{}, p.error(p.peek(), message)
}

func (p *parser) peek() token.Token { return p.tokens[p.current] }

func (p *parser) previous() token.Token { return p.tokens[p.current-1] }

func (p *parser) advance() token.Token {
	current := p.peek()
	if current.Type != token.EOF {
		p.current++
	}
	return current
}

func (p *parser) error(at token.Token, message string) error {
	return &Error{Filename: p.filename, Token: at, Message: message}
}

func base(span token.Span) ast.Base { return ast.Base{SourceSpan: span} }

func between(start, end token.Position) ast.Base {
	return base(token.Span{Start: start, End: end})
}

func decodeString(raw string) (string, error) {
	if len(raw) < 2 {
		return "", fmt.Errorf("invalid string literal")
	}
	quote := raw[0]
	triple := len(raw) >= 6 && raw[0] == raw[1] && raw[1] == raw[2] && raw[len(raw)-1] == quote && raw[len(raw)-2] == quote && raw[len(raw)-3] == quote
	body := raw[1 : len(raw)-1]
	if triple {
		body = raw[3 : len(raw)-3]
	}
	var out strings.Builder
	for len(body) > 0 {
		if body[0] != '\\' {
			r, size := utf8.DecodeRuneInString(body)
			if r == utf8.RuneError && size == 1 {
				return "", fmt.Errorf("invalid UTF-8 in string literal")
			}
			out.WriteRune(r)
			body = body[size:]
			continue
		}
		body = body[1:]
		if body == "" {
			return "", fmt.Errorf("unterminated escape sequence")
		}
		escape := body[0]
		body = body[1:]
		switch escape {
		case 'a':
			out.WriteByte('\a')
		case 'b':
			out.WriteByte('\b')
		case 't':
			out.WriteByte('\t')
		case 'n':
			out.WriteByte('\n')
		case 'v':
			out.WriteByte('\v')
		case 'f':
			out.WriteByte('\f')
		case 'r':
			out.WriteByte('\r')
		case '\\', '/', '"', '\'':
			out.WriteByte(escape)
		case '\n': // escaped physical newline
		case 'u', 'U':
			digits := 4
			if escape == 'U' {
				digits = 6
			}
			if len(body) < digits {
				return "", fmt.Errorf("incomplete Unicode escape")
			}
			value, err := strconv.ParseUint(body[:digits], 16, 32)
			if err != nil {
				return "", fmt.Errorf("invalid Unicode escape")
			}
			body = body[digits:]
			r := rune(value)
			if escape == 'u' && r >= 0xd800 && r <= 0xdbff {
				if len(body) < 6 || body[0] != '\\' || body[1] != 'u' {
					return "", fmt.Errorf("unpaired Unicode surrogate")
				}
				low, lowErr := strconv.ParseUint(body[2:6], 16, 16)
				if lowErr != nil || low < 0xdc00 || low > 0xdfff {
					return "", fmt.Errorf("unpaired Unicode surrogate")
				}
				body = body[6:]
				r = utf16.DecodeRune(r, rune(low))
			} else if utf16.IsSurrogate(r) {
				return "", fmt.Errorf("unpaired Unicode surrogate")
			}
			if !utf8.ValidRune(r) {
				return "", fmt.Errorf("invalid Unicode code point")
			}
			out.WriteRune(r)
		default:
			return "", fmt.Errorf("unknown escape sequence \\%c", escape)
		}
	}
	return out.String(), nil
}

func validateInteger(raw string) error {
	value := strings.ReplaceAll(raw, "_", "")
	if strings.HasSuffix(value, "UL") {
		value = strings.TrimSuffix(value, "UL")
	} else if strings.HasSuffix(value, "U") {
		value = strings.TrimSuffix(value, "U")
	} else if strings.HasSuffix(value, "L") {
		value = strings.TrimSuffix(value, "L")
	}
	if value == "" {
		return fmt.Errorf("invalid integer literal")
	}
	base := 10
	digits := value
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 16
		digits = value[2:]
	} else if strings.HasPrefix(value, "0b") || strings.HasPrefix(value, "0B") {
		base = 2
		digits = value[2:]
	}
	if digits == "" {
		return fmt.Errorf("invalid integer literal")
	}
	for _, digit := range digits {
		valid := digit >= '0' && digit <= '9' && int(digit-'0') < base
		if base == 16 {
			valid = valid || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F'
		}
		if !valid {
			return fmt.Errorf("invalid integer literal")
		}
	}
	return nil
}
