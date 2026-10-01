package parser

import (
	"strings"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/token"
)

func (p *parser) parseExpression(minPrecedence int) (ast.Expression, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	left, err = p.parsePostfix(left)
	if err != nil {
		return nil, err
	}

	for {
		typ := p.peek().Type
		operator := p.peek().Lexeme
		precedence, rightAssociative := infixPrecedence(typ)
		if typ == token.Not && p.peekN(1).Type == token.In {
			operator, precedence = "not in", 3
		}
		if typ == token.Is && p.peekN(1).Type == token.Not {
			operator, precedence = "is not", 3
		}
		if precedence < minPrecedence {
			break
		}
		start := left.Span().Start
		operatorStart := p.advance()
		operatorEnd := operatorStart
		if operator == "not in" || operator == "is not" {
			operatorEnd = p.advance()
		}
		operatorSpan := token.Span{Start: operatorStart.Span.Start, End: operatorEnd.Span.End}
		if typ == token.As || typ == token.Is {
			right, typeErr := p.parseTypeExpression()
			if typeErr != nil {
				return nil, typeErr
			}
			left = &ast.BinaryExpression{
				Base: spanFrom(start, right.Span().End), Left: left, Operator: operator, OperatorSpan: operatorSpan, Right: right,
			}
			continue
		}
		nextMin := precedence + 1
		if rightAssociative {
			nextMin = precedence
		}
		right, err := p.parseExpression(nextMin)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Base: spanFrom(start, right.Span().End), Left: left, Operator: operator, OperatorSpan: operatorSpan, Right: right,
		}
	}

	if minPrecedence == 0 && p.at(token.If) {
		start := left.Span().Start
		ifToken := p.advance()
		condition, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		elseToken, err := p.expect(token.Else, "expected else in ternary expression")
		if err != nil {
			return nil, err
		}
		alternative, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		left = &ast.TernaryExpression{
			Base: spanFrom(start, alternative.Span().End), Value: left, IfSpan: ifToken.Span,
			Condition: condition, ElseSpan: elseToken.Span, Alternative: alternative,
		}
	}
	return left, nil
}

func (p *parser) parsePrefix() (ast.Expression, error) {
	tok := p.advance()
	switch tok.Type {
	case token.Identifier:
		return &ast.Identifier{Base: base(tok.Span), Name: tok.Lexeme}, nil
	case token.Integer:
		return &ast.Literal{Base: base(tok.Span), Kind: ast.IntegerLiteral, Raw: tok.Lexeme}, nil
	case token.Float:
		return &ast.Literal{Base: base(tok.Span), Kind: ast.FloatLiteral, Raw: tok.Lexeme}, nil
	case token.String:
		return stringLiteral(base(tok.Span), ast.StringLiteral, "", tok.Lexeme), nil
	case token.Ampersand:
		if !p.at(token.String) {
			return nil, p.error(tok, "expected string after '&'")
		}
		value := p.advance()
		return stringLiteral(spanFrom(tok.Span.Start, value.Span.End), ast.StringNameLiteral, "&", value.Lexeme), nil
	case token.Caret:
		if !p.at(token.String) {
			return nil, p.error(tok, "expected string after '^'")
		}
		value := p.advance()
		return stringLiteral(spanFrom(tok.Span.Start, value.Span.End), ast.NodePathLiteral, "^", value.Lexeme), nil
	case token.True, token.False:
		return &ast.Literal{Base: base(tok.Span), Kind: ast.BoolLiteral, Raw: tok.Lexeme}, nil
	case token.Null:
		return &ast.Literal{Base: base(tok.Span), Kind: ast.NullLiteral, Raw: tok.Lexeme}, nil
	case token.Minus, token.Plus, token.Not, token.Bang, token.Tilde, token.Await:
		operandPrecedence := 11
		if tok.Type == token.Not {
			operandPrecedence = 3
		}
		operand, err := p.parseExpression(operandPrecedence)
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpression{
			Base: spanFrom(tok.Span.Start, operand.Span().End), Operator: tok.Lexeme, OperatorSpan: tok.Span, Operand: operand,
		}, nil
	case token.LParen:
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RParen, "expected ')' after expression"); err != nil {
			return nil, err
		}
		return expr, nil
	case token.LBracket:
		return p.parseArray(tok)
	case token.LBrace:
		return p.parseDictionary(tok)
	case token.Dollar, token.Percent:
		return p.parseNodePath(tok)
	case token.Func:
		return p.parseLambda(tok)
	default:
		if isNameToken(tok) {
			return &ast.Identifier{Base: base(tok.Span), Name: tok.Lexeme}, nil
		}
		return nil, p.error(tok, "expected expression")
	}
}

