package parser

import (
	"strings"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/token"
)

func (p *parser) parseAnnotation() (ast.Statement, error) {
	start := p.advance()
	name, err := p.expectAnnotationName(start)
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
	value, err := p.parseExpression(0)
	if err != nil {
		return nil, err
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
	name, err := p.expectIdentifier("expected variable name")
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
		switch {
		case p.peekN(1).Type == token.Assign:
			// "var x: = 1" leaves the type to the value, exactly as ":=" does.
			// Godot has no ":=" token at all: it reads the colon and the "="
			// separately, so the two spellings are the same declaration.
			p.advance()
			declaration.Inferred = true
		case p.accessorNameFollows():
			// Shorthand accessors may be written on the declaration's own line,
			// where no type and no initializer may follow them.
			if err := p.parseAccessors(declaration, false); err != nil {
				return nil, err
			}
			return declaration, nil
		default:
			p.advance()
			declaration.Type, declaration.TypeSpan, err = p.parseTypeUntil(token.Assign, token.InferAssign, token.Colon, token.Newline)
			if err != nil {
				return nil, err
			}
			if declaration.Type == "" {
				return nil, p.error(p.peek(), "expected type after ':'")
			}
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
		if err := p.parseAccessors(declaration, p.accessorBlockFollows()); err != nil {
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

// accessorNameFollows reports whether the colon the parser is sitting on is
// followed by "get" or "set". Godot looks for the two names before it reads a
// type, so a type could not be named either of them anyway.
func (p *parser) accessorNameFollows() bool {
	next := p.peekN(1)
	return next.Type == token.Identifier && (next.Lexeme == "get" || next.Lexeme == "set")
}

// parseAccessors parses the property accessors introduced by a declaration's
// colon, which has not been consumed. block says the accessors are written in an
// indented block, which an accessor carrying a body needs and a shorthand
// accessor may do without.
func (p *parser) parseAccessors(declaration *ast.VariableDeclaration, block bool) error {
	if _, err := p.expect(token.Colon, "expected ':' before property accessors"); err != nil {
		return err
	}
	declaration.AccessorBlock = block
	if !block {
		for {
			if err := p.parseAccessor(declaration, false); err != nil {
				return err
			}
			if !p.match(token.Comma) {
				break
			}
		}
		declaration.SourceSpan.End = p.previous().Span.End
		return nil
	}
	// A comment may end the colon's line, and more may sit inside the block
	// outside an accessor body. None of them belongs to an accessor's suite, so
	// they are held by the declaration.
	var comments []ast.CollectionComment
	p.takeCollectionComments(&comments, 0)
	defer func() { declaration.AccessorComments = comments }()
	if _, err := p.expect(token.Newline, "expected newline before property accessors"); err != nil {
		return err
	}
	// A comment written at the declaration's own level sits ahead of the block
	// it opens, since a comment-only line does not open one.
	for {
		for p.match(token.Newline) {
		}
		if !p.at(token.Comment) {
			break
		}
		p.takeCollectionComments(&comments, 0)
		if _, err := p.expect(token.Newline, "expected end of comment"); err != nil {
			return err
		}
	}
	if _, err := p.expect(token.Indent, "expected indented property accessors"); err != nil {
		return err
	}
	for !p.at(token.Dedent, token.EOF) {
		blankLines := p.takeBlankLines()
		for p.match(token.Newline) {
			blankLines++
		}
		if p.at(token.Dedent) {
			// Blank lines written at the end of the block belong to whatever
			// follows the statement that owns it.
			p.blankLines = blankLines
			break
		}
		if p.at(token.Comment) {
			p.takeCollectionComments(&comments, p.accessorsWritten(declaration))
			if _, err := p.expect(token.Newline, "expected end of comment"); err != nil {
				return err
			}
			continue
		}
		if err := p.parseAccessor(declaration, true); err != nil {
			return err
		}
		if declaration.GetterName != "" || declaration.SetterName != "" {
			// A shorthand accessor stands on its line beside the other one, so
			// a comma may separate the two inside the block as well.
			if p.match(token.Comma) {
				p.match(token.Newline)
			}
		}
	}
	end, err := p.expect(token.Dedent, "expected end of property accessors")
	if err != nil {
		return err
	}
	declaration.SourceSpan.End = end.Span.End
	return nil
}

// accessorsWritten returns how many accessors the declaration has read so far,
// which is the index a comment inside the block is anchored to.
func (p *parser) accessorsWritten(declaration *ast.VariableDeclaration) int {
	written := 0
	for _, present := range []bool{
		declaration.Getter != nil || declaration.GetterName != "",
		declaration.Setter != nil || declaration.SetterName != "",
	} {
		if present {
			written++
		}
	}
	return written
}

// parseAccessor parses one property accessor. Godot decides from the first one
// whether the property names its methods with "=" or carries their bodies, and
// requires the rest to be written the same way.
func (p *parser) parseAccessor(declaration *ast.VariableDeclaration, block bool) error {
	accessor, err := p.expectIdentifier("expected get or set accessor")
	if err != nil {
		return err
	}
	if accessor.Lexeme != "get" && accessor.Lexeme != "set" {
		return p.error(accessor, "expected get or set accessor")
	}
	shorthand := p.at(token.Assign)
	written := p.accessorsWritten(declaration)
	if written > 0 && shorthand != (declaration.GetterName != "" || declaration.SetterName != "") {
		return p.error(accessor, "a property's accessors are either both named with '=' or both written with a body")
	}
	if !shorthand && !block {
		return p.error(p.peek(), "an accessor with a body needs an indented block")
	}
	if accessor.Lexeme == "get" {
		if declaration.GetterKeywordSpan != (token.Span{}) {
			return p.error(accessor, "a property may only have one getter")
		}
		declaration.GetterKeywordSpan = accessor.Span
	} else {
		if declaration.SetterKeywordSpan != (token.Span{}) {
			return p.error(accessor, "a property may only have one setter")
		}
		declaration.SetterKeywordSpan = accessor.Span
	}
	if shorthand {
		p.advance()
		name, nameErr := p.expectIdentifier("expected accessor method name after '='")
		if nameErr != nil {
			return nameErr
		}
		if accessor.Lexeme == "get" {
			declaration.GetterName, declaration.GetterNameSpan = name.Lexeme, name.Span
		} else {
			declaration.SetterName, declaration.SetterNameSpan = name.Lexeme, name.Span
		}
		return nil
	}
	if accessor.Lexeme == "get" {
		if p.match(token.LParen) {
			if _, err = p.expect(token.RParen, "expected ')' after get"); err != nil {
				return err
			}
		}
		var end token.Position
		declaration.Getter, end, err = p.parseSuite()
		if err != nil {
			return err
		}
		declaration.GetterSpan = token.Span{Start: accessor.Span.Start, End: componentEnd(declaration.Getter, end)}
		return nil
	}
	parameter := "value"
	var parameterSpan token.Span
	if p.match(token.LParen) {
		name, nameErr := p.expectIdentifier("expected setter parameter")
		if nameErr != nil {
			return nameErr
		}
		parameter = name.Lexeme
		parameterSpan = name.Span
		if _, nameErr = p.expect(token.RParen, "expected ')' after setter parameter"); nameErr != nil {
			return nameErr
		}
	}
	body, end, err := p.parseSuite()
	if err != nil {
		return err
	}
	declaration.Setter = &ast.PropertySetter{
		Base: spanFrom(accessor.Span.Start, componentEnd(body, end)), KeywordSpan: accessor.Span,
		Parameter: parameter, ParameterSpan: parameterSpan, Body: body,
	}
	return nil
}

func (p *parser) parseFunction(keyword, static token.Token) (ast.Statement, error) {
	name, err := p.expectIdentifier("expected function name")
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
		returnType, returnTypeSpan, err = p.parseTypeUntil(token.Colon, token.Newline)
		if err != nil {
			return nil, err
		}
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
			name, err := p.expectIdentifier("expected parameter name")
			if err != nil {
				return nil, nil, err
			}
			parameter := ast.Parameter{Base: base(name.Span), Name: name.Lexeme, NameSpan: name.Span}
			// A parameter list is written across lines, so a comment may break
			// the parameter anywhere it continues: before its type, inside a
			// dotted type name, or before its default value. Such a comment is
			// held until the parameter it interrupts has an index, and then
			// anchored after it, which keeps it on that parameter's line.
			var interrupting []ast.CollectionComment
			p.takeCollectionComments(&interrupting, 0)
			if rest.Type == token.Ellipsis {
				parameter.Variadic = true
				parameter.VariadicSpan = rest.Span
				parameter.SourceSpan.Start = rest.Span.Start
			}
			if p.match(token.Colon) {
				p.takeCollectionComments(&interrupting, 0)
				parameter.Type, parameter.TypeSpan, err = p.parseTypeInto(
					&interrupting, 0,
					token.Assign, token.InferAssign, token.Comma, token.RParen,
				)
				if err != nil {
					return nil, nil, err
				}
				if parameter.Type == "" {
					return nil, nil, p.error(p.peek(), "expected parameter type")
				}
				parameter.SourceSpan.End = parameter.TypeSpan.End
				if next := p.peekPastComments().Type; next == token.Assign || next == token.InferAssign {
					p.takeCollectionComments(&interrupting, 0)
				}
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
			for _, comment := range interrupting {
				comment.Index = len(parameters)
				comments = append(comments, comment)
			}
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
	name, err := p.expectIdentifier("expected class name")
	if err != nil {
		return nil, err
	}
	extends := ""
	var extendsSpan, baseTypeSpan token.Span
	if p.at(token.Extends) {
		extendsSpan = p.advance().Span
		extends, baseTypeSpan, err = p.parseTypeUntil(token.Colon)
		if err != nil {
			return nil, err
		}
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
	name, err := p.expectIdentifier("expected signal name")
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
	if token.IsIdentifier(p.peek().Type) && p.peekN(1).Type == token.LBrace {
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
		name, err := p.expectIdentifier("expected enum member")
		if err != nil {
			return nil, err
		}
		member := ast.EnumMember{Base: base(name.Span), Name: name.Lexeme, NameSpan: name.Span}
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
		p.dropBlankLines()
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
		p.dropBlankLines()
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
	variable, err := p.expectIdentifier("expected loop variable")
	if err != nil {
		return nil, err
	}
	typeName := ""
	var typeSpan token.Span
	if p.match(token.Colon) {
		typeName, typeSpan, err = p.parseTypeUntil(token.In)
		if err != nil {
			return nil, err
		}
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
	// A comment may end the "match" line, and more may sit between it and the
	// first case. None of them belongs to a case body, so they are held by the
	// statement, anchored to the case they precede.
	var comments []ast.CollectionComment
	p.takeCollectionComments(&comments, 0)
	if _, err := p.expect(token.Newline, "expected newline after match expression"); err != nil {
		return nil, err
	}
	for {
		for p.match(token.Newline) {
		}
		if !p.at(token.Comment) {
			break
		}
		p.takeCollectionComments(&comments, 0)
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
			p.takeCollectionComments(&comments, len(cases))
			p.match(token.Newline)
			continue
		}
		if p.at(token.Pass) {
			// A bare "pass" stands for a match that handles nothing. Godot
			// accepts it beside real branches too, so it is kept as a case
			// holding no pattern rather than discarded.
			keyword := p.advance()
			cases = append(cases, ast.MatchCase{Base: base(keyword.Span)})
			if _, err := p.expect(token.Newline, "expected end of line after pass"); err != nil {
				return nil, err
			}
			continue
		}
		matchCase := ast.MatchCase{}
		var comma token.Token
		for {
			pattern, err := p.parsePattern()
			if err != nil {
				return nil, err
			}
			if _, isRest := pattern.(*ast.RestPattern); isRest {
				return nil, p.error(p.previous(), "the '..' pattern is only allowed inside an array or dictionary pattern")
			}
			matchCase.Patterns = append(matchCase.Patterns, pattern)
			if len(matchCase.Patterns) == 1 {
				matchCase.SourceSpan.Start = pattern.Span().Start
			}
			if !p.at(token.Comma) {
				break
			}
			comma = p.advance()
		}
		// A branch that binds the matched value may hold no other pattern,
		// because the other patterns would leave the name unbound. The rule
		// covers a bind at any depth, so a bind inside an array or dictionary
		// pattern counts as well.
		if len(matchCase.Patterns) > 1 {
			for _, pattern := range matchCase.Patterns {
				if bindsValue(pattern) {
					return nil, p.error(comma, "a variable bind may not be combined with another pattern")
				}
			}
		}
		if p.at(token.When) {
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
		Comments: comments,
	}, nil
}

// parseAssert parses assert(condition) or assert(condition, message), which
// Godot reads as a statement rather than as a call to a function of that name.
// It is kept as a call expression all the same, because that is how it is
// written and how it is formatted, and because no function may carry the name.
func (p *parser) parseAssert() (ast.Statement, error) {
	keyword := p.advance()
	if !p.at(token.LParen) {
		return nil, p.error(p.peek(), "expected '(' after assert")
	}
	p.advance()
	arguments, comments, end, err := p.parseArguments()
	if err != nil {
		return nil, err
	}
	if len(arguments) == 0 || len(arguments) > 2 {
		return nil, p.error(keyword, "assert takes a condition and an optional message")
	}
	call := &ast.CallExpression{
		Base:      spanFrom(keyword.Span.Start, end.Span.End),
		Callee:    &ast.Identifier{Base: base(keyword.Span), Name: keyword.Lexeme},
		Arguments: arguments, Comments: comments,
	}
	return &ast.ExpressionStatement{Base: base(call.Span()), Expression: call}, nil
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

// parseTypeUntil reads a type that has nowhere to put a comment, so a comment
// always ends it.
func (p *parser) parseTypeUntil(stops ...token.Type) (string, token.Span, error) {
	return p.parseTypeInto(nil, 0, stops...)
}

// parseTypeInto reads a type, stopping before the first of stops it finds
// outside the type's brackets. A comment ends the type, so that it stays a
// comment rather than being read as more type text, except that a type broken
// after one of its dots continues past the comment: comments then holds it,
// anchored to the item at index. Inside the type's brackets, where the rest of
// the type would follow the comment onto the next line, a comment is an error,
// as it is for Godot.
func (p *parser) parseTypeInto(comments *[]ast.CollectionComment, index int, stops ...token.Type) (string, token.Span, error) {
	stop := make(map[token.Type]bool, len(stops))
	for _, typ := range stops {
		stop[typ] = true
	}
	var out strings.Builder
	var sourceSpan token.Span
	depth := 0
	for !p.at(token.EOF) {
		tok := p.peek()
		if tok.Type == token.Comment {
			if depth > 0 {
				return "", token.Span{}, p.error(tok, "a type argument list is written on one line")
			}
			if comments != nil && strings.HasSuffix(out.String(), ".") {
				// The name continues after the dot, so the comment interrupts
				// the type rather than ending it.
				p.takeCollectionComments(comments, index)
				continue
			}
			break
		}
		if depth == 0 && stop[tok.Type] {
			break
		}
		if token.IsKeyword(tok.Type) && !token.IsIdentifier(tok.Type) && tok.Type != token.Void {
			// A type is built from names, dots, brackets and commas, so a
			// reserved keyword here is not a type Godot would accept. The one
			// exception is "void", which names the absence of a return value.
			return "", token.Span{}, p.error(tok, "expected type name")
		}
		switch tok.Type {
		case token.LBracket:
			depth++
		case token.RBracket:
			if depth == 0 {
				return out.String(), sourceSpan, nil
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
	return out.String(), sourceSpan, nil
}

func componentEnd(body []ast.Statement, fallback token.Position) token.Position {
	for i := len(body) - 1; i >= 0; i-- {
		if body[i] != nil {
			return body[i].Span().End
		}
	}
	return fallback
}

// bindsValue reports whether pattern binds the matched value, at any depth
// inside an array or dictionary pattern.
func bindsValue(pattern ast.Expression) bool {
	bound := false
	ast.Inspect(pattern, func(node ast.Node) bool {
		if _, ok := node.(*ast.BindingPattern); ok {
			bound = true
		}
		return !bound
	})
	return bound
}
