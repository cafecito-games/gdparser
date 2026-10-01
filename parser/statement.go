package parser

import (
	"strings"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/token"
)

func (p *parser) parseAnnotation() (ast.Statement, error) {
	start := p.advance()
	name, err := p.expectName("expected annotation name")
	if err != nil {
		return nil, err
	}
	var arguments []ast.Expression
	var comments []ast.CollectionComment
	end := name.Span.End
	if p.match(token.LParen) {
		p.takeCollectionComments(&comments, 0)
		if !p.at(token.RParen) {
			for {
				argument, err := p.parseExpression(0)
				if err != nil {
					return nil, err
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
		closing, err := p.expect(token.RParen, "expected ')' after annotation")
		if err != nil {
			return nil, err
		}
		end = closing.Span.End
	}
	return &ast.Annotation{
		Base: spanFrom(start.Span.Start, end), Name: name.Lexeme, NameSpan: name.Span, Arguments: arguments,
		Comments: comments,
	}, nil
}

func (p *parser) parseDirective() (ast.Statement, error) {
	start := p.advance()
	var value ast.Expression
	var err error
	if start.Type != token.Tool {
		value, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
	}
	var extends ast.Expression
	var extendsSpan token.Span
	if start.Type == token.ClassName && p.at(token.Extends) {
		extendsSpan = p.advance().Span
		extends, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
	}
	end := start.Span.End
	if value != nil {
		end = value.Span().End
	}
	if extends != nil {
		end = extends.Span().End
	}
	return &ast.Directive{
		Base: spanFrom(start.Span.Start, end), Name: start.Lexeme, KeywordSpan: start.Span,
		Value: value, Extends: extends, ExtendsSpan: extendsSpan,
	}, nil
}

func (p *parser) parseVariable() (ast.Statement, error) {
	keyword := p.advance()
	return p.parseVariableAfter(keyword, token.Token{}, keyword.Type == token.Const)
}

func (p *parser) parseVariableAfter(keyword, static token.Token, constant bool) (ast.Statement, error) {
	name, err := p.expectName("expected variable name")
	if err != nil {
		return nil, err
	}
	start := keyword.Span.Start
	if static.Type == token.Static {
		start = static.Span.Start
	}
	declaration := &ast.VariableDeclaration{
		Base: spanFrom(start, name.Span.End), Name: name.Lexeme, NameSpan: name.Span,
		Constant: constant, Static: static.Type == token.Static, StaticSpan: static.Span, KeywordSpan: keyword.Span,
	}
	// A colon that ends the line opens the property accessor block rather than
	// introducing a type, which is how an untyped property is written.
	if p.at(token.Colon) && !p.accessorBlockFollows() {
		p.advance()
		declaration.Type, declaration.TypeSpan = p.parseTypeUntil(token.Assign, token.InferAssign, token.Colon, token.Newline, token.Comment)
		if declaration.Type == "" {
			return nil, p.error(p.peek(), "expected type after ':'")
		}
	}
	if p.at(token.InferAssign) {
		operator := p.advance()
		declaration.Inferred = true
		declaration.OperatorSpan = operator.Span
		declaration.Value, err = p.parseExpression(0)
	} else if p.at(token.Assign) {
		operator := p.advance()
		declaration.OperatorSpan = operator.Span
		declaration.Value, err = p.parseExpression(0)
	}
	if err != nil {
		return nil, err
	}
	hasAccessors := false
	if p.at(token.Colon) {
		if err := p.parsePropertyAccessors(declaration); err != nil {
			return nil, err
		}
		hasAccessors = true
	}
	if hasAccessors {
		// parsePropertyAccessors extends the declaration through the accessor block.
	} else if declaration.Value != nil {
		declaration.SourceSpan.End = declaration.Value.Span().End
	} else if p.current > 0 {
		declaration.SourceSpan.End = p.previous().Span.End
	}
	return declaration, nil
}

// accessorBlockFollows reports whether the colon the parser is sitting on ends
// its line, which makes it the colon of a property accessor block.
func (p *parser) accessorBlockFollows() bool {
	return p.peekN(1).Type == token.Newline || p.peekN(1).Type == token.Comment
}

func (p *parser) parsePropertyAccessors(declaration *ast.VariableDeclaration) error {
	if _, err := p.expect(token.Colon, "expected ':' before property accessors"); err != nil {
		return err
	}
	if _, err := p.expect(token.Newline, "expected newline before property accessors"); err != nil {
		return err
	}
	indent, err := p.expect(token.Indent, "expected indented property accessors")
	if err != nil {
		return err
	}
	_ = indent
	for !p.at(token.Dedent, token.EOF) {
		blankLines := p.takeBlankLines()
		for p.match(token.Newline) {
			blankLines++
		}
		if p.at(token.Dedent) {
			// Blank lines written at the end of the block belong to whatever follows
			// the statement that owns it.
			p.blankLines = blankLines
			break
		}
		if p.at(token.Comment) {
			p.advance()
			if _, err := p.expect(token.Newline, "expected end of comment"); err != nil {
				return err
			}
			continue
		}
		accessor, err := p.expectName("expected get or set accessor")
		if err != nil {
			return err
		}
		switch accessor.Lexeme {
		case "get":
			if p.match(token.LParen) {
				if _, err = p.expect(token.RParen, "expected ')' after get"); err != nil {
					return err
				}
			}
			var end token.Position
			declaration.Getter, end, err = p.parseSuite()
			declaration.GetterKeywordSpan = accessor.Span
			declaration.GetterSpan = token.Span{Start: accessor.Span.Start, End: componentEnd(declaration.Getter, end)}
		case "set":
			parameter := "value"
			var parameterSpan token.Span
			if p.match(token.LParen) {
				name, nameErr := p.expectName("expected setter parameter")
				if nameErr != nil {
					return nameErr
				}
				parameter = name.Lexeme
				parameterSpan = name.Span
				if _, nameErr = p.expect(token.RParen, "expected ')' after setter parameter"); nameErr != nil {
					return nameErr
				}
			}
			var body []ast.Statement
			var end token.Position
			body, end, err = p.parseSuite()
			declaration.Setter = &ast.PropertySetter{
				Base: spanFrom(accessor.Span.Start, componentEnd(body, end)), KeywordSpan: accessor.Span,
				Parameter: parameter, ParameterSpan: parameterSpan, Body: body,
			}
		default:
			return p.error(accessor, "expected get or set accessor")
		}
		if err != nil {
			return err
		}
	}
	end, err := p.expect(token.Dedent, "expected end of property accessors")
	if err != nil {
		return err
	}
	declaration.SourceSpan.End = end.Span.End
	return nil
}

func (p *parser) parseFunction(keyword, static token.Token) (ast.Statement, error) {
	name, err := p.expectName("expected function name")
	if err != nil {
		return nil, err
	}
	parameters, parameterComments, err := p.parseParameters(true)
	if err != nil {
		return nil, err
	}
	returnType := ""
	var returnArrowSpan, returnTypeSpan token.Span
	if p.at(token.Arrow) {
		returnArrowSpan = p.advance().Span
		returnType, returnTypeSpan = p.parseTypeUntil(token.Colon, token.Newline, token.Comment)
		if returnType == "" {
			return nil, p.error(p.peek(), "expected return type after '->'")
		}
	}
	start := keyword.Span.Start
	if static.Type == token.Static {
		start = static.Span.Start
	}
	if !p.at(token.Colon) {
		return &ast.FunctionDeclaration{
			Base: spanFrom(start, p.previous().Span.End), Name: name.Lexeme, NameSpan: name.Span, Parameters: parameters,
			ReturnType: returnType, ReturnTypeSpan: returnTypeSpan, ReturnArrowSpan: returnArrowSpan,
			Static: static.Type == token.Static, StaticSpan: static.Span, KeywordSpan: keyword.Span, Abstract: true,
			ParameterComments: parameterComments,
		}, nil
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	return &ast.FunctionDeclaration{
		Base: spanFrom(start, end), Name: name.Lexeme, NameSpan: name.Span, Parameters: parameters,
		ReturnType: returnType, ReturnTypeSpan: returnTypeSpan, ReturnArrowSpan: returnArrowSpan,
		Static: static.Type == token.Static, StaticSpan: static.Span, KeywordSpan: keyword.Span, Body: body,
		ParameterComments: parameterComments,
	}, nil
}

// parseParameters parses a parenthesized parameter list. A rest parameter is
// written "...name" and may only be the last parameter, so variadic reports
// whether the construct accepts one at all: a function and a lambda do, a
// signal does not.
func (p *parser) parseParameters(variadic bool) ([]ast.Parameter, []ast.CollectionComment, error) {
	if _, err := p.expect(token.LParen, "expected '('"); err != nil {
		return nil, nil, err
	}
	var parameters []ast.Parameter
	var comments []ast.CollectionComment
	p.takeCollectionComments(&comments, 0)
	if !p.at(token.RParen) {
		for {
			if len(parameters) > 0 && parameters[len(parameters)-1].Variadic {
				return nil, nil, p.error(p.peek(), "no parameter may follow a rest parameter")
			}
			var rest token.Token
			if variadic && p.at(token.Ellipsis) {
				rest = p.advance()
				// A comment may sit between the dots and the name, as it may in
				// every other place a parameter list breaks across lines.
				p.takeCollectionComments(&comments, len(parameters))
			}
			name, err := p.expectName("expected parameter name")
			if err != nil {
				return nil, nil, err
			}
			parameter := ast.Parameter{Base: base(name.Span), Name: name.Lexeme, NameSpan: name.Span}
			if rest.Type == token.Ellipsis {
				parameter.Variadic = true
				parameter.VariadicSpan = rest.Span
				parameter.SourceSpan.Start = rest.Span.Start
			}
			if p.match(token.Colon) {
				parameter.Type, parameter.TypeSpan = p.parseTypeUntil(token.Assign, token.InferAssign, token.Comma, token.RParen)
				if parameter.Type == "" {
					return nil, nil, p.error(p.peek(), "expected parameter type")
				}
				parameter.SourceSpan.End = parameter.TypeSpan.End
			}
			if p.at(token.Assign, token.InferAssign) {
				operator := p.advance()
				if parameter.Variadic {
					return nil, nil, p.error(operator, "a rest parameter cannot have a default value")
				}
				parameter.DefaultOperatorSpan = operator.Span
				parameter.Default, err = p.parseExpression(0)
				if err != nil {
					return nil, nil, err
				}
				parameter.SourceSpan.End = parameter.Default.Span().End
			}
			parameters = append(parameters, parameter)
			p.takeCollectionComments(&comments, len(parameters))
			if !p.match(token.Comma) {
				break
			}
			p.takeCollectionComments(&comments, len(parameters))
			if p.at(token.RParen) {
				break
			}
		}
	}
	if _, err := p.expect(token.RParen, "expected ')' after parameters"); err != nil {
		return nil, nil, err
	}
	return parameters, comments, nil
}

func (p *parser) parseClass() (ast.Statement, error) {
	start := p.advance()
	name, err := p.expectName("expected class name")
	if err != nil {
		return nil, err
	}
	extends := ""
	var extendsSpan, baseTypeSpan token.Span
	if p.at(token.Extends) {
		extendsSpan = p.advance().Span
		extends, baseTypeSpan = p.parseTypeUntil(token.Colon)
		if extends == "" {
			return nil, p.error(p.peek(), "expected base class")
		}
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	return &ast.ClassDeclaration{
		Base: spanFrom(start.Span.Start, end), Name: name.Lexeme, NameSpan: name.Span,
		Extends: extends, BaseTypeSpan: baseTypeSpan, ExtendsSpan: extendsSpan, KeywordSpan: start.Span, Body: body,
	}, nil
}

func (p *parser) parseSignal() (ast.Statement, error) {
	start := p.advance()
	name, err := p.expectName("expected signal name")
	if err != nil {
		return nil, err
	}
	var parameters []ast.Parameter
	var parameterComments []ast.CollectionComment
	if p.at(token.LParen) {
		parameters, parameterComments, err = p.parseParameters(false)
		if err != nil {
			return nil, err
		}
	}
	return &ast.SignalDeclaration{
		Base: spanFrom(start.Span.Start, p.previous().Span.End), Name: name.Lexeme, NameSpan: name.Span,
		KeywordSpan: start.Span, Parameters: parameters, ParameterComments: parameterComments,
	}, nil
}

func (p *parser) parseEnum() (ast.Statement, error) {
	start := p.advance()
	name := ""
	var nameSpan token.Span
	if p.at(token.Identifier) && p.peekN(1).Type == token.LBrace {
		nameToken := p.advance()
		name = nameToken.Lexeme
		nameSpan = nameToken.Span
	}
	if _, err := p.expect(token.LBrace, "expected '{' in enum declaration"); err != nil {
		return nil, err
	}
	var members []ast.EnumMember
	var comments []ast.CollectionComment
	for !p.at(token.RBrace) {
		if p.match(token.Comma) {
			continue
		}
		if p.at(token.Comment) {
			p.takeCollectionComments(&comments, len(members))
			continue
		}
		memberName, err := p.expectName("expected enum member")
		if err != nil {
			return nil, err
		}
		member := ast.EnumMember{Base: base(memberName.Span), Name: memberName.Lexeme, NameSpan: memberName.Span}
		if p.at(token.Assign) {
			operator := p.advance()
			member.OperatorSpan = operator.Span
			member.Value, err = p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			member.SourceSpan.End = member.Value.Span().End
		}
		members = append(members, member)
		p.takeCollectionComments(&comments, len(members))
		if !p.match(token.Comma) {
			break
		}
		if p.at(token.RBrace) {
			break
		}
	}
	end, err := p.expect(token.RBrace, "expected '}' after enum")
	if err != nil {
		return nil, err
	}
	return &ast.EnumDeclaration{
		Base: spanFrom(start.Span.Start, end.Span.End), Name: name, NameSpan: nameSpan,
		KeywordSpan: start.Span, Members: members, Comments: comments,
	}, nil
}

func (p *parser) parseIf() (ast.Statement, error) {
	start := p.advance()
	condition, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	statement := &ast.IfStatement{
		Base: spanFrom(start.Span.Start, end),
		Branches: []ast.Branch{{
			Base: spanFrom(start.Span.Start, componentEnd(body, end)), KeywordSpan: start.Span, Condition: condition, Body: body,
		}},
	}
	p.match(token.Newline)
	for p.at(token.Elif) {
		keyword := p.advance()
		condition, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		body, end, err = p.parseSuite()
		if err != nil {
			return nil, err
		}
		statement.Branches = append(statement.Branches, ast.Branch{
			Base: spanFrom(keyword.Span.Start, componentEnd(body, end)), KeywordSpan: keyword.Span, Condition: condition, Body: body,
		})
		statement.SourceSpan.End = end
		p.match(token.Newline)
	}
	if p.at(token.Else) {
		keyword := p.advance()
		statement.Else, end, err = p.parseSuite()
		if err != nil {
			return nil, err
		}
		statement.ElseKeywordSpan = keyword.Span
		statement.ElseSpan = token.Span{Start: keyword.Span.Start, End: componentEnd(statement.Else, end)}
		statement.SourceSpan.End = end
	}
	return statement, nil
}

func (p *parser) parseWhile() (ast.Statement, error) {
	start := p.advance()
	condition, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	return &ast.WhileStatement{
		Base: spanFrom(start.Span.Start, end), KeywordSpan: start.Span, Condition: condition, Body: body,
	}, nil
}

func (p *parser) parseFor() (ast.Statement, error) {
	start := p.advance()
	variable, err := p.expectName("expected loop variable")
	if err != nil {
		return nil, err
	}
	typeName := ""
	var typeSpan token.Span
	if p.match(token.Colon) {
		typeName, typeSpan = p.parseTypeUntil(token.In)
		if typeName == "" {
			return nil, p.error(p.peek(), "expected loop variable type")
		}
	}
	in, err := p.expect(token.In, "expected 'in' after loop variable")
	if err != nil {
		return nil, err
	}
	iterable, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	return &ast.ForStatement{
		Base: spanFrom(start.Span.Start, end), KeywordSpan: start.Span,
		Variable: variable.Lexeme, VariableSpan: variable.Span, Type: typeName, TypeSpan: typeSpan,
		InSpan: in.Span, Iterable: iterable, Body: body,
	}, nil
}

func (p *parser) parseMatch() (ast.Statement, error) {
	start := p.advance()
	value, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Colon, "expected ':' after match expression"); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Newline, "expected newline after match expression"); err != nil {
		return nil, err
	}
	for {
		for p.match(token.Newline) {
		}
		if !p.at(token.Comment) {
			break
		}
		p.advance() // Match-level comments are currently trivia.
		if _, err := p.expect(token.Newline, "expected end of comment line"); err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(token.Indent, "expected indented match cases"); err != nil {
		return nil, err
	}
	var cases []ast.MatchCase
	for !p.at(token.Dedent, token.EOF) {
		blankLines := p.takeBlankLines()
		for p.match(token.Newline) {
			blankLines++
		}
		if p.at(token.Dedent) {
			// Blank lines written at the end of the block belong to whatever follows
			// the statement that owns it.
			p.blankLines = blankLines
			break
		}
		if p.at(token.Comment) {
			p.advance()
			p.match(token.Newline)
			continue
		}
		matchCase := ast.MatchCase{}
		for {
			pattern, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			matchCase.Patterns = append(matchCase.Patterns, pattern)
			if len(matchCase.Patterns) == 1 {
				matchCase.SourceSpan.Start = pattern.Span().Start
			}
			if !p.match(token.Comma) {
				break
			}
		}
		if p.at(token.Identifier) && p.peek().Lexeme == "when" {
			matchCase.WhenSpan = p.advance().Span
			matchCase.Guard, err = p.parseExpression(0)
			if err != nil {
				return nil, err
			}
		}
		var caseEnd token.Position
		matchCase.Body, caseEnd, err = p.parseSuite()
		if err != nil {
			return nil, err
		}
		matchCase.SourceSpan.End = componentEnd(matchCase.Body, caseEnd)
		cases = append(cases, matchCase)
	}
	end, err := p.expect(token.Dedent, "expected end of match block")
	if err != nil {
		return nil, err
	}
	return &ast.MatchStatement{
		Base: spanFrom(start.Span.Start, end.Span.End), KeywordSpan: start.Span, Value: value, Cases: cases,
	}, nil
}

func (p *parser) parseReturn() (ast.Statement, error) {
	start := p.advance()
	statement := &ast.ReturnStatement{Base: base(start.Span), KeywordSpan: start.Span}
	if !p.at(token.Newline, token.Comment) {
		value, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		statement.Value = value
		statement.SourceSpan.End = value.Span().End
	}
	return statement, nil
}

func (p *parser) parseExpressionStatement() (ast.Statement, error) {
	expression, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	if isAssignment(p.peek().Type) {
		operator := p.advance()
		value, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		return &ast.Assignment{
			Base: spanFrom(expression.Span().Start, value.Span().End), Target: expression,
			Operator: operator.Lexeme, OperatorSpan: operator.Span, Value: value,
		}, nil
	}
	return &ast.ExpressionStatement{Base: base(expression.Span()), Expression: expression}, nil
}

func isAssignment(typ token.Type) bool {
	switch typ {
	case token.Assign, token.PlusAssign, token.MinusAssign, token.StarAssign, token.SlashAssign,
		token.PercentAssign, token.AmpAssign, token.PipeAssign, token.CaretAssign,
		token.ShiftLeftAssign, token.ShiftRightAssign:
		return true
	default:
		return false
	}
}

func (p *parser) parseTypeUntil(stops ...token.Type) (string, token.Span) {
	stop := make(map[token.Type]bool, len(stops))
	for _, typ := range stops {
		stop[typ] = true
	}
	var out strings.Builder
	var sourceSpan token.Span
	depth := 0
	for !p.at(token.EOF) {
		tok := p.peek()
		if depth == 0 && stop[tok.Type] {
			break
		}
		switch tok.Type {
		case token.LBracket:
			depth++
		case token.RBracket:
			if depth == 0 {
				return out.String(), sourceSpan
			}
			depth--
		}
		p.advance()
		if sourceSpan == (token.Span{}) {
			sourceSpan.Start = tok.Span.Start
		}
		sourceSpan.End = tok.Span.End
		if tok.Type == token.Comma {
			out.WriteString(", ")
		} else {
			out.WriteString(tok.Lexeme)
		}
	}
	return out.String(), sourceSpan
}

func componentEnd(body []ast.Statement, fallback token.Position) token.Position {
	for i := len(body) - 1; i >= 0; i-- {
		if body[i] != nil {
			return body[i].Span().End
		}
	}
	return fallback
}
