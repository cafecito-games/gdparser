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

type printer struct{ options Options }

// statements renders a statement list with the blank lines between its members,
// and returns nil when the list is empty.
func (p *printer) statements(list []ast.Statement) doc {
	if len(list) == 0 {
		return nil
	}
	gaps := p.blankLineGaps(list)
	parts := make([]doc, 0, len(list)*2)
	for index, statement := range list {
		if index > 0 {
			for range gaps[index] + 1 {
				parts = append(parts, hardLine)
			}
		}
		parts = append(parts, p.statement(statement))
	}
	return concat(parts...)
}

// body renders a suite's statements, substituting pass for an empty one.
func (p *printer) body(list []ast.Statement) doc {
	if document := p.statements(list); document != nil {
		return document
	}
	return text("pass")
}

// suite renders an indented block beneath a statement header.
func (p *printer) suite(list []ast.Statement) doc {
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
		arguments := make([]doc, len(node.Arguments))
		for index, argument := range node.Arguments {
			arguments[index] = p.expression(argument, 0)
		}
		document = concat(document, p.collection(argumentLayout, arguments, node.Comments))
	}
	if node.TrailingComment != nil {
		document = concat(document, text("  "+p.commentText(node.TrailingComment)))
	}
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
			parts = append(parts, text(keyword), p.expression(branch.Condition, 0), text(":"), p.suite(branch.Body))
		}
		if node.Else != nil {
			parts = append(parts, hardLine, text("else:"), p.suite(node.Else))
		}
		return concat(parts...)
	case *ast.WhileStatement:
		return concat(text("while "), p.expression(node.Condition, 0), text(":"), p.suite(node.Body))
	case *ast.ForStatement:
		variable := node.Variable
		if node.Type != "" {
			variable += ": " + node.Type
		}
		return concat(text("for "+variable+" in "), p.expression(node.Iterable, 0), text(":"), p.suite(node.Body))
	case *ast.MatchStatement:
		parts := []doc{text("match "), p.expression(node.Value, 0), text(":")}
		cases := make([]doc, len(node.Cases))
		for index, matchCase := range node.Cases {
			cases[index] = p.matchCase(matchCase)
		}
		return concat(concat(parts...), nest(1, concat(hardLine, join(hardLine, cases))))
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
	if node.Getter == nil && node.Setter == nil {
		return header
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
	return concat(header, concat(colon...), nest(1, concat(accessors...)))
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
	return concat(text(header+" "), p.collection(enumLayout, members, node.Comments))
}

