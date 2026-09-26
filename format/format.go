// Package format emits canonical GDScript from an AST.
package format

import (
	"fmt"
	"strings"

	"github.com/cafecito-games/gdparser/ast"
)

// File formats a parsed or programmatically constructed GDScript file.
func File(file *ast.File) string {
	if file == nil {
		return ""
	}
	var printer printer
	printer.statements(file.Statements, 0)
	return printer.builder.String()
}

type printer struct{ builder strings.Builder }

func (p *printer) statements(statements []ast.Statement, depth int) {
	for _, statement := range statements {
		p.statement(statement, depth)
	}
}

func (p *printer) line(depth int, text string) {
	p.builder.WriteString(strings.Repeat("\t", depth))
	p.builder.WriteString(text)
	p.builder.WriteByte('\n')
}

func (p *printer) block(body []ast.Statement, depth int) {
	if len(body) == 0 {
		p.line(depth, "pass")
		return
	}
	p.statements(body, depth)
}

func (p *printer) statement(statement ast.Statement, depth int) {
	switch node := statement.(type) {
	case *ast.Annotation:
		text := "@" + node.Name
		if node.Arguments != nil {
			args := make([]string, len(node.Arguments))
			for i, argument := range node.Arguments {
				args[i] = expressionAt(argument, 0, depth)
			}
			text += "(" + strings.Join(args, ", ") + ")"
		}
		p.line(depth, text)
	case *ast.Comment:
		p.line(depth, node.Text)
	case *ast.Directive:
		text := node.Name
		if node.Value != nil {
			text += " " + expressionAt(node.Value, 0, depth)
		}
		if node.Extends != nil {
			text += " extends " + expressionAt(node.Extends, 0, depth)
		}
		p.line(depth, text)
	case *ast.ExpressionStatement:
		p.line(depth, expressionAt(node.Expression, 0, depth))
	case *ast.VariableDeclaration:
		keyword := "var"
		if node.Constant {
			keyword = "const"
		}
		if node.Static {
			keyword = "static " + keyword
		}
		text := keyword + " " + node.Name
		if node.Type != "" {
			text += ": " + node.Type
		}
		if node.Value != nil {
			operator := " = "
			if node.Inferred {
				operator = " := "
			}
			text += operator + expressionAt(node.Value, 0, depth)
		}
		if node.Getter == nil && node.Setter == nil {
			p.line(depth, text)
			break
		}
		p.line(depth, text+":")
		if node.Getter != nil {
			p.line(depth+1, "get:")
			p.block(node.Getter, depth+2)
		}
		if node.Setter != nil {
			p.line(depth+1, "set("+node.Setter.Parameter+"):")
			p.block(node.Setter.Body, depth+2)
		}
	case *ast.Assignment:
		p.line(depth, expressionAt(node.Target, 0, depth)+" "+node.Operator+" "+expressionAt(node.Value, 0, depth))
	case *ast.ReturnStatement:
		text := "return"
		if node.Value != nil {
			text += " " + expressionAt(node.Value, 0, depth)
		}
		p.line(depth, text)
	case *ast.KeywordStatement:
		p.line(depth, node.Keyword)
	case *ast.FunctionDeclaration:
		prefix := "func "
		if node.Static {
			prefix = "static func "
		}
		params := make([]string, len(node.Parameters))
		for i, parameter := range node.Parameters {
			params[i] = formatParameterAt(parameter, depth)
		}
		text := prefix + node.Name + "(" + strings.Join(params, ", ") + ")"
		if node.ReturnType != "" {
			text += " -> " + node.ReturnType
		}
		if node.Abstract {
			p.line(depth, text)
		} else {
			p.line(depth, text+":")
			p.block(node.Body, depth+1)
		}
	case *ast.ClassDeclaration:
		text := "class " + node.Name
		if node.Extends != "" {
			text += " extends " + node.Extends
		}
		p.line(depth, text+":")
		p.block(node.Body, depth+1)
	case *ast.SignalDeclaration:
		params := make([]string, len(node.Parameters))
		for i, parameter := range node.Parameters {
			params[i] = formatParameterAt(parameter, depth)
		}
		text := "signal " + node.Name
		if node.Parameters != nil {
			text += "(" + strings.Join(params, ", ") + ")"
		}
		p.line(depth, text)
	case *ast.EnumDeclaration:
		text := "enum"
		if node.Name != "" {
			text += " " + node.Name
		}
		if enumHasComments(node) {
			p.line(depth, text+" {")
			for _, member := range node.Members {
				for _, comment := range member.Comments {
					p.line(depth+1, comment.Text)
				}
				memberText := member.Name
				if member.Value != nil {
					memberText += " = " + expressionAt(member.Value, 0, depth+1)
				}
				p.line(depth+1, memberText+",")
			}
			p.line(depth, "}")
		} else {
			members := make([]string, len(node.Members))
			for i, member := range node.Members {
				members[i] = member.Name
				if member.Value != nil {
					members[i] += " = " + expressionAt(member.Value, 0, depth)
				}
			}
			p.line(depth, text+" { "+strings.Join(members, ", ")+" }")
		}
	case *ast.IfStatement:
		for i, branch := range node.Branches {
			keyword := "if"
			if i > 0 {
				keyword = "elif"
			}
			p.line(depth, keyword+" "+expressionAt(branch.Condition, 0, depth)+":")
			p.block(branch.Body, depth+1)
		}
		if node.Else != nil {
			p.line(depth, "else:")
			p.block(node.Else, depth+1)
		}
	case *ast.WhileStatement:
		p.line(depth, "while "+expressionAt(node.Condition, 0, depth)+":")
		p.block(node.Body, depth+1)
	case *ast.ForStatement:
		variable := node.Variable
		if node.Type != "" {
			variable += ": " + node.Type
		}
		p.line(depth, "for "+variable+" in "+expressionAt(node.Iterable, 0, depth)+":")
		p.block(node.Body, depth+1)
	case *ast.MatchStatement:
		p.line(depth, "match "+expressionAt(node.Value, 0, depth)+":")
		for _, matchCase := range node.Cases {
			patterns := make([]string, len(matchCase.Patterns))
			for i, pattern := range matchCase.Patterns {
				patterns[i] = expressionAt(pattern, 0, depth+1)
			}
			text := strings.Join(patterns, ", ")
			if matchCase.Guard != nil {
				text += " when " + expressionAt(matchCase.Guard, 0, depth+1)
			}
			p.line(depth+1, text+":")
			p.block(matchCase.Body, depth+2)
		}
	}
}

