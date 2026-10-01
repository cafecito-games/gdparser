// Package parser parses GDScript tokens into a typed AST.
package parser

import (
	"fmt"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/lexer"
	"github.com/cafecito-games/gdparser/token"
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

// Parse parses one GDScript source file.
func Parse(filename string, source []byte) (*ast.File, error) {
	tokens, err := lexer.Lex(source)
	if err != nil {
		if filename != "" {
			return nil, fmt.Errorf("%s:%w", filename, err)
		}
		return nil, err
	}
	p := &parser{filename: filename, tokens: tokens}
	statements, err := p.parseStatements(false)
	if err != nil {
		return nil, err
	}
	file := &ast.File{Name: filename, Statements: statements}
	if len(tokens) > 0 {
		file.SourceSpan = token.Span{Start: tokens[0].Span.Start, End: tokens[len(tokens)-1].Span.End}
	}
	return file, nil
}

type parser struct {
	filename string
	tokens   []token.Token
	current  int
	// blankLines carries the blank lines found at the end of a nested block
	// across the return from that block, so they count towards the statement
	// that follows it.
	blankLines int
}

// takeBlankLines returns and clears the blank lines left over by a nested block.
func (p *parser) takeBlankLines() int {
	count := p.blankLines
	p.blankLines = 0
	return count
}

func (p *parser) parseStatements(block bool) ([]ast.Statement, error) {
	var statements []ast.Statement
	var pending []*ast.Annotation
	pendingBlankLines := 0
	// flush emits annotations that decorate no declaration as plain statements.
	flush := func() {
		for index, annotation := range pending {
			if index == 0 {
				annotation.BlankLinesBefore = pendingBlankLines
			}
			statements = append(statements, annotation)
		}
		pending = nil
	}
	for !p.at(token.EOF) {
		blankLines := p.takeBlankLines()
		for p.match(token.Newline) {
			blankLines++
		}
		if block && p.at(token.Dedent) {
			p.advance()
			p.blankLines = blankLines
			flush()
			return statements, nil
		}
		if p.at(token.EOF) {
			break
		}
		if p.at(token.Dedent) {
			return nil, p.error(p.peek(), "unexpected dedent")
		}

		stmt, compound, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if annotation, ok := stmt.(*ast.Annotation); ok {
			if len(pending) == 0 {
				pendingBlankLines = blankLines
			}
			pending = append(pending, annotation)
			if compound {
				continue
			}
			annotation.OwnLine = true
			if p.at(token.Comment) {
				annotation.TrailingComment = commentNode(p.advance())
			}
			if _, err := p.expect(token.Newline, "expected end of line"); err != nil {
				return nil, err
			}
			continue
		}
		if len(pending) > 0 {
			if attachAnnotations(stmt, pending) {
				blankLines = pendingBlankLines
				pending = nil
			} else {
				flush()
				blankLines = 0
			}
		}
		if trivia := ast.TriviaOf(stmt); trivia != nil {
			trivia.BlankLinesBefore = blankLines
		}
		statements = append(statements, stmt)
		if compound {
			continue
		}
		if p.match(token.Semicolon) {
			continue
		}
		if p.at(token.Comment) {
			comment := commentNode(p.advance())
			if trivia := ast.TriviaOf(stmt); trivia != nil {
				trivia.TrailingComment = comment
			} else {
				statements = append(statements, comment)
			}
		}
		if _, err := p.expect(token.Newline, "expected end of line"); err != nil {
			return nil, err
		}
	}
	flush()
	if block {
		return nil, p.error(p.peek(), "expected an indented block")
	}
	return statements, nil
}

// attachAnnotations binds annotations to the declaration they decorate and
// reports whether statement can carry them.
func attachAnnotations(statement ast.Statement, annotations []*ast.Annotation) bool {
	switch declaration := statement.(type) {
	case *ast.VariableDeclaration:
		declaration.Annotations = annotations
	case *ast.FunctionDeclaration:
		declaration.Annotations = annotations
	case *ast.ClassDeclaration:
		declaration.Annotations = annotations
	case *ast.SignalDeclaration:
		declaration.Annotations = annotations
	case *ast.EnumDeclaration:
		declaration.Annotations = annotations
	default:
		return false
	}
	return true
}

func (p *parser) parseStatement() (ast.Statement, bool, error) {
	switch p.peek().Type {
	case token.Comment:
		return commentNode(p.advance()), false, nil
	case token.At:
		stmt, err := p.parseAnnotation()
		return stmt, !p.at(token.Newline) && !p.at(token.Comment), err
	case token.Tool:
		tok := p.advance()
		return &ast.Directive{Base: base(tok.Span), Name: tok.Lexeme, KeywordSpan: tok.Span}, false, nil
	case token.Extends, token.ClassName:
		stmt, err := p.parseDirective()
		return stmt, false, err
	case token.Var, token.Const:
		stmt, err := p.parseVariable()
		compound := statementHasBlockLambda(stmt)
		if declaration, ok := stmt.(*ast.VariableDeclaration); ok {
			compound = compound || declaration.Getter != nil || declaration.Setter != nil
		}
		return stmt, compound, err
	case token.Static:
		static := p.advance()
		if p.at(token.Func) {
			keyword := p.advance()
			stmt, err := p.parseFunction(keyword, static)
			return stmt, true, err
		}
		if p.at(token.Var) {
			keyword := p.advance()
			stmt, err := p.parseVariableAfter(keyword, static, false)
			compound := statementHasBlockLambda(stmt)
			if declaration, ok := stmt.(*ast.VariableDeclaration); ok {
				compound = compound || declaration.Getter != nil || declaration.Setter != nil
			}
			return stmt, compound, err
		}
		return nil, false, p.error(p.peek(), "expected func or var after static")
	case token.Func:
		keyword := p.advance()
		stmt, err := p.parseFunction(keyword, token.Token{})
		return stmt, true, err
	case token.Class:
		stmt, err := p.parseClass()
		return stmt, true, err
	case token.Signal:
		stmt, err := p.parseSignal()
		return stmt, false, err
	case token.Enum:
		stmt, err := p.parseEnum()
		return stmt, false, err
	case token.If:
		stmt, err := p.parseIf()
		return stmt, true, err
	case token.While:
		stmt, err := p.parseWhile()
		return stmt, true, err
	case token.For:
		stmt, err := p.parseFor()
		return stmt, true, err
	case token.Match:
		stmt, err := p.parseMatch()
		return stmt, true, err
	case token.Return:
		stmt, err := p.parseReturn()
		return stmt, statementHasBlockLambda(stmt), err
	case token.Pass, token.Break, token.Continue:
		tok := p.advance()
		return &ast.KeywordStatement{Base: base(tok.Span), Keyword: tok.Lexeme, KeywordSpan: tok.Span}, false, nil
	default:
		stmt, err := p.parseExpressionStatement()
		return stmt, statementHasBlockLambda(stmt), err
	}
}

func (p *parser) parseSuite() ([]ast.Statement, token.Position, error) {
	if _, err := p.expect(token.Colon, "expected ':' before block"); err != nil {
		return nil, token.Position{}, err
	}
	var headerComment ast.Statement
	if p.at(token.Comment) {
		headerComment = commentNode(p.advance())
	}
	if !p.at(token.Newline) {
		stmt, _, err := p.parseStatement()
		if err != nil {
			return nil, token.Position{}, err
		}
		body := []ast.Statement{}
		if headerComment != nil {
			body = append(body, headerComment)
		}
		body = append(body, stmt)
		for p.match(token.Semicolon) {
			if p.at(token.Newline, token.Comment) {
				break
			}
			next, _, nextErr := p.parseStatement()
			if nextErr != nil {
				return nil, token.Position{}, nextErr
			}
			body = append(body, next)
		}
		if p.at(token.Comment) {
			body = append(body, commentNode(p.advance()))
		}
		end := stmt.Span().End
		if len(body) > 0 {
			end = body[len(body)-1].Span().End
		}
		return body, end, nil
	}
	if _, err := p.expect(token.Newline, "expected newline before block"); err != nil {
		return nil, token.Position{}, err
	}
	var leading []ast.Statement
	if headerComment != nil {
		leading = append(leading, headerComment)
	}
	for {
		for p.match(token.Newline) {
		}
		if !p.at(token.Comment) {
			break
		}
		leading = append(leading, commentNode(p.advance()))
		if _, err := p.expect(token.Newline, "expected end of comment line"); err != nil {
			return nil, token.Position{}, err
		}
	}
	if _, err := p.expect(token.Indent, "expected an indented block"); err != nil {
		return nil, token.Position{}, err
	}
	body, err := p.parseStatements(true)
	if err != nil {
		return nil, token.Position{}, err
	}
	end := p.previous().Span.End
	return append(leading, body...), end, nil
}

func (p *parser) at(types ...token.Type) bool {
	current := p.peek().Type
	for _, typ := range types {
		if current == typ {
			return true
		}
	}
	return false
}

func (p *parser) match(types ...token.Type) bool {
	if p.at(types...) {
		p.advance()
		return true
	}
	return false
}

func (p *parser) expect(typ token.Type, message string) (token.Token, error) {
	if p.at(typ) {
		return p.advance(), nil
	}
	return token.Token{}, p.error(p.peek(), message)
}

func (p *parser) advance() token.Token {
	tok := p.peek()
	if p.current < len(p.tokens) {
		p.current++
	}
	return tok
}

func (p *parser) peek() token.Token {
	if p.current >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.current]
}

