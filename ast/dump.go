package ast

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// Dump writes a compact, source-ordered tree representation of node.
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
	case *Identifier:
		return " " + n.Name
	case *LambdaExpression:
		if n.Name != "" {
			return " " + n.Name
		}
	case *BindingPattern:
		return " var " + n.Name
	case *Literal:
		return " " + n.Raw
	case *Comment:
		if n.TrailsHeader {
			return " " + n.Text + " (trails header)"
		}
		return " " + n.Text
	case *Annotation:
		return " @" + n.Name
	case *Directive:
		return " " + n.Name
	case *UnaryExpression:
		return " " + n.Operator
	case *BinaryExpression:
		return " " + n.Operator
	case *MemberExpression:
		return " ." + n.Property
	case *NodePathExpression:
		prefix := "$"
		if n.Unique {
			prefix = "%"
		}
		return " " + prefix + n.Path
	case *VariableDeclaration:
		keyword := "var"
		if n.Constant {
			keyword = "const"
		}
		return " " + keyword + " " + n.Name
	case *Assignment:
		return " " + n.Operator
	case *KeywordStatement:
		return " " + n.Keyword
	case *FunctionDeclaration:
		return " " + n.Name
	case *ClassDeclaration:
		return " " + n.Name
	case *SignalDeclaration:
		return " " + n.Name
	case *EnumDeclaration:
		return " " + n.Name
	case *ForStatement:
		return " " + n.Variable
	}
	return ""
}
