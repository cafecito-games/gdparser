// Package format emits canonical Godot shading-language source from an AST.
package format

import (
	"strings"

	"github.com/cafecito-games/gdparser/shader/ast"
)

// File formats a complete shader file.
func File(file *ast.File) string {
	if file == nil {
		return ""
	}
	f := formatter{}
	for i, item := range file.Items {
		if i > 0 && isFunctionLike(item) {
			f.newline()
		}
		f.item(item)
		f.newline()
	}
	return f.b.String()
}

type formatter struct {
	b         strings.Builder
	indent    int
	lineStart bool
}

func (f *formatter) write(s string) {
	if f.lineStart && s != "" {
		f.b.WriteString(strings.Repeat("\t", f.indent))
		f.lineStart = false
	}
	f.b.WriteString(s)
}
func (f *formatter) newline() { f.b.WriteByte('\n'); f.lineStart = true }

func (f *formatter) item(item ast.Item) {
	switch n := item.(type) {
	case *ast.Comment:
		f.comment(n)
	case *ast.PreprocessorDirective:
		f.directive(n)
	case *ast.ShaderType:
		f.write("shader_type ")
		f.write(n.Name)
		f.write(";")
	case *ast.RenderMode:
		f.write("render_mode ")
		for i, m := range n.Modes {
			if i > 0 {
				f.write(", ")
			}
			f.expression(m, 0, false)
		}
		f.write(";")
	case *ast.StencilMode:
		f.write("stencil_mode ")
		for i, m := range n.Modes {
			if i > 0 {
				f.write(", ")
			}
			f.expression(m, 0, false)
		}
		f.write(";")
	case *ast.GroupUniforms:
		f.write("group_uniforms")
		if n.Name != "" {
			f.write(" ")
			f.write(n.Name)
			if n.Subgroup != "" {
				f.write(".")
				f.write(n.Subgroup)
			}
		}
		f.write(";")
	case *ast.VariableDeclaration:
		f.variable(n, true)
	case *ast.StructDeclaration:
		f.write("struct ")
		f.write(n.Name)
		f.write(" {")
		f.newline()
		f.indent++
		for _, member := range n.Members {
			switch field := member.(type) {
			case *ast.VariableDeclaration:
				f.variable(field, true)
			case *ast.Comment:
				f.comment(field)
			}
			f.newline()
		}
		f.indent--
		f.write("};")
	case *ast.FunctionDeclaration:
		f.function(n)
	}
}

func isFunctionLike(item ast.Item) bool {
	switch item.(type) {
	case *ast.FunctionDeclaration, *ast.StructDeclaration:
		return true
	}
	return false
}

func (f *formatter) function(n *ast.FunctionDeclaration) {
	f.qualifiers(n.ReturnQualifiers)
	f.write(n.ReturnType)
	f.arrays(n.ReturnArrays)
	f.write(" ")
	f.write(n.Name)
	f.write("(")
	for i, p := range n.Parameters {
		if i > 0 {
			f.write(", ")
		}
		f.qualifiers(p.Qualifiers)
		f.write(p.Type)
		f.write(" ")
		f.write(p.Name)
		f.arrays(p.Arrays)
	}
	f.write(") ")
	f.block(n.Body)
}

func (f *formatter) qualifiers(q ast.Qualifiers) {
	var values []string
	if q.Const {
		values = append(values, "const")
	}
	if q.Invariant {
		values = append(values, "invariant")
	}
	if q.Interpolation != "" {
		values = append(values, q.Interpolation)
	}
	values = append(values, q.Storage...)
	if q.Precision != "" {
		values = append(values, q.Precision)
	}
	if len(values) > 0 {
		f.write(strings.Join(values, " "))
		f.write(" ")
	}
}

