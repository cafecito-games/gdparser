// Package format emits canonical GDScript from an AST.
package format

import (
	"fmt"
	"strings"

	"github.com/cafecito-games/gdparser/ast"
)

// File formats a parsed or programmatically constructed GDScript file using the
// Godot style guide defaults.
func File(file *ast.File) string { return FileWithOptions(file, GodotStyle()) }

// FileWithOptions formats file using options. Unset numeric fields fall back to
// their GodotStyle values, so a partially populated Options inherits the rest of
// the style guide.
func FileWithOptions(file *ast.File, options Options) string {
	if file == nil {
		return ""
	}
	printer := &printer{options: options.normalized()}
	document := printer.statements(file.Statements)
	if document == nil {
		return ""
	}
	return render(concat(document, hardLine), printer.options)
}

type printer struct {
	options Options
	// brackets counts the constructs open around the expression being rendered
	// that indent their contents when broken, within the innermost block. A
	// bracket closing inside one lands on a continuation indentation rather
	// than on a statement's.
	brackets int
}

// inBrackets renders the contents of a construct that indents them when broken.
func (p *printer) inBrackets(render func()) {
	p.brackets++
	render()
	p.brackets--
}

// statements renders a statement list with the blank lines between its members,
// and returns nil when the list is empty.
func (p *printer) statements(list []ast.Statement) doc {
	if len(list) == 0 {
		return nil
	}
	gaps := p.blankLineGaps(list)
	parts := make([]doc, 0, len(list)*2)
	for index, statement := range list {
		switch {
		case index == 0:
		case sharesNextLine(list[index-1]):
			parts = append(parts, text(" "))
		default:
			for range gaps[index] + 1 {
				parts = append(parts, hardLine)
			}
		}
		parts = append(parts, p.statement(statement))
	}
	return concat(parts...)
}

// sharesNextLine reports whether statement is an annotation written ahead of the
// next statement on that statement's line, as "@tool extends Node" is. The style
// guide does not prescribe where such an annotation goes, so it stays there.
func sharesNextLine(statement ast.Statement) bool {
	annotation, ok := statement.(*ast.Annotation)
	return ok && !annotation.OwnLine && annotation.TrailingComment == nil
}

// body renders a suite's statements, substituting pass for an empty one.
func (p *printer) body(list []ast.Statement) doc {
	if document := p.statements(list); document != nil {
		return document
	}
	return text("pass")
}

// suite renders an indented block beneath a statement header, which the caller
// has written through its colon. A comment that ended the header's line in the
// source stays on it.
func (p *printer) suite(list []ast.Statement) doc {
	if len(list) > 0 {
		if comment, ok := list[0].(*ast.Comment); ok && comment.TrailsHeader {
			return concat(text("  "+p.commentText(comment)), nest(1, concat(hardLine, p.body(list[1:]))))
		}
	}
	return nest(1, concat(hardLine, p.body(list)))
}

// blankLineGaps returns the number of blank lines to place before each statement
// in list. Authored blank lines are kept but clamped, and function and class
// declarations are surrounded by exactly the configured count.
func (p *printer) blankLineGaps(list []ast.Statement) []int {
	gaps := make([]int, len(list))
	for index := 1; index < len(list); index++ {
		gaps[index] = min(blankLinesBefore(list[index]), p.options.BlankLines.Nested)
	}
	for index, statement := range list {
		if !isMajorDeclaration(statement) {
			continue
		}
		// A comment run written directly above a declaration documents it, so the
		// gap belongs before the comments rather than between them and the header.
		if start := commentRunStart(list, index); start > 0 {
			gaps[start] = p.options.BlankLines.TopLevel
		}
		if index+1 < len(list) {
			gaps[index+1] = p.options.BlankLines.TopLevel
		}
	}
	return gaps
}

// commentRunStart returns the index of the first comment in the unbroken run of
// comments directly above the statement at index.
func commentRunStart(list []ast.Statement, index int) int {
	start := index
	for start > 0 {
		if _, ok := list[start-1].(*ast.Comment); !ok {
			break
		}
		if blankLinesBefore(list[start]) != 0 {
			break
		}
		start--
	}
	return start
}

func blankLinesBefore(statement ast.Statement) int {
	if trivia := ast.TriviaOf(statement); trivia != nil {
		return trivia.BlankLinesBefore
	}
	return 0
}

// isMajorDeclaration reports whether the style guide requires blank lines around
// statement. Functions and inner classes are only ever declared as members, so
// the rule applies at every depth.
func isMajorDeclaration(statement ast.Statement) bool {
	switch statement.(type) {
	case *ast.FunctionDeclaration, *ast.ClassDeclaration:
		return true
	}
	return false
}

// statement renders one statement with its annotations and trailing comment.
func (p *printer) statement(statement ast.Statement) doc {
	var parts []doc
	for _, annotation := range ast.Annotations(statement) {
		parts = append(parts, p.annotation(annotation))
		if annotation.OwnLine {
			// An annotation on a line of its own carries the comment that ended
			// that line. One written ahead of the declaration shares its line, so
			// the declaration's own trailing comment is the only one there.
			if annotation.TrailingComment != nil {
				parts = append(parts, text("  "+p.commentText(annotation.TrailingComment)))
			}
			parts = append(parts, hardLine)
			continue
		}
		parts = append(parts, text(" "))
	}
	parts = append(parts, p.statementBody(statement))
	if trivia := ast.TriviaOf(statement); trivia != nil && trivia.TrailingComment != nil {
		parts = append(parts, text("  "+p.commentText(trivia.TrailingComment)))
	}
	return concat(parts...)
}