func formatParameterAt(parameter ast.Parameter, depth int) string {
	text := parameter.Name
	if parameter.Type != "" {
		text += ": " + parameter.Type
	}
	if parameter.Default != nil {
		text += " = " + expressionAt(parameter.Default, 0, depth)
	}
	return text
}

func enumHasComments(node *ast.EnumDeclaration) bool {
	for _, member := range node.Members {
		if len(member.Comments) > 0 {
			return true
		}
	}
	return false
}

func expression(expr ast.Expression, parentPrecedence int) string {
	return expressionAt(expr, parentPrecedence, 0)
}

func expressionAt(expr ast.Expression, parentPrecedence, depth int) string {
	if expr == nil {
		return ""
	}
	switch node := expr.(type) {
	case *ast.Identifier:
		return node.Name
	case *ast.TypeExpression:
		return node.Name
	case *ast.Literal:
		return node.Raw
	case *ast.NodePathExpression:
		prefix := "$"
		if node.Unique {
			prefix = "%"
		}
		return prefix + node.Path
	case *ast.UnaryExpression:
		operator := node.Operator
		operandPrecedence := 11
		if operator == "not" || operator == "await" {
			operator += " "
		}
		if node.Operator == "not" {
			operandPrecedence = 3
		}
		text := operator + expressionAt(node.Operand, operandPrecedence, depth)
		if node.Operator == "not" && parentPrecedence >= 3 {
			return "(" + text + ")"
		}
		if 11 < parentPrecedence {
			return "(" + text + ")"
		}
		return text
	case *ast.BinaryExpression:
		precedence := operatorPrecedence(node.Operator)
		leftPrecedence, rightPrecedence := precedence, precedence+1
		if node.Operator == "**" {
			leftPrecedence, rightPrecedence = precedence+1, precedence
		}
		text := expressionAt(node.Left, leftPrecedence, depth) + " " + node.Operator + " " + expressionAt(node.Right, rightPrecedence, depth)
		if precedence < parentPrecedence {
			return "(" + text + ")"
		}
		return text
	case *ast.TernaryExpression:
		text := expressionAt(node.Value, 1, depth) + " if " + expressionAt(node.Condition, 1, depth) + " else " + expressionAt(node.Alternative, 1, depth)
		if parentPrecedence > 0 {
			return "(" + text + ")"
		}
		return text
	case *ast.CallExpression:
		arguments := make([]string, len(node.Arguments))
		for i, argument := range node.Arguments {
			arguments[i] = expressionAt(argument, 0, depth)
		}
		return expressionAt(node.Callee, 12, depth) + "(" + strings.Join(arguments, ", ") + ")"
	case *ast.MemberExpression:
		return expressionAt(node.Object, 12, depth) + "." + node.Property
	case *ast.SubscriptExpression:
		return expressionAt(node.Object, 12, depth) + "[" + expressionAt(node.Index, 0, depth) + "]"
	case *ast.ArrayLiteral:
		elements := make([]string, len(node.Elements))
		for i, element := range node.Elements {
			elements[i] = expressionAt(element, 0, depth)
		}
		return "[" + strings.Join(elements, ", ") + "]"
	case *ast.DictionaryLiteral:
		entries := make([]string, len(node.Entries))
		for i, entry := range node.Entries {
			entries[i] = expressionAt(entry.Key, 0, depth) + ": " + expressionAt(entry.Value, 0, depth)
		}
		return "{" + strings.Join(entries, ", ") + "}"
	case *ast.LambdaExpression:
		parameters := make([]string, len(node.Parameters))
		for i, parameter := range node.Parameters {
			parameters[i] = formatParameterAt(parameter, depth)
		}
		text := "func(" + strings.Join(parameters, ", ") + ")"
		if node.ReturnType != "" {
			text += " -> " + node.ReturnType
		}
		if !node.Inline {
			var bodyPrinter printer
			bodyPrinter.block(node.Body, depth+1)
			result := text + ":\n" + bodyPrinter.builder.String() + strings.Repeat("\t", depth)
			if parentPrecedence > 0 {
				return "(" + result + ")"
			}
			return result
		}
		body := make([]string, len(node.Body))
		for i, statement := range node.Body {
			body[i] = inlineStatement(statement)
		}
		result := text + ": " + strings.Join(body, "; ")
		if parentPrecedence > 0 {
			return "(" + result + ")"
		}
		return result
	default:
		panic(fmt.Sprintf("format: unsupported expression %T", expr))
	}
}

func inlineStatement(statement ast.Statement) string {
	switch node := statement.(type) {
	case *ast.ExpressionStatement:
		return expression(node.Expression, 0)
	case *ast.Assignment:
		return expression(node.Target, 0) + " " + node.Operator + " " + expression(node.Value, 0)
	case *ast.ReturnStatement:
		if node.Value == nil {
			return "return"
		}
		return "return " + expression(node.Value, 0)
	case *ast.KeywordStatement:
		return node.Keyword
	case *ast.VariableDeclaration:
		keyword := "var"
		if node.Constant {
			keyword = "const"
		}
		text := keyword + " " + node.Name
		if node.Type != "" {
			text += ": " + node.Type
		}
		if node.Value != nil {
			text += " = " + expression(node.Value, 0)
		}
		return text
	default:
		panic(fmt.Sprintf("format: unsupported inline statement %T", statement))
	}
}

func operatorPrecedence(operator string) int {
	switch operator {
	case "or":
		return 1
	case "and":
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
