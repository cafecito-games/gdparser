package parser

import (
	"strings"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/token"
)

// parseExpression reads an expression whose operators all bind at or above
// minPrecedence, which is how Godot's parse_precedence reads one. The ternary
// conditional is an infix operator of its own level there rather than a form
// that only a whole expression may take.
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
		if typ == token.Not && p.peekN(1).Type == token.In {
			operator = "not in"
		}
		if typ == token.Is && p.peekN(1).Type == token.Not {
			operator = "is not"
		}
		precedence := ast.OperatorPrecedence(operator)
		if typ == token.If {
			precedence = ast.PrecedenceTernary
		}
		if precedence == ast.PrecedenceNone || precedence < minPrecedence {
			break
		}
		if p.lambdaEnded && !p.multiline() {
			// A lambda body just ended, and nothing encloses it that spans
			// lines, so what follows begins the next statement rather than
			// continuing this expression. Without this an "if" on the line after
			// a lambda would be read as a conditional.
			break
		}
		start := left.Span().Start
		if typ == token.If {
			left, err = p.parseTernary(left, start)
			if err != nil {
				return nil, err
			}
			continue
		}
		operatorStart := p.advance()
		operatorEnd := operatorStart
		if operator == "not in" || operator == "is not" {
			operatorEnd = p.advance()
		}
		operatorSpan := token.Span{Start: operatorStart.Span.Start, End: operatorEnd.Span.End}
		var right ast.Expression
		if typ == token.As || typ == token.Is {
			// A cast and a type test read a type rather than an expression, so
			// the level they bind at decides only where they attach.
			right, err = p.parseTypeExpression()
		} else {
			right, err = p.parseExpression(precedence + 1)
		}
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpression{
			Base: spanFrom(start, right.Span().End), Left: left, Operator: operator, OperatorSpan: operatorSpan, Right: right,
		}
	}
	return left, nil
}

// parseTernary reads the rest of "value if condition else alternative", whose
// "if" the caller has found but not consumed. Godot reads both the condition and
// the alternative at the conditional's own level, which lets the alternative
// hold another conditional without parentheses.
func (p *parser) parseTernary(value ast.Expression, start token.Position) (ast.Expression, error) {
	ifToken := p.advance()
	condition, err := p.parseExpression(ast.PrecedenceTernary)
	if err != nil {
		return nil, err
	}
	elseToken, err := p.expect(token.Else, "expected else in ternary expression")
	if err != nil {
		return nil, err
	}
	alternative, err := p.parseExpression(ast.PrecedenceTernary)
	if err != nil {
		return nil, err
	}
	return &ast.TernaryExpression{
		Base: spanFrom(start, alternative.Span().End), Value: value, IfSpan: ifToken.Span,
		Condition: condition, ElseSpan: elseToken.Span, Alternative: alternative,
	}, nil
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
		operand, err := p.parseExpression(ast.UnaryOperandPrecedence(tok.Lexeme))
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpression{
			Base: spanFrom(tok.Span.Start, operand.Span().End), Operator: tok.Lexeme, OperatorSpan: tok.Span, Operand: operand,
		}, nil
	case token.LParen:
		p.pushMultiline(true)
		expr, err := p.parseExpression(ast.PrecedenceAssignment)
		if err != nil {
			return nil, err
		}
		p.popMultiline()
		if _, err := p.expect(token.RParen, "expected ')' after expression"); err != nil {
			return nil, err
		}
		return expr, nil
	case token.LBracket:
		p.pushMultiline(true)
		return p.parseArray(tok)
	case token.LBrace:
		p.pushMultiline(true)
		return p.parseDictionary(tok)
	case token.Dollar, token.Percent:
		return p.parseNodePath(tok)
	case token.Func:
		return p.parseLambda(tok)
	case token.Self:
		// Godot models self, super and preload as keywords rather than as
		// names, but each one is spelled exactly as a name is, so each is kept
		// as an identifier here and the grammar around it is what differs. No
		// variable can carry one of these names, so the spelling stays
		// unambiguous.
		return &ast.Identifier{Base: base(tok.Span), Name: tok.Lexeme}, nil
	case token.Super:
		return p.parseSuper(tok)
	case token.Preload:
		return p.parsePreload(tok)
	default:
		// Godot rewrites a keyword that may stand for an identifier into one
		// before it looks for a prefix rule, so "match" and "when" name a value
		// here as well as a declaration.
		if token.IsIdentifier(tok.Type) {
			return &ast.Identifier{Base: base(tok.Span), Name: tok.Lexeme}, nil
		}
		return nil, p.error(tok, "expected expression")
	}
}