func (p *printer) annotation(node *ast.Annotation) doc {
	document := doc(text("@" + node.Name))
	// An argument list that holds only comments still has to be written out,
	// or those comments are lost.
	if node.Arguments != nil || len(node.Comments) > 0 {
		arguments := p.arguments(node.Arguments)
		document = concat(document, p.collection(argumentLayout, arguments, node.Comments))
	}
	// The comment that ended the annotation's line is written by whoever places
	// the line, since an annotation sharing a declaration's line has none of its
	// own.
	return document
}

func (p *printer) statementBody(statement ast.Statement) doc {
	switch node := statement.(type) {
	case *ast.Annotation:
		return p.annotation(node)
	case *ast.Comment:
		return text(p.commentText(node))
	case *ast.Directive:
		parts := []doc{text(node.Name)}
		if node.Value != nil {
			parts = append(parts, text(" "), p.expression(node.Value, 0))
		}
		if node.Extends != nil {
			parts = append(parts, text(" extends "), p.expression(node.Extends, 0))
		}
		return concat(parts...)
	case *ast.ExpressionStatement:
		return p.expression(node.Expression, 0)
	case *ast.VariableDeclaration:
		return p.variable(node)
	case *ast.Assignment:
		return concat(p.expression(node.Target, 0), text(" "+node.Operator+" "), p.expression(node.Value, 0))
	case *ast.ReturnStatement:
		if node.Value == nil {
			return text("return")
		}
		return concat(text("return "), p.expression(node.Value, 0))
	case *ast.KeywordStatement:
		return text(node.Keyword)
	case *ast.FunctionDeclaration:
		return p.function(node)
	case *ast.ClassDeclaration:
		header := "class " + node.Name
		if node.Extends != "" {
			header += " extends " + node.Extends
		}
		return concat(text(header+":"), p.suite(node.Body))
	case *ast.SignalDeclaration:
		document := doc(text("signal " + node.Name))
		if node.Parameters != nil || len(node.ParameterComments) > 0 {
			document = concat(document, p.parameterList(node.Parameters, node.ParameterComments))
		}
		return document
	case *ast.EnumDeclaration:
		return p.enum(node)
	case *ast.IfStatement:
		var parts []doc
		for index, branch := range node.Branches {
			keyword := "if "
			if index > 0 {
				parts = append(parts, hardLine)
				keyword = "elif "
			}
			// A comment above the keyword introduces the branch, so it stands on
			// its own line at the keyword's indentation.
			for _, comment := range branch.Comments {
				parts = append(parts, text(p.commentText(comment)), hardLine)
			}
			parts = append(parts,
				text(keyword),
				closeAfter(p.headerExpression(branch.Condition), ":"),
				p.suite(branch.Body),
			)
		}
		if node.Else != nil {
			parts = append(parts, hardLine)
			for _, comment := range node.ElseComments {
				parts = append(parts, text(p.commentText(comment)), hardLine)
			}
			parts = append(parts, text("else:"), p.suite(node.Else))
		}
		return concat(parts...)
	case *ast.WhileStatement:
		return concat(
			text("while "),
			closeAfter(p.headerExpression(node.Condition), ":"),
			p.suite(node.Body),
		)
	case *ast.ForStatement:
		variable := node.Variable
		if node.Type != "" {
			variable += ": " + node.Type
		}
		return concat(
			text("for "+variable+" in "),
			closeAfter(p.headerExpression(node.Iterable), ":"),
			p.suite(node.Body),
		)
	case *ast.MatchStatement:
		parts := []doc{text("match "), closeAfter(p.headerExpression(node.Value), ":")}
		// A comment that ended the "match" line stays on it.
		for _, comment := range ast.CollectionCommentsAt(node.Comments, 0, true) {
			parts = append(parts, text("  "+p.commentText(comment)))
		}
		var lines []doc
		for index, matchCase := range node.Cases {
			for _, comment := range ast.CollectionCommentsAt(node.Comments, index, false) {
				lines = append(lines, text(p.commentText(comment)))
			}
			if index > 0 {
				// Only the "match" line itself can hold a trailing comment, so
				// one anchored deeper takes a line of its own rather than being
				// dropped.
				for _, comment := range ast.CollectionCommentsAt(node.Comments, index, true) {
					lines = append(lines, text(p.commentText(comment)))
				}
			}
			for _, annotation := range matchCase.Annotations {
				lines = append(lines, p.statement(annotation))
			}
			lines = append(lines, p.matchCase(matchCase))
		}
		for _, trailing := range []bool{false, true} {
			for _, comment := range ast.CollectionCommentsAt(node.Comments, len(node.Cases), trailing) {
				lines = append(lines, text(p.commentText(comment)))
			}
		}
		return concat(concat(parts...), nest(1, concat(hardLine, join(hardLine, lines))))
	default:
		panic(fmt.Sprintf("format: unsupported statement %T", statement))
	}
}

