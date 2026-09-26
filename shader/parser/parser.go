// Package parser parses Godot 4 shading-language tokens into a typed AST.
package parser

import (
	"fmt"
	"strings"

	"github.com/cafecito-games/gdparser/shader/ast"
	"github.com/cafecito-games/gdparser/shader/lexer"
	"github.com/cafecito-games/gdparser/shader/token"
)

// Parse parses one .gdshader or .gdshaderinc file.
func Parse(filename string, source []byte) (*ast.File, error) {
	tokens, err := lexer.Lex(filename, source)
	if err != nil {
		return nil, err
	}
	p := &parser{filename: filename, tokens: tokens, types: builtinTypes()}
	return p.parseFile()
}

type parser struct {
	filename string
	tokens   []token.Token
	at       int
	types    map[string]bool
}

func (p *parser) current() token.Token { return p.tokens[p.at] }
func (p *parser) previous() token.Token {
	if p.at == 0 {
		return p.tokens[0]
	}
	return p.tokens[p.at-1]
}
func (p *parser) advance() token.Token {
	t := p.current()
	if t.Kind != token.EOF {
		p.at++
	}
	return t
}
func (p *parser) check(text string) bool { return p.current().Text == text }
func (p *parser) match(text string) bool {
	if p.check(text) {
		p.advance()
		return true
	}
	return false
}
func (p *parser) errorf(t token.Token, format string, args ...any) error {
	return &token.Error{Filename: p.filename, Position: t.Span.Start, Message: fmt.Sprintf(format, args...)}
}
func (p *parser) expect(text string) (token.Token, error) {
	if !p.check(text) {
		return token.Token{}, p.errorf(p.current(), "expected %q, found %q", text, p.current().Text)
	}
	return p.advance(), nil
}
func (p *parser) expectIdent(what string) (token.Token, error) {
	if p.current().Kind != token.Identifier {
		return token.Token{}, p.errorf(p.current(), "expected %s, found %q", what, p.current().Text)
	}
	return p.advance(), nil
}

func (p *parser) parseFile() (*ast.File, error) {
	file := &ast.File{Name: p.filename}
	start := p.current().Span.Start
	for p.current().Kind != token.EOF {
		item, err := p.parseItem()
		if err != nil {
			return nil, err
		}
		file.Items = append(file.Items, item)
	}
	file.SourceSpan = token.Span{Start: start, End: p.current().Span.End}
	return file, nil
}

func (p *parser) parseItem() (ast.Item, error) {
	if p.current().Kind == token.Comment {
		return comment(p.advance()), nil
	}
	if p.current().Kind == token.Directive {
		return directive(p.advance()), nil
	}
	if p.match("shader_type") {
		start := p.previous()
		name, err := p.expectIdent("shader type")
		if err != nil {
			return nil, err
		}
		end, err := p.expect(";")
		if err != nil {
			return nil, err
		}
		return &ast.ShaderType{Base: base(start, end), Name: name.Text}, nil
	}
	if p.match("render_mode") {
		start := p.previous()
		n := &ast.RenderMode{}
		for {
			e, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			n.Modes = append(n.Modes, e)
			if !p.match(",") {
				break
			}
		}
		end, err := p.expect(";")
		if err != nil {
			return nil, err
		}
		n.SourceSpan = join(start, end)
		return n, nil
	}
	if p.match("stencil_mode") {
		start := p.previous()
		n := &ast.StencilMode{}
		for {
			e, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			n.Modes = append(n.Modes, e)
			if !p.match(",") {
				break
			}
		}
		end, err := p.expect(";")
		if err != nil {
			return nil, err
		}
		n.SourceSpan = join(start, end)
		return n, nil
	}
	if p.match("group_uniforms") {
		start := p.previous()
		n := &ast.GroupUniforms{}
		if !p.check(";") {
			name, err := p.expectIdent("uniform group name")
			if err != nil {
				return nil, err
			}
			n.Name = name.Text
			if p.match(".") {
				sub, err := p.expectIdent("uniform subgroup name")
				if err != nil {
					return nil, err
				}
				n.Subgroup = sub.Text
			}
		}
		end, err := p.expect(";")
		if err != nil {
			return nil, err
		}
		n.SourceSpan = join(start, end)
		return n, nil
	}
	if p.match("struct") {
		return p.parseStruct(p.previous())
	}
	return p.parseTopDeclaration()
}

