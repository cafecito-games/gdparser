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
	p := &parser{filename: filename, scanner: lexer.NewScanner(source)}
	// Line breaks carry meaning at the top level, which is the one frame Godot
	// keeps on its multiline stack for the whole parse.
	p.pushMultiline(false)
	first, err := p.token(0)
	if err != nil {
		return nil, p.wrap(err)
	}
	statements, err := p.parseStatements(false)
	if err != nil {
		return nil, err
	}
	if p.scanErr != nil {
		// A statement list ends at the end of the tokens, which is where a
		// lexical error leaves the scan, so the error is reported here rather
		// than read as the end of the file.
		return nil, p.wrap(p.scanErr)
	}
	file := &ast.File{Name: filename, Statements: statements}
	file.SourceSpan = token.Span{Start: first.Span.Start, End: p.peek().Span.End}
	return file, nil
}

type parser struct {
	filename string
	scanner  *lexer.Scanner
	current  int
	// scanErr holds a lexical error found while looking at a token, so that the
	// peeking helpers can stay free of error returns.
	scanErr error
	// multilineStack records, innermost last, whether a line break carries
	// meaning in the construct being parsed. Godot keeps the same stack, and the
	// tokenizer follows its top.
	multilineStack []bool
	// inLambda reports that the statement being read belongs to a lambda body,
	// where anything that is not a line break may end the body instead.
	inLambda bool
	// lambdaEnded records that a lambda body has just ended. The end of a body
	// is not a token, so it is carried here and spent where a line break would
	// otherwise be required.
	lambdaEnded bool
	// blankLines carries the blank lines found at the end of a nested block
	// across the return from that block, so they count towards the statement
	// that follows it.
	blankLines int
}

// token returns the token at index, recording a lexical error so that the
// peeking helpers need not report one.
func (p *parser) token(index int) (token.Token, error) {
	tok, err := p.scanner.At(index)
	if err != nil {
		if p.scanErr == nil {
			p.scanErr = err
		}
		return token.Token{Type: token.EOF}, err
	}
	return tok, nil
}

// wrap gives a lexical error the filename its position is relative to.
func (p *parser) wrap(err error) error {
	if p.filename == "" {
		return err
	}
	return fmt.Errorf("%s:%w", p.filename, err)
}

// pushMultiline says whether a line break carries meaning in the construct about
// to be parsed, and tells the scanner. Godot's push_multiline does the same.
func (p *parser) pushMultiline(multiline bool) {
	p.multilineStack = append(p.multilineStack, multiline)
	p.scanner.SetMultilineMode(p.current, multiline)
}

// popMultiline returns to the enclosing construct's answer. A lambda body that
// ended inside the construct being closed is forgotten with it: the end of a body
// may stand in for the end of the statement holding it, but only while the
// expression has not moved on, and a closing bracket moves it on.
//
// This is a deliberate departure from a literal reading of upstream. Godot sets
// lambda_ended and does not clear it at the closing bracket, in 4.6 and 4.7
// alike, so the guard at the top of parse_precedence appears to stop it reading
// the "or" in "a.any(func(): return x) or b". Published GDScript is written that
// way all the same, so something must accept it: the shape is in vest 1.10.4, a
// third-party test library, at vest-defs.gd:42. The mark is dropped with the
// bracket here so that such source parses.
func (p *parser) popMultiline() {
	p.multilineStack = p.multilineStack[:len(p.multilineStack)-1]
	p.lambdaEnded = false
	p.scanner.SetMultilineMode(p.current, p.multiline())
}

// multiline reports whether a line break currently carries no meaning, which is
// to say that the parser is inside a bracketed construct.
func (p *parser) multiline() bool {
	if len(p.multilineStack) == 0 {
		return false
	}
	return p.multilineStack[len(p.multilineStack)-1]
}

// dropBlankLines discards the blank lines left over by a nested block. A
// keyword that continues a compound statement, such as elif or else, stands on
// a line of its own that belongs to no block, so blank lines before it precede
// that line rather than the first statement of the block it opens.
func (p *parser) dropBlankLines() { p.blankLines = 0 }

// spendLambdaEnd settles a lambda that ended inside the compound statement just
// read. Godot requires nothing after such a statement, so this only spends a
// mark already left: when the statement's line ends, or its block does, the
// statement ended there and the mark goes with it. A mark still standing where
// something else follows belongs to a lambda body that ended deeper in, so it is
// left for the list holding that body to find. Nothing is marked here, because a
// compound statement needs no end of its own.
func (p *parser) spendLambdaEnd() {
	if !p.lambdaEnded {
		return
	}
	if !p.inLambda || p.at(token.Newline, token.Semicolon, token.EOF, token.Dedent) {
		p.lambdaEnded = false
	}
}