func (p *printer) variable(node *ast.VariableDeclaration) doc {
	keyword := "var"
	if node.Constant {
		keyword = "const"
	}
	if node.Static {
		keyword = "static " + keyword
	}
	parts := []doc{text(keyword + " " + node.Name)}
	if node.Type != "" {
		parts = append(parts, text(": "+node.Type))
	}
	if node.Value != nil {
		operator := " = "
		if node.Inferred {
			operator = " := "
		}
		parts = append(parts, text(operator), p.expression(node.Value, 0))
	}
	header := concat(parts...)
	if shorthand := p.shorthandAccessors(node); shorthand != nil {
		return concat(header, text(": "), join(text(", "), shorthand))
	}
	names := shorthandAccessorNames(node)
	if node.Getter == nil && node.Setter == nil && len(names) == 0 {
		return header
	}
	// The accessor block's colon follows the initializer, so an initializer
	// ending in a comment is parenthesized to give the colon its own line, and
	// one ending in a block lambda's body to keep the colon out of that body.
	if node.Value != nil && (endsWithLineComment(header) || endsInsideBlock(node.Value)) {
		parts[len(parts)-1] = parenthesized(p.expression(node.Value, 0))
		header = concat(parts...)
	}
	var accessors []doc
	// A comment that ended the colon's line stays on it.
	var colon []doc
	colon = append(colon, text(":"))
	for _, comment := range ast.CollectionCommentsAt(node.AccessorComments, 0, true) {
		colon = append(colon, text("  "+p.commentText(comment)))
	}
	written := 0
	addComments := func(index int) {
		for _, trailing := range []bool{false, true} {
			if trailing && index == 0 {
				// Already written beside the colon.
				continue
			}
			for _, comment := range ast.CollectionCommentsAt(node.AccessorComments, index, trailing) {
				accessors = append(accessors, hardLine, text(p.commentText(comment)))
			}
		}
	}
	addComments(written)
	if node.Getter != nil {
		accessors = append(accessors, hardLine, text("get:"), p.suite(node.Getter))
		written++
		addComments(written)
	}
	if node.Setter != nil {
		accessors = append(accessors, hardLine, text("set("+node.Setter.Parameter+"):"), p.suite(node.Setter.Body))
		written++
		addComments(written)
	}
	for index, name := range names {
		// Godot reads a second named accessor only after a comma.
		if index < len(names)-1 {
			name += ","
		}
		accessors = append(accessors, hardLine, text(name))
		written++
		// A named accessor is one line, so the comment that ended it stays on
		// it, ahead of the ones written on the lines below.
		trailing := ast.CollectionCommentsAt(node.AccessorComments, written, true)
		if len(trailing) > 0 {
			accessors = append(accessors, text("  "+p.commentText(trailing[0])))
		}
		for _, comment := range trailing[min(1, len(trailing)):] {
			accessors = append(accessors, hardLine, text(p.commentText(comment)))
		}
		for _, comment := range ast.CollectionCommentsAt(node.AccessorComments, written, false) {
			accessors = append(accessors, hardLine, text(p.commentText(comment)))
		}
	}
	return concat(header, concat(colon...), nest(1, concat(accessors...)))
}

// shorthandAccessors returns the "get = method" accessors of node when they are
// to be written on the declaration's own line, and nil otherwise: a property
// written with accessor bodies, or one whose shorthand accessors were written in
// an indented block, keeps that block because it may also hold comments.
func (p *printer) shorthandAccessors(node *ast.VariableDeclaration) []doc {
	if node.AccessorBlock {
		return nil
	}
	var out []doc
	for _, name := range shorthandAccessorNames(node) {
		out = append(out, text(name))
	}
	return out
}

// shorthandAccessorNames returns the shorthand accessors of node in source
// order, each spelled as it is written.
func shorthandAccessorNames(node *ast.VariableDeclaration) []string {
	var out []string
	getter := node.GetterName != ""
	setter := node.SetterName != ""
	if getter && setter && node.SetterKeywordSpan.Start.Offset < node.GetterKeywordSpan.Start.Offset {
		return []string{"set = " + node.SetterName, "get = " + node.GetterName}
	}
	if getter {
		out = append(out, "get = "+node.GetterName)
	}
	if setter {
		out = append(out, "set = "+node.SetterName)
	}
	return out
}

func (p *printer) function(node *ast.FunctionDeclaration) doc {
	prefix := "func "
	if node.Static {
		prefix = "static func "
	}
	header := concat(text(prefix+node.Name), p.parameterList(node.Parameters, node.ParameterComments))
	if node.ReturnType != "" {
		header = concat(header, text(" -> "+node.ReturnType))
	}
	if node.Abstract {
		return header
	}
	return concat(header, text(":"), p.suite(node.Body))
}

func (p *printer) enum(node *ast.EnumDeclaration) doc {
	header := "enum"
	if node.Name != "" {
		header += " " + node.Name
	}
	members := make([]doc, len(node.Members))
	for index, member := range node.Members {
		if member.Value != nil {
			members[index] = concat(text(member.Name+" = "), p.expression(member.Value, 0))
			continue
		}
		members[index] = text(member.Name)
	}
	return concat(text(header+" "), p.collection(enumLayout, plainItems(members), node.Comments))
}

func (p *printer) matchCase(matchCase ast.MatchCase) doc {
	if len(matchCase.Patterns) == 0 {
		// A case holding no pattern is the bare "pass" of a match that handles
		// nothing.
		return text("pass")
	}
	// A separating comma, the "when" of a guard and the branch's own colon all
	// follow a pattern on its line, so a pattern or guard ending in a comment
	// is parenthesized to keep them off it.
	patterns := make([]doc, len(matchCase.Patterns))
	for index, pattern := range matchCase.Patterns {
		patterns[index] = p.headerExpression(pattern)
	}
	header := group(join(matchPatternSeparator(), patterns))
	if matchCase.Guard != nil {
		header = concat(header, text(" when "), p.headerExpression(matchCase.Guard))
	}
	return concat(closeAfter(header, ":"), p.suite(matchCase.Body))
}

