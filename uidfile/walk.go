package uidfile

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
	if file, ok := node.(*File); ok && file.UID != nil {
		return []Node{file.UID}
	}
	return nil
}