// takeBlankLines returns and clears the blank lines left over by a nested block.
func (p *parser) takeBlankLines() int {
	count := p.blankLines
	p.blankLines = 0
	return count
}

// parseStatements reads a list of statements. block says the list is an indented
// block, which ends at its dedent. A list read anywhere inside a lambda body may
// also end at the first thing that could not continue it, because what follows
// belongs to the expression the lambda was written in; the body and every block
// nested in it then close together.
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
		if block && p.inLambda && p.lambdaEnded {
			// A block inside a lambda body ends with the body, so it may close
			// without the dedent a written line break would have left.
			p.match(token.Dedent)
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

		if p.inLambda && !beginsStatement(p.peek().Type) {
			// Inside a lambda body, source that could not begin a statement is
			// the rest of the expression the lambda was written in, so the body
			// ends here rather than failing. Godot ends it the same way, on an
			// expression that would not parse.
			p.lambdaEnded = true
			p.blankLines = blankLines
			flush()
			return statements, nil
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
			// A compound statement read its own block, so no line break is
			// required after it.
			p.spendLambdaEnd()
			continue
		}
		if p.at(token.Semicolon) {
			// A semicolon ends a statement, and Godot's end_statement consumes a
			// whole run of them, so empty ones in between are not statements.
			for p.match(token.Semicolon) {
			}
			if !p.at(token.Newline, token.Comment) {
				continue
			}
		}
		if p.at(token.Comment) && p.endsLineOf(stmt) {
			comment := commentNode(p.advance())
			if trivia := ast.TriviaOf(stmt); trivia != nil {
				trivia.TrailingComment = comment
			} else {
				statements = append(statements, comment)
			}
		}
		if !p.at(token.Newline) {
			if p.lambdaEnded {
				// A lambda inside this statement ended without a line break of
				// its own, and that end stands in for the one the statement
				// needs.
				p.lambdaEnded = false
				continue
			}
			if p.inLambda {
				// Nothing here could end the statement, so what follows belongs
				// to the expression the lambda was written in and the body ends.
				p.lambdaEnded = true
				flush()
				return statements, nil
			}
		}
		p.lambdaEnded = false
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
	case token.Extends, token.ClassName:
		stmt, err := p.parseDirective()
		return stmt, false, err
	case token.Var, token.Const:
		stmt, err := p.parseVariable()
		compound := false
		if declaration, ok := stmt.(*ast.VariableDeclaration); ok {
			compound = declaration.AccessorBlock
		}
		return stmt, compound, err
	case token.Static:
		static := p.advance()
		if p.at(token.Func) {
			keyword := p.advance()
			stmt, err := p.parseFunction(keyword, static)
			return stmt, hasBody(stmt), err
		}
		if p.at(token.Var) {
			keyword := p.advance()
			stmt, err := p.parseVariableAfter(keyword, static, false)
			compound := false
			if declaration, ok := stmt.(*ast.VariableDeclaration); ok {
				compound = declaration.AccessorBlock
			}
			return stmt, compound, err
		}
		return nil, false, p.error(p.peek(), "expected func or var after static")
	case token.Func:
		keyword := p.advance()
		stmt, err := p.parseFunction(keyword, token.Token{})
		return stmt, hasBody(stmt), err
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
		return stmt, false, err
	case token.Assert:
		stmt, err := p.parseAssert()
		return stmt, false, err
	case token.Pass, token.Break, token.Continue, token.Breakpoint:
		tok := p.advance()
		return &ast.KeywordStatement{Base: base(tok.Span), Keyword: tok.Lexeme, KeywordSpan: tok.Span}, false, nil
	default:
		stmt, err := p.parseExpressionStatement()
		return stmt, false, err
	}
}

func (p *parser) parseSuite() ([]ast.Statement, token.Position, error) {
	return p.parseSuiteFor(false)
}

