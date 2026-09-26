package ast

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Dump writes a compact source-ordered tree representation.
func Dump(writer io.Writer, node Node) error {
	if node == nil {
		_, err := fmt.Fprintln(writer, "<nil>")
		return err
	}
	depth := 0
	var writeErr error
	Inspect(node, func(current Node) bool {
		if current == nil {
			depth--
			return true
		}
		if writeErr == nil {
			_, writeErr = fmt.Fprintf(writer, "%s%s%s\n", strings.Repeat("  ", depth), nodeName(current), nodeLabel(current))
		}
		depth++
		return true
	})
	return writeErr
}

func nodeName(node Node) string {
	typ := reflect.TypeOf(node)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Name()
}

func nodeLabel(node Node) string {
	switch n := node.(type) {
	case *File:
		if n.Name != "" {
			return fmt.Sprintf(" %q", n.Name)
		}
	case *Section:
		return " " + n.Name
	case *Assignment:
		return " " + n.Key
	case *Comment:
		return " " + n.Text
	case *BoolLiteral:
		return fmt.Sprintf(" %t", n.Value)
	case *IntegerLiteral:
		return " " + n.Raw
	case *FloatLiteral:
		return " " + n.Raw
	case *StringLiteral:
		return fmt.Sprintf(" %s%q", n.Prefix, n.Value)
	case *Identifier:
		return " " + n.Name
	case *UnaryExpression:
		return " " + n.Operator
	case *ConstructorCall:
		return " " + n.Name
	case *TypeRef:
		return " " + n.Name
	}
	return ""
}