func (p *printer) matchCase(matchCase ast.MatchCase) doc {
	patterns := make([]doc, len(matchCase.Patterns))
	for index, pattern := range matchCase.Patterns {
		patterns[index] = p.expression(pattern, 0)
	}
	header := join(text(", "), patterns)
	if matchCase.Guard != nil {
		header = concat(header, text(" when "), p.expression(matchCase.Guard, 0))
	}
	return concat(header, text(":"), p.suite(matchCase.Body))
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

// collection renders items inside brackets, on one line when they fit and one
// per line otherwise. Comments written between the brackets are anchored to the
// items they were written against.
func (p *printer) collection(shape layout, items []doc, comments []ast.CollectionComment) doc {
	if len(comments) > 0 {
		return p.commentedCollection(shape, items, comments)
	}
	if len(items) == 0 {
		return text(shape.open + shape.close)
	}
	var tail doc
	if shape.trailingComma && p.options.TrailingCommas == TrailingCommasWhenBroken {
		// An optional comma is dropped after an item that ends in a comment,
		// which would otherwise swallow it, rather than stranded on a line of
		// its own for the sake of a style preference.
		if !endsWithLineComment(items[len(items)-1]) {
			tail = ifBroken(text(","), text(""))
		}
	}
	separated := make([]doc, 0, len(items)*3)
	for index, item := range items {
		if index > 0 {
			separated = append(separated, spaceLine)
		}
		separated = append(separated, item)
		if index == len(items)-1 {
			continue
		}
		// A separating comma is required, so an item ending in a comment moves
		// it onto the next line instead of losing it.
		if endsWithLineComment(item) {
			separated = append(separated, hardLine)
		}
		separated = append(separated, text(","))
	}
	pad := doc(text(""))
	if shape.padFlat {
		pad = ifBroken(text(""), text(" "))
	}
	return group(concat(
		text(shape.open),
		pad,
		nest(shape.levels, concat(softLine, concat(separated...), tail)),
		softLine,
		pad,
		text(shape.close),
	))
}

// commentedCollection renders a bracketed construct that holds comments. Such a
// construct always breaks, because a comment can only survive on a line of its
// own or at the end of the line it was written on.
func (p *printer) commentedCollection(shape layout, items []doc, comments []ast.CollectionComment) doc {
	var lines []doc
	for index, item := range items {
		for _, comment := range ast.CollectionCommentsAt(comments, index, false) {
			lines = append(lines, text(p.commentText(comment)))
		}
		parts := []doc{item}
		switch {
		case index < len(items)-1:
			// A separating comma is required, so an item ending in a comment
			// moves it onto the next line instead of losing it.
			if endsWithLineComment(item) {
				parts = append(parts, hardLine)
			}
			parts = append(parts, text(","))
		case shape.trailingComma && p.options.TrailingCommas == TrailingCommasWhenBroken && !endsWithLineComment(item):
			parts = append(parts, text(","))
		}
		for _, comment := range ast.CollectionCommentsAt(comments, index+1, true) {
			parts = append(parts, text("  "+p.commentText(comment)))
		}
		lines = append(lines, concat(parts...))
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
	items := make([]doc, len(parameters))
	for index, parameter := range parameters {
		items[index] = p.parameter(parameter)
	}
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
	return concat(text(header+" = "), p.expression(parameter.Default, 0))
}

// logicalChain renders a chain of one logical operator, breaking before each
// keyword so that and/or starts its continuation line. A logical expression can
// only continue across lines inside parentheses, so breaking always adds them
// when precedence has not already required them.
func (p *printer) logicalChain(binary *ast.BinaryExpression, parenthesize bool) doc {
	body := nest(2, concat(softLine, concat(p.logicalParts(binary)...)))
	if parenthesize {
		return group(concat(text("("), body, softLine, text(")")))
	}
	return group(concat(
		ifBroken(text("("), text("")),
		body,
		softLine,
		ifBroken(text(")"), text("")),
	))
}

// logicalParts flattens a chain of one logical operator into operands separated
// by breakable operator keywords.
func (p *printer) logicalParts(binary *ast.BinaryExpression) []doc {
	operator := p.operatorText(binary.Operator)
	precedence := operatorPrecedence(operator)
	var parts []doc
	if left, ok := binary.Left.(*ast.BinaryExpression); ok && p.operatorText(left.Operator) == operator {
		parts = p.logicalParts(left)
	} else {
		parts = append(parts, p.expression(binary.Left, precedence))
	}
	return append(parts, spaceLine, text(operator+" "), p.expression(binary.Right, precedence+1))
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
		operandPrecedence := 11
		if operator == "not" {
			operandPrecedence = 3
		}
		spelled := operator
		if operator == "not" || operator == "await" {
			spelled += " "
		}
		inner := concat(text(spelled), p.expression(node.Operand, operandPrecedence))
		if operator == "not" && parentPrecedence >= 3 {
			return parenthesized(inner)
		}
		if 11 < parentPrecedence {
			return parenthesized(inner)
		}
		return inner
	case *ast.BinaryExpression:
		operator := p.operatorText(node.Operator)
		precedence := operatorPrecedence(operator)
		leftPrecedence, rightPrecedence := precedence, precedence+1
		if operator == "**" {
			leftPrecedence, rightPrecedence = precedence+1, precedence
		}
		if isLogicalOperator(operator) {
			return p.logicalChain(node, precedence < parentPrecedence)
		}
		inner := concat(
			p.expression(node.Left, leftPrecedence),
			text(" "+operator+" "),
			p.expression(node.Right, rightPrecedence),
		)
		if precedence < parentPrecedence {
			return parenthesized(inner)
		}
		return inner
	case *ast.TernaryExpression:
		inner := concat(
			p.expression(node.Value, 1), text(" if "),
			p.expression(node.Condition, 1), text(" else "),
			p.expression(node.Alternative, 1),
		)
		if parentPrecedence > 0 {
			return parenthesized(inner)
		}
		return inner
	case *ast.CallExpression:
		arguments := make([]doc, len(node.Arguments))
		for index, argument := range node.Arguments {
			arguments[index] = p.expression(argument, 0)
		}
		return concat(p.expression(node.Callee, 12), p.collection(argumentLayout, arguments, node.Comments))
	case *ast.MemberExpression:
		return concat(p.expression(node.Object, 12), text("."+node.Property))
	case *ast.SubscriptExpression:
		return concat(p.expression(node.Object, 12), text("["), p.expression(node.Index, 0), text("]"))
	case *ast.ArrayLiteral:
		elements := make([]doc, len(node.Elements))
		for index, element := range node.Elements {
			elements[index] = p.expression(element, 0)
		}
		return p.collection(arrayLayout, elements, node.Comments)
	case *ast.DictionaryLiteral:
		separator := ": "
		if node.LuaStyle {
			separator = " = "
		}
		entries := make([]doc, len(node.Entries))
		for index, entry := range node.Entries {
			entries[index] = concat(p.expression(entry.Key, 0), text(separator), p.expression(entry.Value, 0))
		}
		return p.collection(dictionaryLayout, entries, node.Comments)
	case *ast.BindingPattern:
		return text("var " + node.Name)
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
		if node.Inline {
			statements := make([]doc, len(node.Body))
			for index, statement := range node.Body {
				statements[index] = p.inlineStatement(statement)
			}
			inner = concat(header, text(": "), join(text("; "), statements))
		} else {
			inner = concat(header, text(":"), p.suite(node.Body))
		}
		if parentPrecedence > 0 {
			return parenthesized(inner)
		}
		return inner
	default:
		panic(fmt.Sprintf("format: unsupported expression %T", expr))
	}
}

// inlineStatement renders a statement inside a single-line lambda body.
func (p *printer) inlineStatement(statement ast.Statement) doc {
	switch statement.(type) {
	case *ast.ExpressionStatement, *ast.Assignment, *ast.ReturnStatement,
		*ast.KeywordStatement, *ast.VariableDeclaration:
		return p.statementBody(statement)
	default:
		panic(fmt.Sprintf("format: unsupported inline statement %T", statement))
	}
}

func parenthesized(inner doc) doc { return group(concat(text("("), inner, text(")"))) }

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

func operatorPrecedence(operator string) int {
	switch operator {
	case "or", "||":
		return 1
	case "and", "&&":
		return 2
	case "==", "!=", "<", "<=", ">", ">=", "in", "not in", "is", "is not", "as":
		return 3
	case "|":
		return 4
	case "^":
		return 5
	case "&":
		return 6
	case "<<", ">>":
		return 7
	case "+", "-":
		return 8
	case "*", "/", "%":
		return 9
	case "**":
		return 10
	default:
		return 0
	}
}
