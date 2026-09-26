package ast

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Dump writes a compact source-ordered tree representation.
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
func nodeName(n Node) string {
	t := reflect.TypeOf(n)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}
func nodeLabel(node Node) string {
	switch n := node.(type) {
	case *File:
		if n.Name != "" {
			return fmt.Sprintf(" %q", n.Name)
		}
	case *Identifier:
		return " " + n.Name
	case *Literal:
		return " " + n.Raw
	case *Comment:
		return " " + n.Text
	case *PreprocessorDirective:
		return " #" + n.Name
	case *ShaderType:
		return " " + n.Name
	case *GroupUniforms:
		return " " + n.Name
	case *VariableDeclaration:
		return " " + n.Type
	case *Declarator:
		return " " + n.Name
	case *StructDeclaration:
		return " " + n.Name
	case *FunctionDeclaration:
		return " " + n.Name
	case *Parameter:
		return " " + n.Name
	case *UnaryExpression:
		return " " + n.Operator
	case *PostfixExpression:
		return " " + n.Operator
	case *BinaryExpression:
		return " " + n.Operator
	case *AssignmentExpression:
		return " " + n.Operator
	case *MemberExpression:
		return " ." + n.Property
	case *KeywordStatement:
		return " " + n.Keyword
	}
	return ""
}