func (p *parser) parsePostfix(expr ast.Expression) (ast.Expression, error) {
	for {
		switch {
		case p.match(token.LParen):
			var arguments []ast.Expression
			var comments []ast.CollectionComment
			p.takeCollectionComments(&comments, 0)
			if !p.at(token.RParen) {
				for {
					arg, err := p.parseExpression(0)
					if err != nil {
						return nil, err
					}
					arguments = append(arguments, arg)
					p.takeCollectionComments(&comments, len(arguments))
					if !p.match(token.Comma) {
						break
					}
					p.takeCollectionComments(&comments, len(arguments))
					if p.at(token.RParen) {
						break
					}
				}
			}
			end, err := p.expect(token.RParen, "expected ')' after arguments")
			if err != nil {
				return nil, err
			}
			expr = &ast.CallExpression{
				Base: spanFrom(expr.Span().Start, end.Span.End), Callee: expr,
				Arguments: arguments, Comments: comments,
			}
		case p.match(token.Dot):
			property, err := p.expectName("expected property name after '.'")
			if err != nil {
				return nil, err
			}
			expr = &ast.MemberExpression{
				Base: spanFrom(expr.Span().Start, property.Span.End), Object: expr,
				Property: property.Lexeme, PropertySpan: property.Span,
			}
		case p.match(token.LBracket):
			index, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			end, err := p.expect(token.RBracket, "expected ']' after subscript")
			if err != nil {
				return nil, err
			}
			expr = &ast.SubscriptExpression{Base: spanFrom(expr.Span().Start, end.Span.End), Object: expr, Index: index}
		default:
			return expr, nil
		}
	}
}

func (p *parser) parseLambda(start token.Token) (ast.Expression, error) {
	// A lambda may carry a name, which Godot reports in a stack trace instead
	// of the anonymous placeholder.
	name := ""
	var nameSpan token.Span
	if !p.at(token.LParen) {
		named, err := p.expectName("expected lambda name or '(' after func")
		if err != nil {
			return nil, err
		}
		name, nameSpan = named.Lexeme, named.Span
	}
	parameters, parameterComments, err := p.parseParameters(true)
	if err != nil {
		return nil, err
	}
	returnType := ""
	var returnArrowSpan, returnTypeSpan token.Span
	if p.at(token.Arrow) {
		returnArrowSpan = p.advance().Span
		returnType, returnTypeSpan, err = p.parseTypeUntil(token.Colon)
		if err != nil {
			return nil, err
		}
		if returnType == "" {
			return nil, p.error(p.peek(), "expected lambda return type")
		}
	}
	inline := p.peekN(1).Type != token.Newline && p.peekN(1).Type != token.Comment
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	return &ast.LambdaExpression{
		Base: spanFrom(start.Span.Start, end), Name: name, NameSpan: nameSpan,
		Parameters: parameters, ReturnType: returnType,
		ReturnTypeSpan: returnTypeSpan, ReturnArrowSpan: returnArrowSpan, KeywordSpan: start.Span,
		Body: body, Inline: inline, ParameterComments: parameterComments,
	}, nil
}

func (p *parser) parseTypeExpression() (ast.Expression, error) {
	start := p.peek()
	if !isNameToken(start) {
		return nil, p.error(start, "expected type name")
	}
	var name strings.Builder
	name.WriteString(p.advance().Lexeme)
	for p.match(token.Dot) {
		part, err := p.expectName("expected type name after '.'")
		if err != nil {
			return nil, err
		}
		name.WriteByte('.')
		name.WriteString(part.Lexeme)
	}
	if p.match(token.LBracket) {
		name.WriteByte('[')
		for {
			argument, err := p.parseTypeExpression()
			if err != nil {
				return nil, err
			}
			name.WriteString(argument.(*ast.TypeExpression).Name)
			if !p.match(token.Comma) {
				break
			}
			name.WriteString(", ")
		}
		if _, err := p.expect(token.RBracket, "expected ']' after type arguments"); err != nil {
			return nil, err
		}
		name.WriteByte(']')
	}
	return &ast.TypeExpression{Base: spanFrom(start.Span.Start, p.previous().Span.End), Name: name.String()}, nil
}