func (p *parser) peekN(n int) token.Token {
	index := p.current + n
	if index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[index]
}

func (p *parser) previous() token.Token { return p.tokens[p.current-1] }

func (p *parser) error(tok token.Token, message string) error {
	return &Error{Filename: p.filename, Token: tok, Message: message}
}

func statementHasBlockLambda(statement ast.Statement) bool {
	if statement == nil {
		return false
	}
	found := false
	ast.Inspect(statement, func(node ast.Node) bool {
		if lambda, ok := node.(*ast.LambdaExpression); ok && !lambda.Inline {
			found = true
			return false
		}
		return !found
	})
	return found
}

// nameKind says which keywords Godot still accepts where a name is expected.
// The set differs by position, so each name position names its own kind.
type nameKind int

const (
	// declaredName is a name Godot declares or binds: a variable, a function,
	// a parameter, a class, a signal, an enum or its member, a loop variable,
	// a lambda, or a type. Only the contextual keywords may be used there.
	declaredName nameKind = iota
	// memberName follows a dot in an expression, where Godot accepts every
	// keyword that is not a literal.
	memberName
	// annotationName follows '@', where Godot reads the name as text and so
	// accepts every keyword.
	annotationName
)

// isNameToken reports whether tok may stand for a name of the given kind.
// Godot keeps "match" and "tool" usable as names, so a keyword is not reserved
// everywhere, and after a dot or an '@' it relaxes further still.
func isNameToken(tok token.Token, kind nameKind) bool {
	if tok.Type == token.Identifier {
		return true
	}
	if !token.IsKeyword(tok.Type) {
		return false
	}
	switch kind {
	case annotationName:
		return true
	case memberName:
		return tok.Type != token.True && tok.Type != token.False && tok.Type != token.Null
	}
	return tok.Type == token.Match || tok.Type == token.Tool
}

