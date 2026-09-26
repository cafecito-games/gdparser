// Package format emits canonical Godot project configuration source from an
// AST.
package format

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cafecito-games/gdparser/projectconfig/ast"
)

// File formats a parsed or programmatically constructed project configuration.
func File(file *ast.File) string {
	if file == nil {
		return ""
	}
	var builder strings.Builder
	writeStatements(&builder, file.Preamble)
	if len(file.Preamble) > 0 && len(file.Sections) > 0 {
		builder.WriteByte('\n')
	}
	for i, section := range file.Sections {
		if section == nil {
			continue
		}
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteByte('[')
		builder.WriteString(escapeSectionName(section.Name))
		builder.WriteString("]\n")
		writeStatements(&builder, section.Statements)
	}
	return builder.String()
}

func writeStatements(builder *strings.Builder, statements []ast.Statement) {
	for _, statement := range statements {
		switch node := statement.(type) {
		case *ast.Assignment:
			builder.WriteString(formatKey(node.Key))
			builder.WriteByte('=')
			builder.WriteString(Expression(node.Value))
			builder.WriteByte('\n')
		case *ast.Comment:
			text := node.Text
			if text == "" || text[0] != ';' && text[0] != '#' {
				text = "; " + text
			}
			builder.WriteString(text)
			builder.WriteByte('\n')
		}
	}
}

// Expression formats one ConfigFile Variant value.
func Expression(expression ast.Expression) string {
	return expressionAt(expression, 0)
}

func expressionAt(expression ast.Expression, depth int) string {
	if expression == nil {
		return "null"
	}
	switch node := expression.(type) {
	case *ast.NullLiteral:
		return "null"
	case *ast.BoolLiteral:
		if node.Value {
			return "true"
		}
		return "false"
	case *ast.IntegerLiteral:
		if node.Raw == "" {
			return "0"
		}
		return node.Raw
	case *ast.FloatLiteral:
		if node.Raw == "" {
			return "0.0"
		}
		return node.Raw
	case *ast.StringLiteral:
		return node.Prefix + quoteString(node.Value)
	case *ast.Identifier:
		return node.Name
	case *ast.UnaryExpression:
		return node.Operator + expressionAt(node.Operand, depth)
	case *ast.ArrayLiteral:
		return formatArray(node, depth)
	case *ast.DictionaryLiteral:
		return formatDictionary(node, depth)
	case *ast.ConstructorCall:
		return formatConstructor(node, depth)
	case *ast.TypedArrayLiteral:
		return "Array[" + formatTypeRef(node.ElementType) + "](" + expressionAt(node.Value, depth) + ")"
	case *ast.TypedDictionaryLiteral:
		return "Dictionary[" + formatTypeRef(node.KeyType) + ", " + formatTypeRef(node.ValueType) + "](" + expressionAt(node.Value, depth) + ")"
	default:
		return "null"
	}
}