func (f *formatter) variable(n *ast.VariableDeclaration, semicolon bool) {
	f.qualifiers(n.Qualifiers)
	f.write(n.Type)
	f.write(" ")
	for i, d := range n.Declarators {
		if i > 0 {
			f.write(", ")
		}
		f.write(d.Name)
		f.arrays(d.Arrays)
		if d.Value != nil && len(n.Hints) == 0 {
			f.write(" = ")
			f.expression(d.Value, 0, false)
		}
	}
	if len(n.Hints) > 0 {
		f.write(" : ")
		for i, h := range n.Hints {
			if i > 0 {
				f.write(", ")
			}
			f.write(h.Name)
			if len(h.Arguments) > 0 {
				f.write("(")
				for j, a := range h.Arguments {
					if j > 0 {
						f.write(", ")
					}
					f.expression(a, 0, false)
				}
				f.write(")")
			}
		}
		if len(n.Declarators) == 1 && n.Declarators[0].Value != nil {
			f.write(" = ")
			f.expression(n.Declarators[0].Value, 0, false)
		}
	}
	if semicolon {
		f.write(";")
	}
}

func (f *formatter) arrays(arrays []*ast.ArraySpecifier) {
	for _, a := range arrays {
		f.write("[")
		if a.Size != nil {
			f.expression(a.Size, 0, false)
		}
		f.write("]")
	}
}

func (f *formatter) block(n *ast.Block) {
	f.write("{")
	if len(n.Statements) == 0 {
		f.write("}")
		return
	}
	f.newline()
	f.indent++
	for _, s := range n.Statements {
		f.statement(s)
		f.newline()
	}
	f.indent--
	f.write("}")
}

func (f *formatter) statement(stmt ast.Statement) {
	switch n := stmt.(type) {
	case *ast.Comment:
		f.comment(n)
	case *ast.PreprocessorDirective:
		f.directive(n)
	case *ast.Block:
		f.block(n)
	case *ast.VariableDeclaration:
		f.variable(n, true)
	case *ast.ExpressionStatement:
		if n.Expression != nil {
			f.expression(n.Expression, 0, false)
		}
		f.write(";")
	case *ast.ReturnStatement:
		f.write("return")
		if n.Value != nil {
			f.write(" ")
			f.expression(n.Value, 0, false)
		}
		f.write(";")
	case *ast.KeywordStatement:
		f.write(n.Keyword)
		f.write(";")
	case *ast.IfStatement:
		f.write("if (")
		f.expression(n.Condition, 0, false)
		f.write(") ")
		f.controlBody(n.Then)
		if n.Else != nil {
			f.write(" else ")
			if nested, ok := n.Else.(*ast.IfStatement); ok {
				f.statement(nested)
			} else {
				f.controlBody(n.Else)
			}
		}
	case *ast.ForStatement:
		f.write("for (")
		if n.Initializer != nil {
			switch init := n.Initializer.(type) {
			case *ast.VariableDeclaration:
				f.variable(init, false)
			case *ast.ExpressionStatement:
				if init.Expression != nil {
					f.expression(init.Expression, 0, false)
				}
			}
		}
		f.write("; ")
		if n.Condition != nil {
			f.expression(n.Condition, 0, false)
		}
		f.write("; ")
		if n.Update != nil {
			f.expression(n.Update, 0, false)
		}
		f.write(") ")
		f.controlBody(n.Body)
	case *ast.WhileStatement:
		f.write("while (")
		f.expression(n.Condition, 0, false)
		f.write(") ")
		f.controlBody(n.Body)
	case *ast.DoWhileStatement:
		f.write("do ")
		f.controlBody(n.Body)
		f.write(" while (")
		f.expression(n.Condition, 0, false)
		f.write(");")
	case *ast.SwitchStatement:
		f.write("switch (")
		f.expression(n.Value, 0, false)
		f.write(") {")
		f.newline()
		f.indent++
		for _, c := range n.Cases {
			if c.Default {
				f.write("default:")
			} else {
				f.write("case ")
				f.expression(c.Value, 0, false)
				f.write(":")
			}
			f.newline()
			f.indent++
			for _, s := range c.Statements {
				f.statement(s)
				f.newline()
			}
			f.indent--
		}
		f.indent--
		f.write("}")
	}
}

func (f *formatter) controlBody(stmt ast.Statement) {
	if b, ok := stmt.(*ast.Block); ok {
		f.block(b)
		return
	}
	f.newline()
	f.indent++
	f.statement(stmt)
	f.indent--
}