func (p *parser) parseStruct(start token.Token) (ast.Item, error) {
	name, err := p.expectIdent("struct name")
	if err != nil {
		return nil, err
	}
	if _, err = p.expect("{"); err != nil {
		return nil, err
	}
	n := &ast.StructDeclaration{Name: name.Text}
	p.types[name.Text] = true
	for !p.check("}") {
		if p.current().Kind == token.EOF {
			return nil, p.errorf(p.current(), "unterminated struct declaration")
		}
		if p.current().Kind == token.Comment {
			n.Members = append(n.Members, comment(p.advance()))
			continue
		}
		decl, err := p.parseVariableDeclaration(true)
		if err != nil {
			return nil, err
		}
		n.Members = append(n.Members, decl)
	}
	p.advance()
	end, err := p.expect(";")
	if err != nil {
		return nil, err
	}
	n.SourceSpan = join(start, end)
	return n, nil
}

func (p *parser) parseTopDeclaration() (ast.Item, error) {
	start := p.current()
	q := p.parseQualifiers()
	typeTok, err := p.expectIdent("declaration type")
	if err != nil {
		return nil, err
	}
	name, err := p.expectIdent("declaration name")
	if err != nil {
		return nil, err
	}
	if p.check("(") {
		fn, err := p.parseFunction(start, q, typeTok.Text, name)
		if err != nil {
			return nil, err
		}
		return fn, nil
	}
	decl, err := p.finishVariableDeclaration(start, q, typeTok.Text, name, true)
	if err != nil {
		return nil, err
	}
	return decl, nil
}

func (p *parser) parseFunction(start token.Token, q ast.Qualifiers, returnType string, name token.Token) (*ast.FunctionDeclaration, error) {
	n := &ast.FunctionDeclaration{ReturnQualifiers: q, ReturnType: returnType, Name: name.Text}
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	if !p.check(")") {
		for {
			paramStart := p.current()
			pq := p.parseQualifiers()
			typ, err := p.expectIdent("parameter type")
			if err != nil {
				return nil, err
			}
			pn, err := p.expectIdent("parameter name")
			if err != nil {
				return nil, err
			}
			param := &ast.Parameter{Qualifiers: pq, Type: typ.Text, Name: pn.Text}
			arrays, err := p.parseArrays()
			if err != nil {
				return nil, err
			}
			param.Arrays = arrays
			param.SourceSpan = token.Span{Start: paramStart.Span.Start, End: p.previous().Span.End}
			n.Parameters = append(n.Parameters, param)
			if !p.match(",") {
				break
			}
		}
	}
	if _, err := p.expect(")"); err != nil {
		return nil, err
	}
	arrays, err := p.parseArrays()
	if err != nil {
		return nil, err
	}
	n.ReturnArrays = arrays
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	n.Body = body
	n.SourceSpan = token.Span{Start: start.Span.Start, End: body.Span().End}
	return n, nil
}

func (p *parser) parseVariableDeclaration(expectSemicolon bool) (*ast.VariableDeclaration, error) {
	start := p.current()
	q := p.parseQualifiers()
	typ, err := p.expectIdent("variable type")
	if err != nil {
		return nil, err
	}
	name, err := p.expectIdent("variable name")
	if err != nil {
		return nil, err
	}
	return p.finishVariableDeclaration(start, q, typ.Text, name, expectSemicolon)
}

func (p *parser) finishVariableDeclaration(start token.Token, q ast.Qualifiers, typ string, first token.Token, expectSemicolon bool) (*ast.VariableDeclaration, error) {
	n := &ast.VariableDeclaration{Qualifiers: q, Type: typ}
	name := first
	for {
		d := &ast.Declarator{Name: name.Text}
		arrays, err := p.parseArrays()
		if err != nil {
			return nil, err
		}
		d.Arrays = arrays
		if p.match("=") {
			d.Value, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}
		d.SourceSpan = token.Span{Start: name.Span.Start, End: p.previous().Span.End}
		n.Declarators = append(n.Declarators, d)
		if !p.match(",") {
			break
		}
		name, err = p.expectIdent("variable name")
		if err != nil {
			return nil, err
		}
	}
	if p.match(":") {
		for {
			hStart, err := p.expectIdent("uniform hint")
			if err != nil {
				return nil, err
			}
			h := &ast.Hint{Name: hStart.Text}
			if p.match("(") {
				if !p.check(")") {
					for {
						arg, e := p.parseExpression()
						if e != nil {
							return nil, e
						}
						h.Arguments = append(h.Arguments, arg)
						if !p.match(",") {
							break
						}
					}
				}
				if _, err = p.expect(")"); err != nil {
					return nil, err
				}
			}
			h.SourceSpan = token.Span{Start: hStart.Span.Start, End: p.previous().Span.End}
			n.Hints = append(n.Hints, h)
			if !p.match(",") {
				break
			}
		}
	}
	if p.match("=") {
		if len(n.Declarators) != 1 {
			return nil, p.errorf(p.previous(), "uniform hint initializer requires one declarator")
		}
		var err error
		n.Declarators[0].Value, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
		n.Declarators[0].SourceSpan.End = n.Declarators[0].Value.Span().End
	}
	end := p.previous()
	if expectSemicolon {
		var err error
		end, err = p.expect(";")
		if err != nil {
			return nil, err
		}
	}
	n.SourceSpan = join(start, end)
	return n, nil
}