// parseSuiteFor parses the block a colon opens. forLambda marks a lambda body,
// which ends at the first thing that could not continue it rather than only at a
// dedent, because the expression the lambda sits in picks up from there.
func (p *parser) parseSuiteFor(forLambda bool) ([]ast.Statement, token.Position, error) {
	if _, err := p.expect(token.Colon, "expected ':' before block"); err != nil {
		return nil, token.Position{}, err
	}
	var headerComment ast.Statement
	if p.at(token.Comment) {
		headerComment = commentNode(p.advance())
	}
	if !p.at(token.Newline) {
		if forLambda && !beginsStatement(p.peek().Type) {
			// A lambda may carry no body at all, as "func():" written inside an
			// expression does. Godot reads the body as empty and ends it here,
			// leaving what follows to the expression the lambda sits in.
			body := []ast.Statement{}
			if headerComment != nil {
				body = append(body, headerComment)
			}
			return body, p.previous().Span.End, nil
		}
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
		if p.inLambda && !p.at(token.Newline, token.Semicolon, token.EOF, token.Dedent) {
			p.lambdaEnded = true
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
	if tok.Type != token.EOF {
		p.current++
	}
	return tok
}

func (p *parser) peek() token.Token { return p.peekN(0) }

func (p *parser) peekN(n int) token.Token {
	tok, _ := p.token(p.current + n)
	return tok
}

func (p *parser) previous() token.Token {
	tok, _ := p.token(p.current - 1)
	return tok
}

func (p *parser) error(tok token.Token, message string) error {
	if p.scanErr != nil {
		// A token that could not be read is reported as itself rather than as
		// whatever the grammar expected in its place.
		return p.wrap(p.scanErr)
	}
	return &Error{Filename: p.filename, Token: tok, Message: message}
}

// hasBody reports whether a function declaration read its own block. An abstract
// one did not, so it ends with its line like any other simple statement, which is
// what lets a semicolon follow it.
func hasBody(statement ast.Statement) bool {
	declaration, ok := statement.(*ast.FunctionDeclaration)
	return ok && !declaration.Abstract
}

// expectIdentifier consumes a token that may stand where GDScript expects an
// identifier: a declared name, a parameter, a loop variable, a bind, or an
// element of a type name.
func (p *parser) expectIdentifier(message string) (token.Token, error) {
	if token.IsIdentifier(p.peek().Type) {
		return p.advance(), nil
	}
	return token.Token{}, p.error(p.peek(), message)
}

// expectNodeName consumes a token that may name a node, which is what Godot
// accepts both for a member after a dot and for a step of a "$" or "%" path.
func (p *parser) expectNodeName(message string) (token.Token, error) {
	if token.IsNodeName(p.peek().Type) {
		return p.advance(), nil
	}
	return token.Token{}, p.error(p.peek(), message)
}

// expectAnnotationName consumes the name of the annotation introduced by at.
// Godot scans "@" and the name as a single token, so the name may be spelled
// with any reserved word, but nothing may come between the two.
func (p *parser) expectAnnotationName(at token.Token) (token.Token, error) {
	name := p.peek()
	if !isWord(name.Type) || name.Span.Start.Offset != at.Span.End.Offset {
		return token.Token{}, p.error(name, "expected annotation name")
	}
	return p.advance(), nil
}

// isWord reports whether typ is spelled as a word in source, which is every
// identifier and every reserved word including the literals.
func isWord(typ token.Type) bool {
	return typ == token.Identifier || typ == token.Underscore || token.IsKeyword(typ)
}

// beginsStatement reports whether typ could open a statement, which is to say
// that it opens one of the statement forms or that it opens an expression. It is
// what tells a lambda body that it has ended: Godot reaches the same conclusion
// by finding that the expression it tried to read was not there.
func beginsStatement(typ token.Type) bool {
	switch typ {
	case token.At, token.Extends, token.ClassName, token.Var, token.Const,
		token.Static, token.Func, token.Class, token.Signal, token.Enum,
		token.If, token.While, token.For, token.Match, token.Return,
		token.Pass, token.Break, token.Continue, token.Breakpoint,
		token.Assert, token.Comment:
		return true
	}
	return beginsExpression(typ)
}

// beginsExpression reports whether typ has a prefix rule, which is how Godot
// decides whether an expression starts here.
func beginsExpression(typ token.Type) bool {
	switch typ {
	case token.Identifier, token.Integer, token.Float, token.String,
		token.True, token.False, token.Null, token.Ampersand, token.Caret,
		token.Minus, token.Plus, token.Not, token.Bang, token.Tilde,
		token.Await, token.LParen, token.LBracket, token.LBrace,
		token.Dollar, token.Percent, token.Func, token.Self, token.Super,
		token.Preload:
		return true
	}
	return token.IsIdentifier(typ)
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

// endsLineOf reports whether the comment the parser is sitting on ends the
// source line that statement ends on, which is what makes it that statement's
// trailing comment rather than a comment of its own. A block that closed before
// the comment leaves a dedent in between, which carries no text and so stands on
// no line.
func (p *parser) endsLineOf(statement ast.Statement) bool {
	previous := p.previous()
	if endsLine(previous.Type) {
		return false
	}
	return previous.Span.End.Line == p.peek().Span.Start.Line
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
