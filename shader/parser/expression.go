package parser

import (
	"github.com/cafecito-games/gdparser/shader/ast"
	"github.com/cafecito-games/gdparser/shader/token"
)

func (p *parser) parseExpression() (ast.Expression, error) { return p.parsePrecedence(1) }

func (p *parser) parsePrecedence(min int) (ast.Expression, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	for {
		op := p.current().Text
		if op == "?" {
			if 2 < min {
				break
			}
			start := left.Span().Start
			p.advance()
			then, e := p.parsePrecedence(1)
			if e != nil {
				return nil, e
			}
			if _, e = p.expect(":"); e != nil {
				return nil, e
			}
			other, e := p.parsePrecedence(2)
			if e != nil {
				return nil, e
			}
			left = &ast.ConditionalExpression{Base: ast.Base{SourceSpan: token.Span{Start: start, End: other.Span().End}}, Condition: left, Then: then, Else: other}
			continue
		}
		prec, rightAssoc, assignment := infix(op)
		if prec < min {
			break
		}
		p.advance()
		next := prec + 1
		if rightAssoc {
			next = prec
		}
		right, e := p.parsePrecedence(next)
		if e != nil {
			return nil, e
		}
		span := token.Span{Start: left.Span().Start, End: right.Span().End}
		if assignment {
			left = &ast.AssignmentExpression{Base: ast.Base{SourceSpan: span}, Target: left, Operator: op, Value: right}
		} else {
			left = &ast.BinaryExpression{Base: ast.Base{SourceSpan: span}, Left: left, Operator: op, Right: right}
		}
	}
	return left, nil
}

func (p *parser) parsePrefix() (ast.Expression, error) {
	if p.current().Text == "+" || p.current().Text == "-" || p.current().Text == "!" || p.current().Text == "~" || p.current().Text == "++" || p.current().Text == "--" {
		start := p.advance()
		operand, err := p.parsePrecedence(14)
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpression{Base: ast.Base{SourceSpan: token.Span{Start: start.Span.Start, End: operand.Span().End}}, Operator: start.Text, Operand: operand}, nil
	}
	var expr ast.Expression
	t := p.current()
	switch t.Kind {
	case token.Number:
		p.advance()
		expr = &ast.Literal{Base: ast.Base{SourceSpan: t.Span}, Kind: ast.NumberLiteral, Raw: t.Text}
	case token.String:
		p.advance()
		expr = &ast.Literal{Base: ast.Base{SourceSpan: t.Span}, Kind: ast.StringLiteral, Raw: t.Text}
	case token.Identifier:
		p.advance()
		if t.Text == "true" || t.Text == "false" {
			expr = &ast.Literal{Base: ast.Base{SourceSpan: t.Span}, Kind: ast.BoolLiteral, Raw: t.Text}
		} else {
			expr = &ast.Identifier{Base: ast.Base{SourceSpan: t.Span}, Name: t.Text}
		}
	case token.Symbol:
		if t.Text == "{" {
			p.advance()
			list := &ast.InitializerList{}
			for !p.check("}") {
				if p.current().Kind == token.Comment {
					list.Items = append(list.Items, comment(p.advance()))
					continue
				}
				element, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				list.Items = append(list.Items, element.(ast.InitializerItem))
				if p.match(",") {
					continue
				}
				for p.current().Kind == token.Comment {
					list.Items = append(list.Items, comment(p.advance()))
				}
				if !p.check("}") {
					return nil, p.errorf(p.current(), "expected comma or closing brace in initializer")
				}
			}
			end, err := p.expect("}")
			if err != nil {
				return nil, err
			}
			list.SourceSpan = token.Span{Start: t.Span.Start, End: end.Span.End}
			expr = list
			break
		}
		if t.Text != "(" {
			return nil, p.errorf(t, "expected expression, found %q", t.Text)
		}
		p.advance()
		var err error
		expr, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(")"); err != nil {
			return nil, err
		}
	default:
		return nil, p.errorf(t, "expected expression, found %q", t.Text)
	}
	for {
		if p.match("(") {
			start := expr.Span().Start
			call := &ast.CallExpression{Callee: expr}
			if !p.check(")") {
				for {
					arg, err := p.parseExpression()
					if err != nil {
						return nil, err
					}
					call.Arguments = append(call.Arguments, arg)
					if !p.match(",") {
						break
					}
				}
			}
			end, err := p.expect(")")
			if err != nil {
				return nil, err
			}
			call.SourceSpan = token.Span{Start: start, End: end.Span.End}
			expr = call
			continue
		}
		if p.match("[") {
			start := expr.Span().Start
			index := &ast.IndexExpression{Object: expr}
			var err error
			if !p.check("]") {
				index.Index, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
			}
			end, err := p.expect("]")
			if err != nil {
				return nil, err
			}
			index.SourceSpan = token.Span{Start: start, End: end.Span.End}
			expr = index
			continue
		}
		if p.match(".") {
			property, err := p.expectIdent("member name")
			if err != nil {
				return nil, err
			}
			expr = &ast.MemberExpression{Base: ast.Base{SourceSpan: token.Span{Start: expr.Span().Start, End: property.Span.End}}, Object: expr, Property: property.Text}
			continue
		}
		if p.check("++") || p.check("--") {
			op := p.advance()
			expr = &ast.PostfixExpression{Base: ast.Base{SourceSpan: token.Span{Start: expr.Span().Start, End: op.Span.End}}, Operand: expr, Operator: op.Text}
			continue
		}
		break
	}
	return expr, nil
}

func infix(op string) (precedence int, rightAssociative, assignment bool) {
	switch op {
	case "=", "+=", "-=", "*=", "/=", "%=", "<<=", ">>=", "&=", "|=", "^=":
		return 1, true, true
	case "||":
		return 3, false, false
	case "^^":
		return 4, false, false
	case "&&":
		return 5, false, false
	case "|":
		return 6, false, false
	case "^":
		return 7, false, false
	case "&":
		return 8, false, false
	case "==", "!=":
		return 9, false, false
	case "<", "<=", ">", ">=":
		return 10, false, false
	case "<<", ">>":
		return 11, false, false
	case "+", "-":
		return 12, false, false
	case "*", "/", "%":
		return 13, false, false
	default:
		return 0, false, false
	}
}