func formatArray(array *ast.ArrayLiteral, depth int) string {
	if array == nil {
		return "[]"
	}
	compact := make([]string, 0, len(array.Items))
	multiline := false
	for _, item := range array.Items {
		switch item := item.(type) {
		case *ast.ArrayElement:
			value := expressionAt(item.Value, depth+1)
			compact = append(compact, value)
			multiline = multiline || strings.Contains(value, "\n")
		case *ast.Comment:
			multiline = true
		}
	}
	if !multiline {
		return "[" + strings.Join(compact, ", ") + "]"
	}
	var builder strings.Builder
	builder.WriteString("[\n")
	for i, item := range array.Items {
		switch item := item.(type) {
		case *ast.ArrayElement:
			builder.WriteString(indent(depth + 1))
			builder.WriteString(expressionAt(item.Value, depth+1))
			if hasLaterArrayElement(array.Items, i) {
				builder.WriteByte(',')
			}
			builder.WriteByte('\n')
		case *ast.Comment:
			builder.WriteString(indent(depth + 1))
			builder.WriteString(commentText(item))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString(indent(depth))
	builder.WriteByte(']')
	return builder.String()
}

func formatDictionary(dictionary *ast.DictionaryLiteral, depth int) string {
	if dictionary == nil {
		return "{}"
	}
	compact := make([]string, 0, len(dictionary.Items))
	multiline := false
	for _, item := range dictionary.Items {
		switch item := item.(type) {
		case *ast.DictionaryEntry:
			key := expressionAt(item.Key, depth+1)
			value := expressionAt(item.Value, depth+1)
			compact = append(compact, key+": "+value)
			multiline = multiline || len(item.InfixComments) > 0 || strings.Contains(key, "\n") || strings.Contains(value, "\n")
		case *ast.Comment:
			multiline = true
		}
	}
	if !multiline {
		return "{" + strings.Join(compact, ", ") + "}"
	}
	var builder strings.Builder
	builder.WriteString("{\n")
	for i, item := range dictionary.Items {
		switch item := item.(type) {
		case *ast.DictionaryEntry:
			builder.WriteString(indent(depth + 1))
			builder.WriteString(expressionAt(item.Key, depth+1))
			builder.WriteByte(':')
			if len(item.InfixComments) == 0 {
				builder.WriteByte(' ')
				builder.WriteString(expressionAt(item.Value, depth+1))
			} else {
				builder.WriteByte('\n')
				for _, comment := range item.InfixComments {
					builder.WriteString(indent(depth + 2))
					builder.WriteString(commentText(comment))
					builder.WriteByte('\n')
				}
				builder.WriteString(indent(depth + 2))
				builder.WriteString(expressionAt(item.Value, depth+2))
			}
			if hasLaterDictionaryEntry(dictionary.Items, i) {
				builder.WriteByte(',')
			}
			builder.WriteByte('\n')
		case *ast.Comment:
			builder.WriteString(indent(depth + 1))
			builder.WriteString(commentText(item))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString(indent(depth))
	builder.WriteByte('}')
	return builder.String()
}

func formatConstructor(call *ast.ConstructorCall, depth int) string {
	if call == nil {
		return "null"
	}
	compact := make([]string, 0, len(call.Items))
	multiline := false
	for _, item := range call.Items {
		switch item := item.(type) {
		case *ast.ConstructorArgument:
			value := expressionAt(item.Value, depth+1)
			if item.Key != nil {
				value = expressionAt(item.Key, depth+1) + ": " + value
			}
			compact = append(compact, value)
			multiline = multiline || len(item.InfixComments) > 0 || strings.Contains(value, "\n")
		case *ast.Comment:
			multiline = true
		}
	}
	if !multiline {
		return call.Name + "(" + strings.Join(compact, ", ") + ")"
	}
	var builder strings.Builder
	builder.WriteString(call.Name)
	builder.WriteString("(\n")
	for i, item := range call.Items {
		switch item := item.(type) {
		case *ast.ConstructorArgument:
			builder.WriteString(indent(depth + 1))
			if item.Key != nil {
				builder.WriteString(expressionAt(item.Key, depth+1))
				builder.WriteByte(':')
				if len(item.InfixComments) == 0 {
					builder.WriteByte(' ')
					builder.WriteString(expressionAt(item.Value, depth+1))
				} else {
					builder.WriteByte('\n')
					for _, comment := range item.InfixComments {
						builder.WriteString(indent(depth + 2))
						builder.WriteString(commentText(comment))
						builder.WriteByte('\n')
					}
					builder.WriteString(indent(depth + 2))
					builder.WriteString(expressionAt(item.Value, depth+2))
				}
			} else {
				builder.WriteString(expressionAt(item.Value, depth+1))
			}
			if hasLaterConstructorArgument(call.Items, i) {
				builder.WriteByte(',')
			}
			builder.WriteByte('\n')
		case *ast.Comment:
			builder.WriteString(indent(depth + 1))
			builder.WriteString(commentText(item))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString(indent(depth))
	builder.WriteByte(')')
	return builder.String()
}

func formatTypeRef(ref *ast.TypeRef) string {
	if ref == nil {
		return "Variant"
	}
	if len(ref.Arguments) == 0 {
		return ref.Name
	}
	arguments := make([]string, len(ref.Arguments))
	for i, argument := range ref.Arguments {
		arguments[i] = formatTypeRef(argument)
	}
	return ref.Name + "[" + strings.Join(arguments, ", ") + "]"
}

func hasLaterArrayElement(items []ast.ArrayItem, current int) bool {
	for _, item := range items[current+1:] {
		if _, ok := item.(*ast.ArrayElement); ok {
			return true
		}
	}
	return false
}

func hasLaterDictionaryEntry(items []ast.DictionaryItem, current int) bool {
	for _, item := range items[current+1:] {
		if _, ok := item.(*ast.DictionaryEntry); ok {
			return true
		}
	}
	return false
}

func hasLaterConstructorArgument(items []ast.ConstructorItem, current int) bool {
	for _, item := range items[current+1:] {
		if _, ok := item.(*ast.ConstructorArgument); ok {
			return true
		}
	}
	return false
}

func indent(depth int) string { return strings.Repeat("\t", depth) }

func commentText(comment *ast.Comment) string {
	if comment == nil || comment.Text == "" {
		return ";"
	}
	if comment.Text[0] != ';' && comment.Text[0] != '#' {
		return "; " + comment.Text
	}
	return comment.Text
}

func formatKey(key string) string {
	if key != "" {
		valid := true
		for _, r := range key {
			if unicode.IsSpace(r) || strings.ContainsRune("=[];#\"'", r) {
				valid = false
				break
			}
		}
		if valid {
			return key
		}
	}
	return quoteString(key)
}

func escapeSectionName(name string) string {
	return strings.ReplaceAll(name, "]", `\]`)
}

func quoteString(value string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\a':
			builder.WriteString(`\a`)
		case '\b':
			builder.WriteString(`\b`)
		case '\t':
			builder.WriteString(`\t`)
		case '\n':
			builder.WriteString(`\n`)
		case '\v':
			builder.WriteString(`\v`)
		case '\f':
			builder.WriteString(`\f`)
		case '\r':
			builder.WriteString(`\r`)
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&builder, `\u%04x`, r)
			} else if r == utf8.RuneError {
				builder.WriteRune(utf8.RuneError)
			} else {
				builder.WriteRune(r)
			}
		}
	}
	builder.WriteByte('"')
	return builder.String()
}