func (p *parser) parseArray(start token.Token) (ast.Expression, error) {
	return p.parseArrayOf(start, func() (ast.Expression, error) { return p.parseExpression(0) })
}

// parseArrayOf parses a bracketed element list, reading each element with
// parseElement. A match pattern reads its elements as patterns rather than as
// expressions, which is the only difference between the two forms.
func (p *parser) parseArrayOf(start token.Token, parseElement func() (ast.Expression, error)) (ast.Expression, error) {
	var elements []ast.Expression
	var comments []ast.CollectionComment
	p.takeCollectionComments(&comments, 0)
	if !p.at(token.RBracket) {
		for {
			element, err := parseElement()
			if err != nil {
				return nil, err
			}
			elements = append(elements, element)
			p.takeCollectionComments(&comments, len(elements))
			if !p.match(token.Comma) {
				break
			}
			p.takeCollectionComments(&comments, len(elements))
			if p.at(token.RBracket) {
				break
			}
		}
	}
	end, err := p.expect(token.RBracket, "expected ']' after array")
	if err != nil {
		return nil, err
	}
	return &ast.ArrayLiteral{
		Base: spanFrom(start.Span.Start, end.Span.End), Elements: elements, Comments: comments,
	}, nil
}

func (p *parser) parseDictionary(start token.Token) (ast.Expression, error) {
	return p.parseDictionaryOf(start, func() (ast.Expression, error) { return p.parseExpression(0) }, true)
}

// parseDictionaryOf parses a braced entry list, reading each value with
// parseValue. A match pattern reads its values as patterns; its keys stay
// expressions, as Godot's dictionary pattern does, and luaStyle is false there
// because a pattern is written with colons only.
func (p *parser) parseDictionaryOf(start token.Token, parseValue func() (ast.Expression, error), luaAllowed bool) (ast.Expression, error) {
	var entries []ast.DictionaryEntry
	luaStyle := false
	var comments []ast.CollectionComment
	p.takeCollectionComments(&comments, 0)
	if !p.at(token.RBrace) {
		for {
			key, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			separator, err := p.dictionarySeparator(key, luaAllowed, len(entries) > 0, luaStyle)
			if err != nil {
				return nil, err
			}
			luaStyle = separator.Type == token.Assign
			value, err := parseValue()
			if err != nil {
				return nil, err
			}
			entries = append(entries, ast.DictionaryEntry{
				Key: key, Value: value, SeparatorSpan: separator.Span,
			})
			p.takeCollectionComments(&comments, len(entries))
			if !p.match(token.Comma) {
				break
			}
			p.takeCollectionComments(&comments, len(entries))
			if p.at(token.RBrace) {
				break
			}
		}
	}
	end, err := p.expect(token.RBrace, "expected '}' after dictionary")
	if err != nil {
		return nil, err
	}
	return &ast.DictionaryLiteral{
		Base: spanFrom(start.Span.Start, end.Span.End), Entries: entries, Comments: comments,
		LuaStyle: luaStyle && len(entries) > 0,
	}, nil
}

func (p *parser) parseNodePath(start token.Token) (ast.Expression, error) {
	var path strings.Builder
	var pathSpan token.Span
	if p.at(token.String) {
		value := p.advance()
		path.WriteString(value.Lexeme)
		pathSpan = value.Span
	} else {
		for p.at(token.Identifier, token.Slash) {
			part := p.advance()
			path.WriteString(part.Lexeme)
			if pathSpan == (token.Span{}) {
				pathSpan.Start = part.Span.Start
			}
			pathSpan.End = part.Span.End
		}
	}
	if path.Len() == 0 {
		return nil, p.error(p.peek(), "expected node path")
	}
	return &ast.NodePathExpression{
		Base: spanFrom(start.Span.Start, p.previous().Span.End), Path: path.String(), PathSpan: pathSpan,
		Unique: start.Type == token.Percent, PrefixSpan: start.Span,
	}, nil
}