// matchPatternSeparator renders the comma between two patterns. A bare line
// break inside a pattern list ends the list as far as Godot is concerned, and a
// parenthesized group is not a pattern, so a backslash continuation is the only
// way the list can be broken.
//
// A continued pattern is not indented, so the patterns of one case line up
// however many lines they take and the body stays the only thing set in from
// them. The style guide asks for two levels on a continuation to keep it from
// reading as a block, which here would set the patterns in deeper than the body
// they guard; the trailing backslash already says the line continues, and it
// says so at the end of the line, where the eye leaves it.
func matchPatternSeparator() doc {
	return concat(text(","), ifBroken(text(" \\"), text("")), spaceLine)
}

// layout describes how a bracketed construct is broken across lines.
type layout struct {
	open  string
	close string
	// levels is the continuation indentation. The style guide asks for two
	// levels, and one inside arrays, dictionaries, and enums.
	levels int
	// trailingComma appends a comma after the last element when broken, which
	// the style guide asks for in arrays, dictionaries, and enums only.
	trailingComma bool
	// padFlat adds a space inside the braces of a single-line declaration, which
	// the style guide asks for in dictionaries.
	padFlat bool
}

var (
	argumentLayout   = layout{open: "(", close: ")", levels: 2}
	arrayLayout      = layout{open: "[", close: "]", levels: 1, trailingComma: true}
	dictionaryLayout = layout{open: "{", close: "}", levels: 1, trailingComma: true, padFlat: true}
	enumLayout       = layout{open: "{", close: "}", levels: 1, trailingComma: true, padFlat: true}
)

// holdsAnyLastLine reports whether any item holds the line it ends on, which
// leaves no room on that line for the construct's own closing bracket.
func holdsAnyLastLine(items []item) bool {
	for _, member := range items {
		if member.holdsItsLastLine() {
			return true
		}
	}
	return false
}

// arguments renders an expression list as collection items.
func (p *printer) arguments(expressions []ast.Expression) []item {
	items := make([]item, len(expressions))
	p.inBrackets(func() {
		for index, expression := range expressions {
			items[index] = item{
				doc:         p.expression(expression, 0),
				insideMatch: endsInsideMatch(expression),
				insideBlock: endsInsideBlock(expression),
			}
		}
	})
	return items
}

// item is one member of a bracketed construct.
type item struct {
	doc doc
	// insideMatch reports that the item's last line lies inside the case list
	// of a match statement. Godot reads a comma written there as another
	// pattern, so such an item holds its last line as surely as one ending in
	// a comment does.
	insideMatch bool
	// insideBlock reports that the item's last line lies inside the body of a
	// block lambda, which the bracket after it has to leave.
	insideBlock bool
}

// plainItems wraps documents that cannot end inside a match statement.
func plainItems(docs []doc) []item {
	items := make([]item, len(docs))
	for index, d := range docs {
		items[index] = item{doc: d}
	}
	return items
}

// holdsItsLastLine reports whether a comma written straight after it would be
// read as part of it rather than as the separator after it.
func (i item) holdsItsLastLine() bool {
	return endsWithLineComment(i.doc) || i.insideMatch
}

// endsInsideMatch reports whether the last line expr emits lies inside the case
// list of a match statement. A lambda reached through an operator or a call is
// parenthesized, and the parenthesis closes the case list before the item ends,
// so the lambda written as the item itself is the case this answers. A lambda
// held by a subscript index is emitted unparenthesized and so ends an item the
// same way, which #46 covers.
func endsInsideMatch(expr ast.Expression) bool {
	lambda, ok := expr.(*ast.LambdaExpression)
	return ok && !lambda.Inline && statementsEndInsideMatch(lambda.Body)
}

// endsInsideBlock reports whether the last line expr emits lies inside the body
// of a block lambda. As with endsInsideMatch, a lambda reached through an
// operator is parenthesized, so only the lambda written as the item itself
// leaves its block open.
func endsInsideBlock(expr ast.Expression) bool {
	lambda, ok := expr.(*ast.LambdaExpression)
	return ok && !fitsOneLine(lambda)
}

// fitsOneLine reports whether lambda is written with its body on its own line.
// A comment ends the line it is written on, so an inline body may hold one only
// as its last statement, where it stays at the end of the line. A body the
// single-line form cannot hold is written out as a block instead.
func fitsOneLine(lambda *ast.LambdaExpression) bool {
	body, _ := splitTrailingComment(lambda.Body)
	return lambda.Inline && len(body) > 0 && inlinable(body)
}

// needsClosingComma reports whether the last item of a construct must be
// followed by a comma for the construct's bracket to be readable. The body of a
// block lambda ends where the indentation steps back to one the engine knows,
// and a bracket nested in another construct sits on a continuation indentation
// it does not, so there the comma ends the body instead.
func (p *printer) needsClosingComma(last item) bool {
	return last.insideBlock && p.brackets > 0
}

// statementsEndInsideMatch reports whether the last statement of body leaves a
// match statement's case list open, directly or through the blocks that end
// with it. A comment written at the end of the body is emitted inside the case
// body it was written in, so the case list is still open behind it.
func statementsEndInsideMatch(body []ast.Statement) bool {
	body, _ = splitTrailingComment(body)
	if len(body) == 0 {
		return false
	}
	switch node := body[len(body)-1].(type) {
	case *ast.MatchStatement:
		return true
	case *ast.IfStatement:
		if len(node.Else) > 0 {
			return statementsEndInsideMatch(node.Else)
		}
		if len(node.Branches) > 0 {
			return statementsEndInsideMatch(node.Branches[len(node.Branches)-1].Body)
		}
	case *ast.ForStatement:
		return statementsEndInsideMatch(node.Body)
	case *ast.WhileStatement:
		return statementsEndInsideMatch(node.Body)
	case *ast.VariableDeclaration:
		return node.Value != nil && endsInsideMatch(node.Value)
	case *ast.Assignment:
		return endsInsideMatch(node.Value)
	case *ast.ReturnStatement:
		return node.Value != nil && endsInsideMatch(node.Value)
	case *ast.ExpressionStatement:
		return endsInsideMatch(node.Expression)
	}
	return false
}

