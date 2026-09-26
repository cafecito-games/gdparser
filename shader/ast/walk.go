package ast

// Visitor visits nodes depth-first. Returning nil prevents descent.
type Visitor interface{ Visit(Node) Visitor }

// Walk traverses node depth-first.
func Walk(visitor Visitor, node Node) {
	if node == nil {
		return
	}
	next := visitor.Visit(node)
	if next == nil {
		return
	}
	for _, child := range Children(node) {
		Walk(next, child)
	}
	next.Visit(nil)
}

// Inspect traverses node depth-first. Returning false prevents descent.
func Inspect(node Node, fn func(Node) bool) { Walk(inspector(fn), node) }

type inspector func(Node) bool

func (fn inspector) Visit(node Node) Visitor {
	if fn(node) {
		return fn
	}
	return nil
}

// Children returns direct child nodes in source order.
func Children(node Node) []Node {
	var out []Node
	add := func(n Node) {
		if n != nil {
			out = append(out, n)
		}
	}
	addExpr := func(e Expression) {
		if e != nil {
			out = append(out, e)
		}
	}
	addStmt := func(s Statement) {
		if s != nil {
			out = append(out, s)
		}
	}
	switch n := node.(type) {
	case *File:
		for _, v := range n.Items {
			add(v)
		}
	case *RenderMode:
		for _, v := range n.Modes {
			addExpr(v)
		}
	case *StencilMode:
		for _, v := range n.Modes {
			addExpr(v)
		}
	case *VariableDeclaration:
		for _, v := range n.Declarators {
			add(v)
		}
		for _, v := range n.Hints {
			add(v)
		}
	case *Declarator:
		for _, v := range n.Arrays {
			add(v)
		}
		addExpr(n.Value)
	case *ArraySpecifier:
		addExpr(n.Size)
	case *Hint:
		for _, v := range n.Arguments {
			addExpr(v)
		}
	case *StructDeclaration:
		for _, v := range n.Members {
			add(v)
		}
	case *Parameter:
		for _, v := range n.Arrays {
			add(v)
		}
	case *FunctionDeclaration:
		for _, v := range n.ReturnArrays {
			add(v)
		}
		for _, v := range n.Parameters {
			add(v)
		}
		add(n.Body)
	case *Block:
		for _, v := range n.Statements {
			addStmt(v)
		}
	case *ExpressionStatement:
		addExpr(n.Expression)
	case *IfStatement:
		addExpr(n.Condition)
		addStmt(n.Then)
		addStmt(n.Else)
	case *ForStatement:
		addStmt(n.Initializer)
		addExpr(n.Condition)
		addExpr(n.Update)
		addStmt(n.Body)
	case *WhileStatement:
		addExpr(n.Condition)
		addStmt(n.Body)
	case *DoWhileStatement:
		addStmt(n.Body)
		addExpr(n.Condition)
	case *ReturnStatement:
		addExpr(n.Value)
	case *SwitchStatement:
		addExpr(n.Value)
		for _, v := range n.Cases {
			add(v)
		}
	case *SwitchCase:
		addExpr(n.Value)
		for _, v := range n.Statements {
			addStmt(v)
		}
	case *UnaryExpression:
		addExpr(n.Operand)
	case *PostfixExpression:
		addExpr(n.Operand)
	case *BinaryExpression:
		addExpr(n.Left)
		addExpr(n.Right)
	case *ConditionalExpression:
		addExpr(n.Condition)
		addExpr(n.Then)
		addExpr(n.Else)
	case *AssignmentExpression:
		addExpr(n.Target)
		addExpr(n.Value)
	case *CallExpression:
		addExpr(n.Callee)
		for _, v := range n.Arguments {
			addExpr(v)
		}
	case *MemberExpression:
		addExpr(n.Object)
	case *IndexExpression:
		addExpr(n.Object)
		addExpr(n.Index)
	case *InitializerList:
		for _, v := range n.Items {
			add(v)
		}
	}
	return out
}
