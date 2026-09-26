package textresource

import "reflect"

// Visitor visits nodes depth-first. Returning nil prevents descent.
type Visitor interface{ Visit(Node) Visitor }

// Walk traverses node depth-first and in source order.
func Walk(visitor Visitor, node Node) {
	if node == nil || reflect.ValueOf(node).IsNil() {
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

// Children returns direct children in source order.
func Children(node Node) []Node {
	var out []Node
	add := func(n Node) {
		if n != nil && !reflect.ValueOf(n).IsNil() {
			out = append(out, n)
		}
	}
	switch n := node.(type) {
	case *Document:
		for _, v := range n.Items {
			add(v)
		}
	case *Section:
		for _, v := range n.Attributes {
			add(v)
		}
		add(n.HeaderComment)
		for _, v := range n.Body {
			add(v)
		}
	case *Attribute:
		add(n.Value)
	case *Assignment:
		add(n.Value)
		add(n.TrailingComment)
	case *ArrayValue:
		for _, v := range n.Items {
			add(v)
		}
	case *DictionaryValue:
		for _, v := range n.Items {
			add(v)
		}
	case *DictionaryEntry:
		add(n.Key)
		add(n.Value)
	case *CallValue:
		for _, v := range n.Arguments {
			add(v)
		}
	case *TypeRef:
		add(n.Constructor)
		for _, v := range n.Arguments {
			add(v)
		}
	case *TypedArrayValue:
		add(n.ElementType)
		add(n.Array)
	case *TypedDictionaryValue:
		add(n.KeyType)
		add(n.ValueType)
		add(n.Dictionary)
	}
	return out
}