// closingComma renders the comma that follows an item holding the line it ends
// on. Written straight after such an item the comma is read as part of it, and
// written at the item's own indentation the engine cannot leave the block, so
// it takes the next line indented with the item's last line.
//
// It is written whether or not the construct would otherwise take a trailing
// comma. Leaving the block means unindenting, and the engine takes that step
// only to an indentation it already knows: where the construct's bracket
// follows at a statement's indentation the comma may be dropped, but where the
// construct is nested inside another the bracket sits at a continuation
// indentation instead, and only the comma's line supplies the step.
func closingComma() doc {
	return nest(1, concat(hardLine, text(",")))
}

// collection renders items inside brackets, on one line when they fit and one
// per line otherwise. Comments written between the brackets are anchored to the
// items they were written against.
func (p *printer) collection(shape layout, items []item, comments []ast.CollectionComment) doc {
	if len(comments) > 0 {
		return p.commentedCollection(shape, items, comments)
	}
	if len(items) == 0 {
		return text(shape.open + shape.close)
	}
	var tail doc
	switch last := items[len(items)-1]; {
	case last.holdsItsLastLine():
		// The comma is written on a line of its own, below.
	case shape.trailingComma && p.options.TrailingCommas == TrailingCommasWhenBroken:
		tail = ifBroken(text(","), text(""))
	case p.needsClosingComma(last):
		tail = text(",")
	}
	separated := make([]doc, 0, len(items)*3)
	for index, member := range items {
		if index > 0 {
			separated = append(separated, spaceLine)
		}
		separated = append(separated, member.doc)
		if member.holdsItsLastLine() {
			separated = append(separated, closingComma())
			continue
		}
		if index < len(items)-1 {
			separated = append(separated, text(","))
		}
	}
	pad := doc(text(""))
	if shape.padFlat {
		pad = ifBroken(text(""), text(" "))
	}
	// An item ending in a comment cannot share its line with what follows it,
	// not even the closing bracket, so the construct breaks however short it
	// is.
	opening, closing := doc(softLine), doc(softLine)
	if holdsAnyLastLine(items) {
		opening, closing = hardLine, hardLine
	}
	return group(concat(
		text(shape.open),
		pad,
		nest(shape.levels, concat(opening, concat(separated...), tail)),
		closing,
		pad,
		text(shape.close),
	))
}

// commentedCollection renders a bracketed construct that holds comments. Such a
// construct always breaks, because a comment can only survive on a line of its
// own or at the end of the line it was written on.
func (p *printer) commentedCollection(shape layout, items []item, comments []ast.CollectionComment) doc {
	var lines []doc
	for index, member := range items {
		for _, comment := range ast.CollectionCommentsAt(comments, index, false) {
			lines = append(lines, text(p.commentText(comment)))
		}
		parts := []doc{member.doc}
		switch {
		case member.holdsItsLastLine():
			parts = append(parts, closingComma())
		case index < len(items)-1:
			parts = append(parts, text(","))
		case shape.trailingComma && p.options.TrailingCommas == TrailingCommasWhenBroken:
			parts = append(parts, text(","))
		case p.needsClosingComma(member):
			parts = append(parts, text(","))
		}
		// Only one comment fits at the end of a line: a second would be read as
		// part of the first, so it takes a line of its own.
		trailing := ast.CollectionCommentsAt(comments, index+1, true)
		if len(trailing) > 0 {
			parts = append(parts, text("  "+p.commentText(trailing[0])))
		}
		lines = append(lines, concat(parts...))
		for _, comment := range trailing[min(1, len(trailing)):] {
			lines = append(lines, text(p.commentText(comment)))
		}
	}
	for _, comment := range ast.CollectionCommentsAt(comments, len(items), false) {
		lines = append(lines, text(p.commentText(comment)))
	}
	opening := []doc{text(shape.open)}
	// A comment that ended the opening line stays there: no item precedes it to
	// hold it, and the loop above only reaches a trailing comment that follows
	// an item.
	for _, comment := range ast.CollectionCommentsAt(comments, 0, true) {
		opening = append(opening, text("  "+p.commentText(comment)))
	}
	if len(lines) == 0 {
		return concat(concat(opening...), hardLine, text(shape.close))
	}
	return concat(
		concat(opening...),
		nest(shape.levels, concat(hardLine, join(hardLine, lines))),
		hardLine,
		text(shape.close),
	)
}

func (p *printer) parameterList(parameters []ast.Parameter, comments []ast.CollectionComment) doc {
	items := make([]item, len(parameters))
	p.inBrackets(func() {
		for index, parameter := range parameters {
			items[index] = item{doc: p.parameter(parameter)}
			if parameter.Default != nil {
				items[index].insideMatch = endsInsideMatch(parameter.Default)
				items[index].insideBlock = endsInsideBlock(parameter.Default)
			}
		}
	})
	return p.collection(argumentLayout, items, comments)
}

func (p *printer) parameter(parameter ast.Parameter) doc {
	header := parameter.Name
	if parameter.Variadic {
		header = "..." + header
	}
	if parameter.Type != "" {
		header += ": " + parameter.Type
	}
	if parameter.Default == nil {
		return text(header)
	}
	operator := " = "
	if parameter.Inferred {
		operator = " := "
	}
	return concat(text(header+operator), p.expression(parameter.Default, 0))
}