func (p *parser) parseArrays() ([]*ast.ArraySpecifier, error) {
	var arrays []*ast.ArraySpecifier
	for p.match("[") {
		start := p.previous()
		a := &ast.ArraySpecifier{}
		var err error
		if !p.check("]") {
			a.Size, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}
		end, err := p.expect("]")
		if err != nil {
			return nil, err
		}
		a.SourceSpan = join(start, end)
		arrays = append(arrays, a)
	}
	return arrays, nil
}

func (p *parser) parseQualifiers() ast.Qualifiers {
	var q ast.Qualifiers
	for p.current().Kind == token.Identifier {
		s := p.current().Text
		switch s {
		case "const":
			q.Const = true
		case "flat", "smooth":
			q.Interpolation = s
		case "lowp", "mediump", "highp":
			q.Precision = s
		case "invariant":
			q.Invariant = true
		case "uniform", "varying", "in", "out", "inout", "instance", "global":
			q.Storage = append(q.Storage, s)
		default:
			return q
		}
		p.advance()
	}
	return q
}

func (p *parser) parseBlock() (*ast.Block, error) {
	start, err := p.expect("{")
	if err != nil {
		return nil, err
	}
	b := &ast.Block{}
	for !p.check("}") {
		if p.current().Kind == token.EOF {
			return nil, p.errorf(p.current(), "unterminated block")
		}
		stmt, e := p.parseStatement()
		if e != nil {
			return nil, e
		}
		b.Statements = append(b.Statements, stmt)
	}
	end := p.advance()
	b.SourceSpan = join(start, end)
	return b, nil
}

func (p *parser) parseStatement() (ast.Statement, error) {
	if p.current().Kind == token.Comment {
		return comment(p.advance()), nil
	}
	if p.current().Kind == token.Directive {
		return directive(p.advance()), nil
	}
	if p.check("{") {
		return p.parseBlock()
	}
	if p.match("if") {
		return p.parseIf(p.previous())
	}
	if p.match("for") {
		return p.parseFor(p.previous())
	}
	if p.match("while") {
		return p.parseWhile(p.previous())
	}
	if p.match("do") {
		return p.parseDo(p.previous())
	}
	if p.match("switch") {
		return p.parseSwitch(p.previous())
	}
	if p.match("return") {
		start := p.previous()
		n := &ast.ReturnStatement{}
		var err error
		if !p.check(";") {
			n.Value, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}
		end, err := p.expect(";")
		if err != nil {
			return nil, err
		}
		n.SourceSpan = join(start, end)
		return n, nil
	}
	for _, keyword := range []string{"break", "continue", "discard"} {
		if p.match(keyword) {
			start := p.previous()
			end, err := p.expect(";")
			if err != nil {
				return nil, err
			}
			return &ast.KeywordStatement{Base: base(start, end), Keyword: keyword}, nil
		}
	}
	if p.looksLikeDeclaration() {
		return p.parseVariableDeclaration(true)
	}
	start := p.current()
	if p.match(";") {
		return &ast.ExpressionStatement{Base: base(start, p.previous())}, nil
	}
	e, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	end, err := p.expect(";")
	if err != nil {
		return nil, err
	}
	return &ast.ExpressionStatement{Base: base(start, end), Expression: e}, nil
}

func (p *parser) looksLikeDeclaration() bool {
	i := p.at
	for i < len(p.tokens) && isQualifier(p.tokens[i].Text) {
		i++
	}
	if i+1 >= len(p.tokens) || p.tokens[i].Kind != token.Identifier || p.tokens[i+1].Kind != token.Identifier {
		return false
	}
	return p.types[p.tokens[i].Text] || i > p.at || p.tokens[i+1].Text != "="
}

func (p *parser) parseIf(start token.Token) (ast.Statement, error) {
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(")"); err != nil {
		return nil, err
	}
	then, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	n := &ast.IfStatement{Condition: cond, Then: then}
	end := then.Span().End
	if p.match("else") {
		n.Else, err = p.parseStatement()
		if err != nil {
			return nil, err
		}
		end = n.Else.Span().End
	}
	n.SourceSpan = token.Span{Start: start.Span.Start, End: end}
	return n, nil
}

