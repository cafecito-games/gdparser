package uidfile

import (
	"fmt"
	"io"
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
			_, writeErr = fmt.Fprintf(writer, "%s%s\n", strings.Repeat("  ", depth), dumpLabel(current))
		}
		depth++
		return true
	})
	return writeErr
}

func dumpLabel(node Node) string {
	switch node := node.(type) {
	case *File:
		if node.Name != "" {
			return fmt.Sprintf("File %q", node.Name)
		}
		return "File"
	case *UID:
		return "UID " + node.Value
	default:
		return "Node"
	}
}