// chain is a run of operators that bind at one level, held as the operands they
// separate so that it can be rendered either whole or broken before each
// operator. Operator i stands between operands i and i+1.
type chain struct {
	operands  []doc
	operators []string
}

// whole renders the chain on one line.
func (c chain) whole() doc {
	parts := []doc{c.operands[0]}
	for index, operator := range c.operators {
		parts = append(parts, text(" "+operator+" "), c.operands[index+1])
	}
	return concat(parts...)
}

// broken renders the chain with a break before each operator, so that the
// operator starts its continuation line rather than ending the line above it.
func (c chain) broken() doc {
	parts := []doc{c.operands[0]}
	for index, operator := range c.operators {
		parts = append(parts, spaceLine, text(operator+" "), c.operands[index+1])
	}
	return concat(parts...)
}

// holdsABreak reports whether any operand can continue across lines on its own.
func (c chain) holdsABreak() bool {
	for _, operand := range c.operands {
		if holdsABreak(operand) {
			return true
		}
	}
	return false
}

// parenthesizedChain renders a chain inside the parentheses precedence requires
// around it, breaking before each operator. The chain breaks only when it cannot
// fit the line by itself: where a call after it could take the break instead, the
// chain is the shorter read and stays whole.
func parenthesizedChain(links chain) doc {
	body, closing := chainBody(links)
	return yieldingGroup(concat(text("("), body, closing, text(")")))
}

// wrappedChain renders a chain that precedence has not parenthesized. An
// expression can only continue across lines inside parentheses, so breaking
// adds them.
func wrappedChain(links chain) doc {
	body, closing := chainBody(links)
	return wrappingGroup(concat(
		ifBroken(text("("), text("")),
		body,
		closing,
		ifBroken(text(")"), text("")),
	))
}

// chainBody returns the indented contents of a broken chain and the line that
// closes them. Continuation lines are part of the same expression, so they take
// two indentation levels.
func chainBody(links chain) (doc, doc) {
	body := nest(2, concat(softLine, links.broken()))
	if endsWithLineComment(body) {
		return body, hardLine
	}
	return body, softLine
}

// binaryChain flattens a chain of binary operators that bind at one level into
// the operands they separate. Every binary operator is left-associative, so the
// chain runs down the left operands. An operand that binds tighter stays whole,
// so a chain of mixed levels breaks only at its loosest operator and never reads
// as though the parentheses had regrouped it.
func (p *printer) binaryChain(binary *ast.BinaryExpression) chain {
	operator := p.operatorText(binary.Operator)
	precedence := ast.OperatorPrecedence(operator)
	var links chain
	left, ok := binary.Left.(*ast.BinaryExpression)
	if ok && ast.OperatorPrecedence(p.operatorText(left.Operator)) == precedence {
		links = p.binaryChain(left)
	} else {
		links.operands = append(links.operands, p.expression(binary.Left, precedence))
	}
	links.operators = append(links.operators, operator)
	links.operands = append(links.operands, p.expression(binary.Right, precedence+1))
	return links
}

// ternaryChain flattens a conditional expression and the conditionals its
// alternative continues into, separated by their "else" keywords. A value stays
// with the condition that chooses it, which is the shape the style guide's own
// wrapped conditional takes.
func (p *printer) ternaryChain(ternary *ast.TernaryExpression) chain {
	links := chain{operands: []doc{concat(
		p.expression(ternary.Value, ast.PrecedenceTernary+1), text(" if "),
		p.expression(ternary.Condition, ast.PrecedenceTernary),
	)}}
	links.operators = append(links.operators, "else")
	if alternative, ok := ternary.Alternative.(*ast.TernaryExpression); ok {
		rest := p.ternaryChain(alternative)
		links.operands = append(links.operands, rest.operands...)
		links.operators = append(links.operators, rest.operators...)
		return links
	}
	links.operands = append(links.operands, p.expression(ternary.Alternative, ast.PrecedenceTernary))
	return links
}

