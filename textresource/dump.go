package textresource

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Dump writes a compact, source-ordered tree representation.
func Dump(w io.Writer, node Node) error {
	if node == nil {
		_, err := fmt.Fprintln(w, "<nil>")
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
			_, writeErr = fmt.Fprintf(w, "%s%s%s\n", strings.Repeat("  ", depth), nodeName(current), nodeLabel(current))
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
	case *Document:
		if n.Name != "" {
			return " " + strconvQuote(n.Name)
		}
	case *Section:
		return " " + n.Type
	case *Attribute:
		return " " + n.Name
	case *Assignment:
		return " " + n.Property
	case *Comment:
		return " " + n.Text
	case *IdentifierValue:
		return " " + n.Name
	case *StringValue:
		return " " + strconvQuote(n.Value)
	case *CallValue:
		return " " + n.Name
	case *TypeRef:
		return " " + n.Name
	}
	return ""
}
func strconvQuote(value string) string { return fmt.Sprintf("%q", value) }
