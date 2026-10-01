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

// Annotations returns the annotations attached to node, in source order, or nil
// when node is not an annotated declaration.
func Annotations(node Node) []*Annotation {
	switch n := node.(type) {
	case *VariableDeclaration:
		return n.Annotations
	case *FunctionDeclaration:
		return n.Annotations
	case *ClassDeclaration:
		return n.Annotations
	case *SignalDeclaration:
		return n.Annotations
	case *EnumDeclaration:
		return n.Annotations
	}
	return nil
}

// Children returns the direct child nodes of node in source order. Attached
// annotations come first and an attached trailing comment comes last.
func Children(node Node) []Node {
	var out []Node
	for _, annotation := range Annotations(node) {
		out = append(out, annotation)
	}
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
		addExpr(n.Extends)
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
	case *LambdaExpression:
		for _, parameter := range n.Parameters {
			addExpr(parameter.Default)
		}
		addStmts(n.Body)
	case *ExpressionStatement:
		addExpr(n.Expression)
	case *VariableDeclaration:
		addExpr(n.Value)
		addStmts(n.Getter)
		if n.Setter != nil {
			addStmts(n.Setter.Body)
		}
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
			for _, comment := range member.Comments {
				out = append(out, comment)
			}
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
	if bearer, ok := node.(TriviaBearer); ok {
		if comment := bearer.StatementTrivia().TrailingComment; comment != nil {
			out = append(out, comment)
		}
	}
	return out
}
