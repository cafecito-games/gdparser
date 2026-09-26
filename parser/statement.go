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
	end := name.Span.End
	if p.match(token.LParen) {
		if !p.at(token.RParen) {
			for {
				argument, err := p.parseExpression(0)
				if err != nil {
					return nil, err
				}
				arguments = append(arguments, argument)
				if !p.match(token.Comma) {
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
	return &ast.Annotation{Base: spanFrom(start.Span.Start, end), Name: name.Lexeme, Arguments: arguments}, nil
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
	if start.Type == token.ClassName && p.match(token.Extends) {
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
	return &ast.Directive{Base: spanFrom(start.Span.Start, end), Name: start.Lexeme, Value: value, Extends: extends}, nil
}

func (p *parser) parseVariable(static bool) (ast.Statement, error) {
	start := p.advance()
	return p.parseVariableAfter(start, start.Type == token.Const, static)
}

func (p *parser) parseVariableAfter(start token.Token, constant, static bool) (ast.Statement, error) {
	name, err := p.expectName("expected variable name")
	if err != nil {
		return nil, err
	}
	declaration := &ast.VariableDeclaration{
		Base: spanFrom(start.Span.Start, name.Span.End), Name: name.Lexeme, Constant: constant, Static: static,
	}
	if p.match(token.Colon) {
		declaration.Type = p.parseTypeUntil(token.Assign, token.InferAssign, token.Colon, token.Newline, token.Comment)
		if declaration.Type == "" {
			return nil, p.error(p.peek(), "expected type after ':'")
		}
	}
	if p.match(token.InferAssign) {
		declaration.Inferred = true
		declaration.Value, err = p.parseExpression(0)
	} else if p.match(token.Assign) {
		declaration.Value, err = p.parseExpression(0)
	}
	if err != nil {
		return nil, err
	}
	if p.at(token.Colon) {
		if err := p.parsePropertyAccessors(declaration); err != nil {
			return nil, err
		}
	}
	if declaration.Value != nil {
		declaration.SourceSpan.End = declaration.Value.Span().End
	} else if p.current > 0 {
		declaration.SourceSpan.End = p.previous().Span.End
	}
	return declaration, nil
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
		for p.match(token.Newline) {
		}
		if p.at(token.Dedent) {
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
			declaration.Getter, _, err = p.parseSuite()
		case "set":
			parameter := "value"
			if p.match(token.LParen) {
				name, nameErr := p.expectName("expected setter parameter")
				if nameErr != nil {
					return nameErr
				}
				parameter = name.Lexeme
				if _, nameErr = p.expect(token.RParen, "expected ')' after setter parameter"); nameErr != nil {
					return nameErr
				}
			}
			var body []ast.Statement
			body, _, err = p.parseSuite()
			declaration.Setter = &ast.PropertySetter{Parameter: parameter, Body: body}
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

func (p *parser) parseFunction(start token.Token, static bool) (ast.Statement, error) {
	name, err := p.expectName("expected function name")
	if err != nil {
		return nil, err
	}
	parameters, err := p.parseParameters()
	if err != nil {
		return nil, err
	}
	returnType := ""
	if p.match(token.Arrow) {
		returnType = p.parseTypeUntil(token.Colon, token.Newline, token.Comment)
		if returnType == "" {
			return nil, p.error(p.peek(), "expected return type after '->'")
		}
	}
	if !p.at(token.Colon) {
		return &ast.FunctionDeclaration{
			Base: spanFrom(start.Span.Start, p.previous().Span.End), Name: name.Lexeme, Parameters: parameters,
			ReturnType: returnType, Static: static, Abstract: true,
		}, nil
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	return &ast.FunctionDeclaration{
		Base: spanFrom(start.Span.Start, end), Name: name.Lexeme, Parameters: parameters,
		ReturnType: returnType, Static: static, Body: body,
	}, nil
}

func (p *parser) parseParameters() ([]ast.Parameter, error) {
	if _, err := p.expect(token.LParen, "expected '('"); err != nil {
		return nil, err
	}
	var parameters []ast.Parameter
	if !p.at(token.RParen) {
		for {
			name, err := p.expectName("expected parameter name")
			if err != nil {
				return nil, err
			}
			parameter := ast.Parameter{Name: name.Lexeme}
			if p.match(token.Colon) {
				parameter.Type = p.parseTypeUntil(token.Assign, token.InferAssign, token.Comma, token.RParen)
				if parameter.Type == "" {
					return nil, p.error(p.peek(), "expected parameter type")
				}
			}
			if p.match(token.Assign, token.InferAssign) {
				parameter.Default, err = p.parseExpression(0)
				if err != nil {
					return nil, err
				}
			}
			parameters = append(parameters, parameter)
			if !p.match(token.Comma) {
				break
			}
			if p.at(token.RParen) {
				break
			}
		}
	}
	if _, err := p.expect(token.RParen, "expected ')' after parameters"); err != nil {
		return nil, err
	}
	return parameters, nil
}

func (p *parser) parseClass() (ast.Statement, error) {
	start := p.advance()
	name, err := p.expectName("expected class name")
	if err != nil {
		return nil, err
	}
	extends := ""
	if p.match(token.Extends) {
		extends = p.parseTypeUntil(token.Colon)
		if extends == "" {
			return nil, p.error(p.peek(), "expected base class")
		}
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return nil, err
	}
	return &ast.ClassDeclaration{Base: spanFrom(start.Span.Start, end), Name: name.Lexeme, Extends: extends, Body: body}, nil
}

func (p *parser) parseSignal() (ast.Statement, error) {
	start := p.advance()
	name, err := p.expectName("expected signal name")
	if err != nil {
		return nil, err
	}
	var parameters []ast.Parameter
	if p.at(token.LParen) {
		parameters, err = p.parseParameters()
		if err != nil {
			return nil, err
		}
	}
	return &ast.SignalDeclaration{Base: spanFrom(start.Span.Start, p.previous().Span.End), Name: name.Lexeme, Parameters: parameters}, nil
}

func (p *parser) parseEnum() (ast.Statement, error) {
	start := p.advance()
	name := ""
	if p.at(token.Identifier) && p.peekN(1).Type == token.LBrace {
		name = p.advance().Lexeme
	}
	if _, err := p.expect(token.LBrace, "expected '{' in enum declaration"); err != nil {
		return nil, err
	}
	var members []ast.EnumMember
	var pendingComments []*ast.Comment
	for !p.at(token.RBrace) {
		if p.match(token.Comma) {
			continue
		}
		if p.at(token.Comment) {
			pendingComments = append(pendingComments, commentNode(p.advance()))
			continue
		}
		memberName, err := p.expectName("expected enum member")
		if err != nil {
			return nil, err
		}
		member := ast.EnumMember{Name: memberName.Lexeme, Comments: pendingComments}
		pendingComments = nil
		if p.match(token.Assign) {
			member.Value, err = p.parseExpression(0)
			if err != nil {
				return nil, err
			}
		}
		members = append(members, member)
		for p.at(token.Comment) {
			pendingComments = append(pendingComments, commentNode(p.advance()))
		}
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
	return &ast.EnumDeclaration{Base: spanFrom(start.Span.Start, end.Span.End), Name: name, Members: members}, nil
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
	statement := &ast.IfStatement{Base: spanFrom(start.Span.Start, end), Branches: []ast.Branch{{Condition: condition, Body: body}}}
	p.match(token.Newline)
	for p.match(token.Elif) {
		condition, err = p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		body, end, err = p.parseSuite()
		if err != nil {
			return nil, err
		}
		statement.Branches = append(statement.Branches, ast.Branch{Condition: condition, Body: body})
		statement.SourceSpan.End = end
		p.match(token.Newline)
	}
	if p.match(token.Else) {
		statement.Else, end, err = p.parseSuite()
		if err != nil {
			return nil, err
		}
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
	return &ast.WhileStatement{Base: spanFrom(start.Span.Start, end), Condition: condition, Body: body}, nil
}

func (p *parser) parseFor() (ast.Statement, error) {
	start := p.advance()
	variable, err := p.expectName("expected loop variable")
	if err != nil {
		return nil, err
	}
	typeName := ""
	if p.match(token.Colon) {
		typeName = p.parseTypeUntil(token.In)
		if typeName == "" {
			return nil, p.error(p.peek(), "expected loop variable type")
		}
	}
	if _, err := p.expect(token.In, "expected 'in' after loop variable"); err != nil {
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
	return &ast.ForStatement{Base: spanFrom(start.Span.Start, end), Variable: variable.Lexeme, Type: typeName, Iterable: iterable, Body: body}, nil
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
		for p.match(token.Newline) {
		}
		if p.at(token.Dedent) {
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
			if !p.match(token.Comma) {
				break
			}
		}
		if p.at(token.Identifier) && p.peek().Lexeme == "when" {
			p.advance()
			matchCase.Guard, err = p.parseExpression(0)
			if err != nil {
				return nil, err
			}
		}
		matchCase.Body, _, err = p.parseSuite()
		if err != nil {
			return nil, err
		}
		cases = append(cases, matchCase)
	}
	end, err := p.expect(token.Dedent, "expected end of match block")
	if err != nil {
		return nil, err
	}
	return &ast.MatchStatement{Base: spanFrom(start.Span.Start, end.Span.End), Value: value, Cases: cases}, nil
}

func (p *parser) parseReturn() (ast.Statement, error) {
	start := p.advance()
	statement := &ast.ReturnStatement{Base: base(start.Span)}
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
		return &ast.Assignment{Base: spanFrom(expression.Span().Start, value.Span().End), Target: expression, Operator: operator.Lexeme, Value: value}, nil
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

func (p *parser) parseTypeUntil(stops ...token.Type) string {
	stop := make(map[token.Type]bool, len(stops))
	for _, typ := range stops {
		stop[typ] = true
	}
	var out strings.Builder
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
				return out.String()
			}
			depth--
		}
		p.advance()
		if tok.Type == token.Comma {
			out.WriteString(", ")
		} else {
			out.WriteString(tok.Lexeme)
		}
	}
	return out.String()
}
