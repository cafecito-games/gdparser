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
	addStatements := func(statements []Statement) {
		for _, statement := range statements {
			if statement != nil {
				out = append(out, statement)
			}
		}
	}
	addExpression := func(expression Expression) {
		if expression != nil {
			out = append(out, expression)
		}
	}
	switch n := node.(type) {
	case *File:
		addStatements(n.Preamble)
		for _, section := range n.Sections {
			if section != nil {
				out = append(out, section)
			}
		}
	case *Section:
		addStatements(n.Statements)
	case *Assignment:
		addExpression(n.Value)
	case *UnaryExpression:
		addExpression(n.Operand)
	case *ArrayLiteral:
		for _, item := range n.Items {
			if item != nil {
				out = append(out, item)
			}
		}
	case *ArrayElement:
		addExpression(n.Value)
	case *DictionaryLiteral:
		for _, item := range n.Items {
			if item != nil {
				out = append(out, item)
			}
		}
	case *DictionaryEntry:
		addExpression(n.Key)
		for _, comment := range n.InfixComments {
			if comment != nil {
				out = append(out, comment)
			}
		}
		addExpression(n.Value)
	case *ConstructorCall:
		for _, item := range n.Items {
			if item != nil {
				out = append(out, item)
			}
		}
	case *ConstructorArgument:
		addExpression(n.Key)
		for _, comment := range n.InfixComments {
			if comment != nil {
				out = append(out, comment)
			}
		}
		addExpression(n.Value)
	case *TypeRef:
		for _, argument := range n.Arguments {
			if argument != nil {
				out = append(out, argument)
			}
		}
	case *TypedArrayLiteral:
		if n.ElementType != nil {
			out = append(out, n.ElementType)
		}
		addExpression(n.Value)
	case *TypedDictionaryLiteral:
		if n.KeyType != nil {
			out = append(out, n.KeyType)
		}
		if n.ValueType != nil {
			out = append(out, n.ValueType)
		}
		addExpression(n.Value)
	}
	return out
}