// parseSuper parses the "super" of a parent call, which Godot writes either as
// super(arguments), calling the parent's version of the current function, or as
// super.name(arguments). The call itself is read as a postfix, so this only has
// to reject a "super" that names no call.
func (p *parser) parseSuper(keyword token.Token) (ast.Expression, error) {
	expr := ast.Expression(&ast.Identifier{Base: base(keyword.Span), Name: keyword.Lexeme})
	if p.match(token.Dot) {
		name, err := p.expectIdentifier("expected function name after '.'")
		if err != nil {
			return nil, err
		}
		expr = &ast.MemberExpression{
			Base: spanFrom(keyword.Span.Start, name.Span.End), Object: expr,
			Property: name.Lexeme, PropertySpan: name.Span,
		}
	}
	if !p.at(token.LParen) {
		return nil, p.error(p.peek(), "expected '(' after super call")
	}
	return expr, nil
}

// parsePreload parses preload(path), which Godot reads as a keyword taking
// exactly one argument rather than as a call to a function of that name.
func (p *parser) parsePreload(keyword token.Token) (ast.Expression, error) {
	if !p.at(token.LParen) {
		return nil, p.error(p.peek(), "expected '(' after preload")
	}
	p.advance()
	p.pushMultiline(true)
	callee := ast.Expression(&ast.Identifier{Base: base(keyword.Span), Name: keyword.Lexeme})
	arguments, comments, end, err := p.parseArguments()
	if err != nil {
		return nil, err
	}
	if len(arguments) != 1 {
		return nil, p.error(keyword, "preload takes one path")
	}
	return &ast.CallExpression{
		Base: spanFrom(keyword.Span.Start, end.Span.End), Callee: callee,
		Arguments: arguments, Comments: comments,
	}, nil
}

