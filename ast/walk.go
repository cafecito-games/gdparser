package ast

// Visitor visits nodes depth-first. Returning nil prevents descent into a node.
type Visitor interface {
	Visit(Node) Visitor
}

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
func Inspect(node Node, fn func(Node) bool) {
	Walk(inspector(fn), node)
}

type inspector func(Node) bool

func (fn inspector) Visit(node Node) Visitor {
	if fn(node) {
		return fn
	}
	return nil
}

// Children returns the direct child nodes of node in source order.
func Children(node Node) []Node {
	var out []Node
	addExpr := func(expr Expression) {
		if expr != nil {
			out = append(out, expr)
		}
	}
	addStmts := func(stmts []Statement) {
		for _, stmt := range stmts {
			out = append(out, stmt)
		}
	}

	switch n := node.(type) {
	case *File:
		addStmts(n.Statements)
	case *Annotation:
		for _, arg := range n.Arguments {
			addExpr(arg)
		}
	case *Directive:
		addExpr(n.Value)
	case *UnaryExpression:
		addExpr(n.Operand)
	case *BinaryExpression:
		addExpr(n.Left)
		addExpr(n.Right)
	case *TernaryExpression:
		addExpr(n.Value)
		addExpr(n.Condition)
		addExpr(n.Alternative)
	case *CallExpression:
		addExpr(n.Callee)
		for _, arg := range n.Arguments {
			addExpr(arg)
		}
	case *MemberExpression:
		addExpr(n.Object)
	case *SubscriptExpression:
		addExpr(n.Object)
		addExpr(n.Index)
	case *ArrayLiteral:
		for _, elem := range n.Elements {
			addExpr(elem)
		}
	case *DictionaryLiteral:
		for _, entry := range n.Entries {
			addExpr(entry.Key)
			addExpr(entry.Value)
		}
	case *ExpressionStatement:
		addExpr(n.Expression)
	case *VariableDeclaration:
		addExpr(n.Value)
	case *Assignment:
		addExpr(n.Target)
		addExpr(n.Value)
	case *ReturnStatement:
		addExpr(n.Value)
	case *IfStatement:
		for _, branch := range n.Branches {
			addExpr(branch.Condition)
			addStmts(branch.Body)
		}
		addStmts(n.Else)
	case *WhileStatement:
		addExpr(n.Condition)
		addStmts(n.Body)
	case *ForStatement:
		addExpr(n.Iterable)
		addStmts(n.Body)
	case *FunctionDeclaration:
		for _, parameter := range n.Parameters {
			addExpr(parameter.Default)
		}
		addStmts(n.Body)
	case *ClassDeclaration:
		addStmts(n.Body)
	case *SignalDeclaration:
		for _, parameter := range n.Parameters {
			addExpr(parameter.Default)
		}
	case *EnumDeclaration:
		for _, member := range n.Members {
			addExpr(member.Value)
		}
	case *MatchStatement:
		addExpr(n.Value)
		for _, matchCase := range n.Cases {
			for _, pattern := range matchCase.Patterns {
				addExpr(pattern)
			}
			addExpr(matchCase.Guard)
			addStmts(matchCase.Body)
		}
	}
	return out
}