func (f *formatter) comment(n *ast.Comment) {
	lines := strings.Split(n.Text, "\n")
	for i, line := range lines {
		if i > 0 {
			f.newline()
		}
		f.write(line)
	}
}
func (f *formatter) directive(n *ast.PreprocessorDirective) {
	text := "#" + n.Name
	if n.Body != "" {
		text += " " + n.Body
	}
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			f.newline()
		}
		f.write(line)
	}
}

func (f *formatter) expression(expr ast.Expression, parent int, right bool) {
	if expr == nil {
		return
	}
	prec := expressionPrecedence(expr)
	parens := prec < parent || right && prec == parent && !rightAssociative(expr)
	if parens {
		f.write("(")
	}
	switch n := expr.(type) {
	case *ast.Literal:
		f.write(n.Raw)
	case *ast.Identifier:
		f.write(n.Name)
	case *ast.UnaryExpression:
		f.write(n.Operator)
		f.expression(n.Operand, prec, true)
	case *ast.PostfixExpression:
		f.expression(n.Operand, prec, false)
		f.write(n.Operator)
	case *ast.BinaryExpression:
		f.expression(n.Left, prec, false)
		f.write(" ")
		f.write(n.Operator)
		f.write(" ")
		f.expression(n.Right, prec, true)
	case *ast.AssignmentExpression:
		f.expression(n.Target, prec+1, false)
		f.write(" ")
		f.write(n.Operator)
		f.write(" ")
		f.expression(n.Value, prec, true)
	case *ast.ConditionalExpression:
		f.expression(n.Condition, prec+1, false)
		f.write(" ? ")
		f.expression(n.Then, 0, false)
		f.write(" : ")
		f.expression(n.Else, prec, true)
	case *ast.CallExpression:
		f.expression(n.Callee, prec, false)
		f.write("(")
		for i, a := range n.Arguments {
			if i > 0 {
				f.write(", ")
			}
			f.expression(a, 0, false)
		}
		f.write(")")
	case *ast.MemberExpression:
		f.expression(n.Object, prec, false)
		f.write(".")
		f.write(n.Property)
	case *ast.IndexExpression:
		f.expression(n.Object, prec, false)
		f.write("[")
		if n.Index != nil {
			f.expression(n.Index, 0, false)
		}
		f.write("]")
	case *ast.InitializerList:
		f.initializer(n)
	}
	if parens {
		f.write(")")
	}
}

func (f *formatter) initializer(n *ast.InitializerList) {
	hasComments := false
	lastExpression := -1
	for i, item := range n.Items {
		if _, ok := item.(*ast.Comment); ok {
			hasComments = true
		} else {
			lastExpression = i
		}
	}
	f.write("{")
	if !hasComments {
		for i, item := range n.Items {
			if i > 0 {
				f.write(", ")
			}
			f.expression(item.(ast.Expression), 0, false)
		}
		f.write("}")
		return
	}
	f.newline()
	f.indent++
	for i, item := range n.Items {
		switch value := item.(type) {
		case ast.Expression:
			f.expression(value, 0, false)
			if i != lastExpression {
				f.write(",")
			}
		case *ast.Comment:
			f.comment(value)
		}
		f.newline()
	}
	f.indent--
	f.write("}")
}

func expressionPrecedence(expr ast.Expression) int {
	switch n := expr.(type) {
	case *ast.AssignmentExpression:
		return 1
	case *ast.ConditionalExpression:
		return 2
	case *ast.BinaryExpression:
		p, _, _ := operatorPrecedence(n.Operator)
		return p
	case *ast.UnaryExpression:
		return 14
	case *ast.PostfixExpression, *ast.CallExpression, *ast.MemberExpression, *ast.IndexExpression:
		return 15
	case *ast.InitializerList:
		return 16
	default:
		return 16
	}
}
func rightAssociative(expr ast.Expression) bool {
	switch expr.(type) {
	case *ast.AssignmentExpression, *ast.ConditionalExpression, *ast.UnaryExpression:
		return true
	}
	return false
}
func operatorPrecedence(op string) (int, bool, bool) {
	switch op {
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
	}
	return 0, false, false
}