func (p *printer) expression(expr ast.Expression, parentPrecedence int) doc {
	if expr == nil {
		return text("")
	}
	switch node := expr.(type) {
	case *ast.Identifier:
		return text(node.Name)
	case *ast.TypeExpression:
		return text(node.Name)
	case *ast.Literal:
		return text(p.literal(node))
	case *ast.NodePathExpression:
		prefix := "$"
		if node.Unique {
			prefix = "%"
		}
		return text(prefix + node.Path)
	case *ast.UnaryExpression:
		operator := p.operatorText(node.Operator)
		// A prefix operator reads its operand at one level, which is also the
		// level the whole unary expression binds at, so the same number decides
		// both whether the operand needs parentheses and whether this does.
		precedence := ast.UnaryOperandPrecedence(operator)
		spelled := operator
		if operator == "not" || operator == "await" {
			spelled += " "
		}
		inner := concat(text(spelled), p.expression(node.Operand, precedence))
		if precedence < parentPrecedence {
			return parenthesized(inner)
		}
		return inner
	case *ast.BinaryExpression:
		operator := p.operatorText(node.Operator)
		var links chain
		p.inBrackets(func() { links = p.binaryChain(node) })
		if ast.OperatorPrecedence(operator) < parentPrecedence {
			return parenthesizedChain(links)
		}
		// The style guide names where and/or belongs on a continuation line, so
		// a logical chain takes the break even when something inside it could.
		if !isLogicalOperator(operator) && links.holdsABreak() {
			return links.whole()
		}
		return wrappedChain(links)
	case *ast.TernaryExpression:
		var links chain
		p.inBrackets(func() { links = p.ternaryChain(node) })
		if ast.PrecedenceTernary < parentPrecedence {
			return parenthesizedChain(links)
		}
		if links.holdsABreak() {
			return links.whole()
		}
		return wrappedChain(links)
	case *ast.CallExpression:
		return concat(
			p.expression(node.Callee, ast.PrecedenceCall),
			p.collection(argumentLayout, p.arguments(node.Arguments), node.Comments),
		)
	case *ast.MemberExpression:
		return closeAfter(p.expression(node.Object, ast.PrecedenceAttribute), "."+node.Property)
	case *ast.SubscriptExpression:
		// A subscript has no comma to end the body of a block lambda written as
		// its index, so such a lambda is parenthesized, and the parenthesis ends
		// the body before the bracket has to.
		indexPrecedence := 0
		if endsInsideBlock(node.Index) {
			indexPrecedence = ast.PrecedenceSubscript
		}
		return concat(
			p.expression(node.Object, ast.PrecedenceSubscript),
			text("["),
			closeAfter(p.expression(node.Index, indexPrecedence), "]"),
		)
	case *ast.ArrayLiteral:
		return p.collection(arrayLayout, p.arguments(node.Elements), node.Comments)
	case *ast.DictionaryLiteral:
		separator := ": "
		if node.LuaStyle {
			separator = " = "
		}
		entries := make([]item, len(node.Entries))
		p.inBrackets(func() {
			for index, entry := range node.Entries {
				if entry.Value == nil {
					// A dictionary pattern may test that a key is present without
					// constraining its value, which is written as the key alone.
					entries[index] = item{doc: p.headerExpression(entry.Key)}
					continue
				}
				entries[index] = item{
					doc: concat(
						closeAfter(p.headerExpression(entry.Key), separator),
						p.expression(entry.Value, 0),
					),
					insideMatch: endsInsideMatch(entry.Value),
					insideBlock: endsInsideBlock(entry.Value),
				}
			}
		})
		return p.collection(dictionaryLayout, entries, node.Comments)
	case *ast.BindingPattern:
		return text("var " + node.Name)
	case *ast.WildcardPattern:
		return text("_")
	case *ast.RestPattern:
		return text("..")
	case *ast.LambdaExpression:
		keyword := "func"
		if node.Name != "" {
			keyword += " " + node.Name
		}
		header := concat(text(keyword), p.parameterList(node.Parameters, node.ParameterComments))
		if node.ReturnType != "" {
			header = concat(header, text(" -> "+node.ReturnType))
		}
		var inner doc
		// The body is a block of its own, so the brackets around the lambda are
		// not around the statements in it.
		brackets := p.brackets
		p.brackets = 0
		defer func() { p.brackets = brackets }()
		body, trailing := splitTrailingComment(node.Body)
		if fitsOneLine(node) {
			statements := make([]doc, len(body))
			for index, statement := range body {
				statements[index] = p.inlineStatement(statement)
			}
			inner = concat(header, text(": "), join(text("; "), statements))
			if trailing != nil {
				inner = concat(inner, text("  "+p.commentText(trailing)))
			}
		} else {
			inner = concat(header, text(":"), p.suite(node.Body))
		}
		if parentPrecedence > 0 {
			if statementsEndInsideMatch(node.Body) {
				// The closing parenthesis would land in the case list the body
				// ends inside, where it is read as another pattern, so it takes
				// the next line, indented with the body it closes.
				return group(concat(text("("), inner, nest(1, concat(hardLine, text(")")))))
			}
			return parenthesized(inner)
		}
		return inner
	default:
		panic(fmt.Sprintf("format: unsupported expression %T", expr))
	}
}

// splitTrailingComment separates a trailing comment from the statements before
// it, which is how a comment reaches an inline lambda body.
func splitTrailingComment(body []ast.Statement) ([]ast.Statement, *ast.Comment) {
	if len(body) == 0 {
		return body, nil
	}
	comment, ok := body[len(body)-1].(*ast.Comment)
	if !ok {
		return body, nil
	}
	return body[:len(body)-1], comment
}

// inlinable reports whether every statement of body can be written on one line.
func inlinable(body []ast.Statement) bool {
	for _, statement := range body {
		switch statement.(type) {
		case *ast.ExpressionStatement, *ast.Assignment, *ast.ReturnStatement,
			*ast.KeywordStatement, *ast.VariableDeclaration:
		default:
			return false
		}
	}
	return true
}

// inlineStatement renders a statement inside a single-line lambda body. Only a
// statement inlinable reports on reaches it, so the panic marks a tree the
// formatter does not support rather than input it cannot format.
func (p *printer) inlineStatement(statement ast.Statement) doc {
	switch statement.(type) {
	case *ast.ExpressionStatement, *ast.Assignment, *ast.ReturnStatement,
		*ast.KeywordStatement, *ast.VariableDeclaration:
		return p.statementBody(statement)
	default:
		panic(fmt.Sprintf("format: unsupported inline statement %T", statement))
	}
}

func parenthesized(inner doc) doc {
	if endsWithLineComment(inner) {
		// The comment holds the rest of its line, and the parenthesis closes a
		// block the comment was written inside, so it takes the next line
		// indented with that block, as the comma after such an item does.
		return group(concat(text("("), inner, nest(1, concat(hardLine, text(")")))))
	}
	return group(concat(text("("), closeAfter(inner, ")")))
}