func infixPrecedence(typ token.Type) (int, bool) {
	switch typ {
	case token.Or:
		return 1, false
	case token.And:
		return 2, false
	case token.Equal, token.NotEqual, token.Less, token.LessEqual, token.Greater, token.GreaterEqual, token.In, token.Is, token.As:
		return 3, false
	case token.Pipe:
		return 4, false
	case token.Caret:
		return 5, false
	case token.Ampersand:
		return 6, false
	case token.ShiftLeft, token.ShiftRight:
		return 7, false
	case token.Plus, token.Minus:
		return 8, false
	case token.Star, token.Slash, token.Percent:
		return 9, false
	case token.DoubleStar:
		return 10, true
	default:
		return -1, false
	}
}

// stringLiteral builds a string, string name, or node path literal, recording
// the quote character and the triple-quoted and raw-prefixed forms so that the
// formatter can requote it without re-lexing its escapes.
func stringLiteral(nodeBase ast.Base, kind ast.LiteralKind, prefix, lexeme string) *ast.Literal {
	literal := &ast.Literal{Base: nodeBase, Kind: kind, Raw: prefix + lexeme}
	body := lexeme
	if strings.HasPrefix(body, "r") {
		literal.RawPrefix = true
		body = body[1:]
	}
	if body == "" {
		return literal
	}
	literal.Quote = body[0]
	literal.Triple = len(body) >= 6 && body[1] == literal.Quote && body[2] == literal.Quote
	return literal
}

// parsePattern parses one match pattern. A pattern is an expression, except
// that it may also bind the matched value with "var name", and that an array or
// dictionary pattern holds patterns rather than expressions.
func (p *parser) parsePattern() (ast.Expression, error) {
	switch {
	case p.at(token.Var):
		keyword := p.advance()
		// A bind name is a plain identifier. The wildcard "_" names nothing, so
		// Godot rejects it here even though it is a pattern of its own.
		if p.peek().Type != token.Identifier || p.peek().Lexeme == "_" {
			return nil, p.error(p.peek(), "expected bind name after 'var'")
		}
		name, err := p.expectName("expected bind name after 'var'")
		if err != nil {
			return nil, err
		}
		return &ast.BindingPattern{
			Base: spanFrom(keyword.Span.Start, name.Span.End), Name: name.Lexeme,
			NameSpan: name.Span, KeywordSpan: keyword.Span,
		}, nil
	case p.at(token.LBracket):
		return p.parseArrayOf(p.advance(), p.parsePattern)
	case p.at(token.LBrace):
		return p.parseDictionaryOf(p.advance(), p.parsePattern, false)
	}
	return p.parseExpression(0)
}

// dictionarySeparator reads the ":" or "=" that follows a dictionary key. A
// literal is written in one style throughout, so once an entry has been read
// the separator it used is the only one the rest may use. Every key named with
// "=" is an identifier or a string, not only the first. A dictionary pattern
// takes colons only, which is luaAllowed being false.
func (p *parser) dictionarySeparator(key ast.Expression, luaAllowed, decided, luaStyle bool) (token.Token, error) {
	if decided && luaStyle {
		if !p.at(token.Assign) {
			return token.Token{}, p.error(p.peek(), "expected '=' after dictionary key, since the dictionary is written in the \"{key = value}\" style")
		}
		return p.luaDictionarySeparator(key)
	}
	if decided {
		if !p.at(token.Colon) {
			return token.Token{}, p.error(p.peek(), "expected ':' after dictionary key, since the dictionary is written in the \"{\"key\": value}\" style")
		}
		return p.advance(), nil
	}
	if luaAllowed && p.at(token.Assign) {
		return p.luaDictionarySeparator(key)
	}
	return p.expect(token.Colon, "expected ':' after dictionary key")
}

// luaDictionarySeparator reads the "=" of a "{key = value}" entry, which names
// its key and so requires an identifier or a string.
func (p *parser) luaDictionarySeparator(key ast.Expression) (token.Token, error) {
	if !namesADictionaryKey(key) {
		return token.Token{}, p.error(p.peek(), "expected an identifier or a string before '=' as a dictionary key")
	}
	return p.advance(), nil
}

// namesADictionaryKey reports whether key may be written before "=" in a
// dictionary, which Godot allows for an identifier and for a string.
func namesADictionaryKey(key ast.Expression) bool {
	switch node := key.(type) {
	case *ast.Identifier:
		return true
	case *ast.Literal:
		return node.Kind == ast.StringLiteral
	}
	return false
}