func (p *parser) parsePostfix(expr ast.Expression) (ast.Expression, error) {
	for {
		switch {
		case p.match(token.LParen):
			p.pushMultiline(true)
			arguments, comments, end, err := p.parseArguments()
			if err != nil {
				return nil, err
			}
			expr = &ast.CallExpression{
				Base: spanFrom(expr.Span().Start, end.Span.End), Callee: expr,
				Arguments: arguments, Comments: comments,
			}
		case p.match(token.Dot):
			property, err := p.expectNodeName("expected property name after '.'")
			if err != nil {
				return nil, err
			}
			expr = &ast.MemberExpression{
				Base: spanFrom(expr.Span().Start, property.Span.End), Object: expr,
				Property: property.Lexeme, PropertySpan: property.Span,
			}
		case p.match(token.LBracket):
			p.pushMultiline(true)
			index, err := p.parseExpression(ast.PrecedenceAssignment)
			if err != nil {
				return nil, err
			}
			p.popMultiline()
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

// parseArguments reads the arguments of a call whose "(" has been consumed,
// through the closing ")" it returns.
func (p *parser) parseArguments() ([]ast.Expression, []ast.CollectionComment, token.Token, error) {
	var arguments []ast.Expression
	var comments []ast.CollectionComment
	p.takeCollectionComments(&comments, 0)
	if !p.at(token.RParen) {
		for {
			argument, err := p.parseExpression(ast.PrecedenceAssignment)
			if err != nil {
				return nil, nil, token.Token{}, err
			}
			arguments = append(arguments, argument)
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
	p.popMultiline()
	end, err := p.expect(token.RParen, "expected ')' after arguments")
	if err != nil {
		return nil, nil, token.Token{}, err
	}
	return arguments, comments, end, nil
}

func (p *parser) parseLambda(start token.Token) (ast.Expression, error) {
	// A line break inside a lambda body carries meaning even when the lambda is
	// written inside brackets, where one otherwise would not. Godot resets the
	// mode for the body and sets the indentation built up so far aside, so the
	// body may open a block of its own and the enclosing lines keep theirs.
	multilineContext := p.multiline()
	p.pushMultiline(false)
	if multilineContext {
		p.scanner.PushExpressionIndentedBlock()
	}
	// A lambda may carry a name, which Godot reports in a stack trace instead
	// of the anonymous placeholder.
	name := ""
	var nameSpan token.Span
	if !p.at(token.LParen) {
		named, err := p.expectIdentifier("expected lambda name or '(' after func")
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
		returnType, returnTypeSpan, err = p.parseType(true, nil, 0)
		if err != nil {
			return nil, err
		}
		if returnType == "" {
			return nil, p.error(p.peek(), "expected lambda return type")
		}
	}
	inline := p.peekN(1).Type != token.Newline && p.peekN(1).Type != token.Comment
	previousInLambda := p.inLambda
	p.inLambda = true
	// A body about to be read has not ended, whatever an earlier lambda in the
	// same statement left behind. Godot reads a body's first statement before it
	// looks at the marker at all, which comes to the same thing.
	p.lambdaEnded = false
	body, end, err := p.parseSuiteFor(true)
	p.inLambda = previousInLambda
	if err != nil {
		return nil, err
	}
	p.popMultiline()
	if multilineContext {
		p.scanner.PopExpressionIndentedBlock()
	}
	// The body is over, whether it ended at a dedent or at the first thing that
	// could not continue it. Either way the statement holding the lambda may end
	// here, which is what lambdaEnded carries. It is marked after the body's own
	// multiline frame is popped, since popping one forgets such a mark.
	p.lambdaEnded = true
	return &ast.LambdaExpression{
		Base: spanFrom(start.Span.Start, end), Name: name, NameSpan: nameSpan,
		Parameters: parameters, ReturnType: returnType,
		ReturnTypeSpan: returnTypeSpan, ReturnArrowSpan: returnArrowSpan, KeywordSpan: start.Span,
		Body: body, Inline: inline, ParameterComments: parameterComments,
	}, nil
}

// parseTypeExpression reads the type that "as" or "is" tests against, which is
// the same grammar a declaration's type uses, minus "void".
func (p *parser) parseTypeExpression() (ast.Expression, error) {
	start := p.peek()
	name, span, err := p.parseType(false, nil, 0)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, p.error(start, "expected type name")
	}
	return &ast.TypeExpression{Base: base(span), Name: name}, nil
}

func (p *parser) parseArray(start token.Token) (ast.Expression, error) {
	return p.parseArrayOf(start, func() (ast.Expression, error) { return p.parseExpression(ast.PrecedenceAssignment) })
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
	p.popMultiline()
	end, err := p.expect(token.RBracket, "expected ']' after array")
	if err != nil {
		return nil, err
	}
	return &ast.ArrayLiteral{
		Base: spanFrom(start.Span.Start, end.Span.End), Elements: elements, Comments: comments,
	}, nil
}

func (p *parser) parseDictionary(start token.Token) (ast.Expression, error) {
	return p.parseDictionaryOf(start, func() (ast.Expression, error) { return p.parseExpression(ast.PrecedenceAssignment) }, true)
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
			key, err := p.parseExpression(ast.PrecedenceAssignment)
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
	p.popMultiline()
	end, err := p.expect(token.RBrace, "expected '}' after dictionary")
	if err != nil {
		return nil, err
	}
	return &ast.DictionaryLiteral{
		Base: spanFrom(start.Span.Start, end.Span.End), Entries: entries, Comments: comments,
		LuaStyle: luaStyle && len(entries) > 0,
	}, nil
}

// pathStep says how much of a node path has been read, which is what decides
// whether a "/" or a "%" may come next.
type pathStep int

const (
	// pathStart is the position right after the "$" or "%" that opens the path.
	pathStart pathStep = iota
	// pathSlash follows a "/" separator.
	pathSlash
	// pathPercent follows the "%" that marks the next name as unique.
	pathPercent
	// pathName follows a node name, whether written bare or as a string.
	pathName
)

// parseNodePath parses the $Node/Child and %UniqueNode shorthand. Godot reads
// the path as names separated by "/", where a name may be written bare or as a
// string, and where a "%" before a name marks that one node as unique. Nearly
// every keyword may name a node, so the path keeps its own predicate rather
// than the one a declared name uses.
func (p *parser) parseNodePath(start token.Token) (ast.Expression, error) {
	var path strings.Builder
	var pathSpan token.Span
	write := func(tok token.Token, text string) {
		path.WriteString(text)
		if pathSpan == (token.Span{}) {
			pathSpan.Start = tok.Span.Start
		}
		pathSpan.End = tok.Span.End
	}
	step := pathStart
	if start.Type == token.Dollar && p.at(token.Slash) {
		// A path may be written from the scene root, as in $/root/Main.
		write(p.advance(), "/")
		step = pathSlash
	}
	for {
		switch {
		case p.at(token.String):
			tok := p.advance()
			write(tok, tok.Lexeme)
			step = pathName
		case token.IsNodeName(p.peek().Type):
			tok := p.advance()
			write(tok, tok.Lexeme)
			step = pathName
		case !p.at(token.Slash, token.Percent):
			return nil, p.error(p.peek(), "expected node path")
		}
		switch {
		case p.at(token.Slash):
			if step != pathStart && step != pathName {
				return nil, p.error(p.peek(), "'/' may only separate node names in a node path")
			}
			write(p.advance(), "/")
			step = pathSlash
		case p.at(token.Percent):
			if step != pathStart && step != pathSlash {
				return nil, p.error(p.peek(), "'%' may only begin a node name in a node path")
			}
			write(p.advance(), "%")
			step = pathPercent
		default:
			if path.Len() == 0 || step != pathName {
				return nil, p.error(p.peek(), "expected node path")
			}
			return &ast.NodePathExpression{
				Base: spanFrom(start.Span.Start, p.previous().Span.End), Path: path.String(), PathSpan: pathSpan,
				Unique: start.Type == token.Percent, PrefixSpan: start.Span,
			}, nil
		}
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
		name, err := p.expectIdentifier("expected bind name after 'var'")
		if err != nil {
			return nil, err
		}
		return &ast.BindingPattern{
			Base: spanFrom(keyword.Span.Start, name.Span.End), Name: name.Lexeme,
			NameSpan: name.Span, KeywordSpan: keyword.Span,
		}, nil
	case p.at(token.Underscore):
		keyword := p.advance()
		return &ast.WildcardPattern{Base: base(keyword.Span)}, nil
	case p.at(token.Range):
		keyword := p.advance()
		return &ast.RestPattern{Base: base(keyword.Span)}, nil
	case p.at(token.LBracket):
		opener := p.advance()
		p.pushMultiline(true)
		return p.parseArrayPattern(opener)
	case p.at(token.LBrace):
		opener := p.advance()
		p.pushMultiline(true)
		return p.parseDictionaryPattern(opener)
	}
	return p.parseExpression(ast.PrecedenceAssignment)
}

// parseDictionaryPattern parses a braced dictionary pattern, whose "{" has been
// consumed. It differs from a dictionary literal in three ways: a key may stand
// alone, which tests only that the key is present; ".." may stand in for the
// entries the pattern does not list; and the "{key = value}" spelling has no
// pattern form, so an entry is always written with a colon.
// parseArrayPattern parses a bracketed array pattern, whose "[" has been
// consumed. Its elements are patterns, and ".." may stand in for the elements
// the pattern does not list, as the last one.
func (p *parser) parseArrayPattern(start token.Token) (ast.Expression, error) {
	rest := false
	array, err := p.parseArrayOf(start, func() (ast.Expression, error) {
		if rest {
			return nil, p.error(p.peek(), "the '..' pattern must be the last element of an array pattern")
		}
		element, elementErr := p.parsePattern()
		if elementErr != nil {
			return nil, elementErr
		}
		_, rest = element.(*ast.RestPattern)
		return element, nil
	})
	if err != nil {
		return nil, err
	}
	return array, nil
}

func (p *parser) parseDictionaryPattern(start token.Token) (ast.Expression, error) {
	var entries []ast.DictionaryEntry
	var comments []ast.CollectionComment
	rest := false
	p.takeCollectionComments(&comments, 0)
	for !p.at(token.RBrace) {
		if rest {
			return nil, p.error(p.peek(), "the '..' pattern must be the last entry of a dictionary pattern")
		}
		if p.at(token.Range) {
			// The rest is held as an entry whose key is the ".." itself, so that
			// an entry with no value is written as its key alone either way.
			keyword := p.advance()
			entries = append(entries, ast.DictionaryEntry{Key: &ast.RestPattern{Base: base(keyword.Span)}})
			rest = true
		} else {
			key, err := p.parseExpression(ast.PrecedenceAssignment)
			if err != nil {
				return nil, err
			}
			entry := ast.DictionaryEntry{Key: key}
			if p.at(token.Colon) {
				entry.SeparatorSpan = p.advance().Span
				value, valueErr := p.parsePattern()
				if valueErr != nil {
					return nil, valueErr
				}
				if _, isRest := value.(*ast.RestPattern); isRest {
					return nil, p.error(p.previous(), "the '..' pattern cannot stand for a value")
				}
				entry.Value = value
			}
			entries = append(entries, entry)
		}
		p.takeCollectionComments(&comments, len(entries))
		if !p.match(token.Comma) {
			break
		}
		p.takeCollectionComments(&comments, len(entries))
	}
	p.popMultiline()
	end, err := p.expect(token.RBrace, "expected '}' after dictionary pattern")
	if err != nil {
		return nil, err
	}
	return &ast.DictionaryLiteral{
		Base: spanFrom(start.Span.Start, end.Span.End), Entries: entries, Comments: comments,
	}, nil
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