// headerExpression renders an expression that a mandatory token follows, such
// as the colon of a statement header or of a dictionary entry. A comment holds
// the rest of its line, so an expression ending in one is parenthesized, which
// gives the token a line of its own to land on and keeps the expression
// readable to the parser as well as to the engine.
func (p *printer) headerExpression(expr ast.Expression) doc {
	rendered := p.expression(expr, 0)
	if endsWithLineComment(rendered) {
		return parenthesized(rendered)
	}
	return rendered
}

// closeAfter appends a closing token to inner, on the next line when inner ends
// in a comment. A comment holds the rest of its line, so a token written after
// one on the same line is commented out.
func closeAfter(inner doc, closing string) doc {
	if endsWithLineComment(inner) {
		return concat(inner, hardLine, text(closing))
	}
	return concat(inner, text(closing))
}

func isLogicalOperator(operator string) bool {
	switch operator {
	case "and", "or", "&&", "||":
		return true
	}
	return false
}

// operatorText returns the spelling of operator, preferring the style guide's
// plain English boolean operators.
func (p *printer) operatorText(operator string) string {
	if p.options.Operators != WordOperators {
		return operator
	}
	switch operator {
	case "&&":
		return "and"
	case "||":
		return "or"
	case "!":
		return "not"
	}
	return operator
}

func (p *printer) literal(node *ast.Literal) string {
	switch node.Kind {
	case ast.IntegerLiteral, ast.FloatLiteral:
		if p.options.Numbers == NormalizeNumbers {
			return normalizeNumber(node.Raw)
		}
	case ast.StringLiteral, ast.StringNameLiteral, ast.NodePathLiteral:
		return p.requote(node)
	}
	return node.Raw
}

// normalizeNumber restores omitted zeros in a float and lowercases a radix
// prefix and its digits. Digit separators are never added or removed.
func normalizeNumber(raw string) string {
	if len(raw) > 1 && raw[0] == '0' {
		switch raw[1] {
		case 'x', 'X', 'b', 'B':
			return "0" + strings.ToLower(raw[1:])
		}
	}
	mantissa, exponent := raw, ""
	if index := strings.IndexAny(raw, "eE"); index >= 0 {
		mantissa, exponent = raw[:index], raw[index:]
	}
	if strings.HasPrefix(mantissa, ".") {
		mantissa = "0" + mantissa
	}
	if strings.HasSuffix(mantissa, ".") {
		mantissa += "0"
	}
	return mantissa + exponent
}

// requote rewrites a string literal to the quote character that needs the fewest
// escapes, preferring the configured style when either works. The value of the
// string is never changed.
func (p *printer) requote(node *ast.Literal) string {
	if p.options.QuoteStyle == PreserveQuotes || node.Triple || node.Quote == 0 {
		return node.Raw
	}
	open := strings.IndexByte(node.Raw, node.Quote)
	if open < 0 || len(node.Raw) < open+2 {
		return node.Raw
	}
	prefix, body := node.Raw[:open], node.Raw[open+1:len(node.Raw)-1]
	preferred, alternate := byte('"'), byte('\'')
	if p.options.QuoteStyle == SingleQuotes {
		preferred, alternate = alternate, preferred
	}
	if node.RawPrefix {
		// A raw literal has no escapes, so its quote character must not appear in
		// the body at all.
		switch {
		case !strings.Contains(body, string(preferred)):
			return prefix + string(preferred) + body + string(preferred)
		case !strings.Contains(body, string(alternate)):
			return prefix + string(alternate) + body + string(alternate)
		default:
			return node.Raw
		}
	}
	atoms := splitEscapes(body)
	quote := preferred
	if countLiteral(atoms, preferred) > 0 && countLiteral(atoms, alternate) == 0 {
		quote = alternate
	}
	return prefix + string(quote) + encodeString(atoms, quote) + string(quote)
}

// splitEscapes splits a string literal body into atoms, reducing an escaped
// quote to the bare character it stands for so that the choice of delimiter can
// be made independently of how the source happened to escape it.
func splitEscapes(body string) []string {
	atoms := make([]string, 0, len(body))
	for index := 0; index < len(body); index++ {
		if body[index] != '\\' || index+1 >= len(body) {
			atoms = append(atoms, body[index:index+1])
			continue
		}
		if next := body[index+1]; next == '"' || next == '\'' {
			atoms = append(atoms, string(next))
		} else {
			atoms = append(atoms, body[index:index+2])
		}
		index++
	}
	return atoms
}

// countLiteral returns how many atoms are an unescaped occurrence of target.
func countLiteral(atoms []string, target byte) int {
	count := 0
	for _, atom := range atoms {
		if len(atom) == 1 && atom[0] == target {
			count++
		}
	}
	return count
}

// encodeString reassembles atoms into a body delimited by quote, escaping only
// the delimiter itself.
func encodeString(atoms []string, quote byte) string {
	var builder strings.Builder
	for _, atom := range atoms {
		if len(atom) == 1 && atom[0] == quote {
			builder.WriteByte('\\')
		}
		builder.WriteString(atom)
	}
	return builder.String()
}

// commentText returns the comment's source text with one space after its marker.
// Region markers are left alone, since the style guide requires no space there.
func (p *printer) commentText(comment *ast.Comment) string {
	raw := comment.Text
	if p.options.CommentSpacing != NormalizeComments || !strings.HasPrefix(raw, "#") {
		return raw
	}
	marker, body := "#", raw[1:]
	if strings.HasPrefix(body, "#") {
		marker, body = "##", body[1:]
	}
	if strings.HasPrefix(body, "region") || strings.HasPrefix(body, "endregion") {
		return raw
	}
	if body == "" || strings.HasPrefix(body, " ") {
		return raw
	}
	return marker + " " + body
}