func (p *parser) parseFor(start token.Token) (ast.Statement, error) {
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	n := &ast.ForStatement{}
	if !p.check(";") {
		if p.looksLikeDeclaration() {
			d, err := p.parseVariableDeclaration(true)
			if err != nil {
				return nil, err
			}
			n.Initializer = d
		} else {
			s := p.current()
			e, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			end, err := p.expect(";")
			if err != nil {
				return nil, err
			}
			n.Initializer = &ast.ExpressionStatement{Base: base(s, end), Expression: e}
		}
	} else {
		p.advance()
	}
	var err error
	if !p.check(";") {
		n.Condition, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.expect(";"); err != nil {
		return nil, err
	}
	if !p.check(")") {
		n.Update, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.expect(")"); err != nil {
		return nil, err
	}
	n.Body, err = p.parseStatement()
	if err != nil {
		return nil, err
	}
	n.SourceSpan = token.Span{Start: start.Span.Start, End: n.Body.Span().End}
	return n, nil
}

func (p *parser) parseWhile(start token.Token) (ast.Statement, error) {
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(")"); err != nil {
		return nil, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return &ast.WhileStatement{Base: ast.Base{SourceSpan: token.Span{Start: start.Span.Start, End: body.Span().End}}, Condition: cond, Body: body}, nil
}

func (p *parser) parseDo(start token.Token) (ast.Statement, error) {
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect("while"); err != nil {
		return nil, err
	}
	if _, err = p.expect("("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(")"); err != nil {
		return nil, err
	}
	end, err := p.expect(";")
	if err != nil {
		return nil, err
	}
	return &ast.DoWhileStatement{Base: base(start, end), Body: body, Condition: cond}, nil
}

func (p *parser) parseSwitch(start token.Token) (ast.Statement, error) {
	if _, err := p.expect("("); err != nil {
		return nil, err
	}
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect(")"); err != nil {
		return nil, err
	}
	if _, err = p.expect("{"); err != nil {
		return nil, err
	}
	n := &ast.SwitchStatement{Value: value}
	for !p.check("}") {
		caseStart := p.current()
		c := &ast.SwitchCase{}
		if p.match("case") {
			c.Value, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		} else if p.match("default") {
			c.Default = true
		} else {
			return nil, p.errorf(p.current(), "expected case or default")
		}
		if _, err = p.expect(":"); err != nil {
			return nil, err
		}
		for !p.check("case") && !p.check("default") && !p.check("}") {
			s, e := p.parseStatement()
			if e != nil {
				return nil, e
			}
			c.Statements = append(c.Statements, s)
		}
		c.SourceSpan = token.Span{Start: caseStart.Span.Start, End: p.previous().Span.End}
		n.Cases = append(n.Cases, c)
	}
	end := p.advance()
	n.SourceSpan = join(start, end)
	return n, nil
}

func comment(t token.Token) *ast.Comment {
	return &ast.Comment{Base: ast.Base{SourceSpan: t.Span}, Text: t.Text, Block: strings.HasPrefix(t.Text, "/*")}
}
func directive(t token.Token) *ast.PreprocessorDirective {
	text := strings.TrimSpace(strings.TrimPrefix(t.Text, "#"))
	name, body, _ := strings.Cut(text, " ")
	if body == "" {
		name, body, _ = strings.Cut(text, "\t")
	}
	return &ast.PreprocessorDirective{Base: ast.Base{SourceSpan: t.Span}, Name: name, Body: strings.TrimSpace(body)}
}
func base(a, b token.Token) ast.Base   { return ast.Base{SourceSpan: join(a, b)} }
func join(a, b token.Token) token.Span { return token.Span{Start: a.Span.Start, End: b.Span.End} }
func isQualifier(s string) bool {
	switch s {
	case "const", "flat", "smooth", "lowp", "mediump", "highp", "invariant", "uniform", "varying", "in", "out", "inout", "instance", "global":
		return true
	}
	return false
}
func builtinTypes() map[string]bool {
	m := map[string]bool{}
	for _, s := range []string{"void", "bool", "bvec2", "bvec3", "bvec4", "int", "ivec2", "ivec3", "ivec4", "uint", "uvec2", "uvec3", "uvec4", "float", "vec2", "vec3", "vec4", "mat2", "mat3", "mat4", "sampler2D", "isampler2D", "usampler2D", "sampler2DArray", "isampler2DArray", "usampler2DArray", "sampler3D", "isampler3D", "usampler3D", "samplerCube", "samplerCubeArray", "samplerExternalOES"} {
		m[s] = true
	}
	return m
}
