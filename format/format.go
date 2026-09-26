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
				args[i] = expression(argument, 0)
			}
			text += "(" + strings.Join(args, ", ") + ")"
		}
		p.line(depth, text)
	case *ast.Comment:
		p.line(depth, node.Text)
	case *ast.Directive:
		text := node.Name
		if node.Value != nil {
			text += " " + expression(node.Value, 0)
		}
		p.line(depth, text)
	case *ast.ExpressionStatement:
		p.line(depth, expression(node.Expression, 0))
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
			operator := " = "
			if node.Inferred {
				operator = " := "
			}
			text += operator + expression(node.Value, 0)
		}
		p.line(depth, text)
	case *ast.Assignment:
		p.line(depth, expression(node.Target, 0)+" "+node.Operator+" "+expression(node.Value, 0))
	case *ast.ReturnStatement:
		text := "return"
		if node.Value != nil {
			text += " " + expression(node.Value, 0)
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
			params[i] = formatParameter(parameter)
		}
		text := prefix + node.Name + "(" + strings.Join(params, ", ") + ")"
		if node.ReturnType != "" {
			text += " -> " + node.ReturnType
		}
		p.line(depth, text+":")
		p.block(node.Body, depth+1)
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
			params[i] = formatParameter(parameter)
		}
		text := "signal " + node.Name
		if node.Parameters != nil {
			text += "(" + strings.Join(params, ", ") + ")"
		}
		p.line(depth, text)
	case *ast.EnumDeclaration:
		members := make([]string, len(node.Members))
		for i, member := range node.Members {
			members[i] = member.Name
			if member.Value != nil {
				members[i] += " = " + expression(member.Value, 0)
			}
		}
		text := "enum"
		if node.Name != "" {
			text += " " + node.Name
		}
		p.line(depth, text+" { "+strings.Join(members, ", ")+" }")
	case *ast.IfStatement:
		for i, branch := range node.Branches {
			keyword := "if"
			if i > 0 {
				keyword = "elif"
			}
			p.line(depth, keyword+" "+expression(branch.Condition, 0)+":")
			p.block(branch.Body, depth+1)
		}
		if node.Else != nil {
			p.line(depth, "else:")
			p.block(node.Else, depth+1)
		}
	case *ast.WhileStatement:
		p.line(depth, "while "+expression(node.Condition, 0)+":")
		p.block(node.Body, depth+1)
	case *ast.ForStatement:
		p.line(depth, "for "+node.Variable+" in "+expression(node.Iterable, 0)+":")
		p.block(node.Body, depth+1)
	case *ast.MatchStatement:
		p.line(depth, "match "+expression(node.Value, 0)+":")
		for _, matchCase := range node.Cases {
			patterns := make([]string, len(matchCase.Patterns))
			for i, pattern := range matchCase.Patterns {
				patterns[i] = expression(pattern, 0)
			}
			text := strings.Join(patterns, ", ")
			if matchCase.Guard != nil {
				text += " when " + expression(matchCase.Guard, 0)
			}
			p.line(depth+1, text+":")
			p.block(matchCase.Body, depth+2)
		}
	}
}

func formatParameter(parameter ast.Parameter) string {
	text := parameter.Name
	if parameter.Type != "" {
		text += ": " + parameter.Type
	}
	if parameter.Default != nil {
		text += " = " + expression(parameter.Default, 0)
	}
	return text
}

func expression(expr ast.Expression, parentPrecedence int) string {
	if expr == nil {
		return ""
	}
	switch node := expr.(type) {
	case *ast.Identifier:
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
		text := operator + expression(node.Operand, operandPrecedence)
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
		text := expression(node.Left, leftPrecedence) + " " + node.Operator + " " + expression(node.Right, rightPrecedence)
		if precedence < parentPrecedence {
			return "(" + text + ")"
		}
		return text
	case *ast.TernaryExpression:
		text := expression(node.Value, 1) + " if " + expression(node.Condition, 1) + " else " + expression(node.Alternative, 1)
		if parentPrecedence > 0 {
			return "(" + text + ")"
		}
		return text
	case *ast.CallExpression:
		arguments := make([]string, len(node.Arguments))
		for i, argument := range node.Arguments {
			arguments[i] = expression(argument, 0)
		}
		return expression(node.Callee, 12) + "(" + strings.Join(arguments, ", ") + ")"
	case *ast.MemberExpression:
		return expression(node.Object, 12) + "." + node.Property
	case *ast.SubscriptExpression:
		return expression(node.Object, 12) + "[" + expression(node.Index, 0) + "]"
	case *ast.ArrayLiteral:
		elements := make([]string, len(node.Elements))
		for i, element := range node.Elements {
			elements[i] = expression(element, 0)
		}
		return "[" + strings.Join(elements, ", ") + "]"
	case *ast.DictionaryLiteral:
		entries := make([]string, len(node.Entries))
		for i, entry := range node.Entries {
			entries[i] = expression(entry.Key, 0) + ": " + expression(entry.Value, 0)
		}
		return "{" + strings.Join(entries, ", ") + "}"
	default:
		panic(fmt.Sprintf("format: unsupported expression %T", expr))
	}
}

func operatorPrecedence(operator string) int {
	switch operator {
	case "or":
		return 1
	case "and":
		return 2
	case "==", "!=", "<", "<=", ">", ">=", "in", "not in", "is", "as":
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