func (p *parser) expectName(kind nameKind, message string) (token.Token, error) {
	if isNameToken(p.peek(), kind) {
		return p.advance(), nil
	}
	return token.Token{}, p.error(p.peek(), message)
}

func base(span token.Span) ast.Base { return ast.Base{SourceSpan: span} }

func spanFrom(start token.Position, end token.Position) ast.Base {
	return ast.Base{SourceSpan: token.Span{Start: start, End: end}}
}

// takeCollectionComments consumes a run of comments and anchors each one to the
// item at index. A comment that starts on the line the previous token ended is
// recorded as trailing that line. A newline, indent or dedent carries no text,
// so a comment that follows one begins its own line however the spans read.
func (p *parser) takeCollectionComments(comments *[]ast.CollectionComment, index int) {
	for p.at(token.Comment) {
		trailing := p.current > 0 && !endsLine(p.previous().Type) &&
			p.previous().Span.End.Line == p.peek().Span.Start.Line
		*comments = append(*comments, ast.CollectionComment{
			Comment: commentNode(p.advance()), Index: index, Trailing: trailing,
		})
	}
}

// endsLine reports whether typ is a token that carries no text of its own and
// so cannot have a comment trailing it on the same line.
func endsLine(typ token.Type) bool {
	return typ == token.Newline || typ == token.Indent || typ == token.Dedent
}

// peekPastComments returns the first token that is not part of the comment run
// at the current position.
func (p *parser) peekPastComments() token.Token {
	offset := 0
	for p.peekN(offset).Type == token.Comment {
		offset++
	}
	return p.peekN(offset)
}

func commentNode(tok token.Token) *ast.Comment {
	return &ast.Comment{Base: base(tok.Span), Text: tok.Lexeme, Documentation: len(tok.Lexeme) > 1 && tok.Lexeme[1] == '#'}
}
