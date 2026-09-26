package textresource

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Format emits canonical, parseable Godot text-resource source.
func Format(document *Document) string {
	if document == nil {
		return ""
	}
	var f formatter
	for i, item := range document.Items {
		if i > 0 {
			if _, ok := item.(*Section); ok {
				if f.b.Len() > 0 && !strings.HasSuffix(f.b.String(), "\n\n") {
					f.b.WriteByte('\n')
				}
			}
		}
		f.item(item)
	}
	return f.b.String()
}

type formatter struct {
	b      strings.Builder
	indent int
}

func (f *formatter) item(item Item) {
	switch n := item.(type) {
	case *Comment:
		f.comment(n)
	case *Section:
		f.b.WriteByte('[')
		f.b.WriteString(n.Type)
		for _, a := range n.Attributes {
			if a == nil {
				continue
			}
			f.b.WriteByte(' ')
			f.b.WriteString(a.Name)
			f.b.WriteByte('=')
			f.value(a.Value)
		}
		f.b.WriteByte(']')
		if n.HeaderComment != nil {
			f.b.WriteString(" ;")
			f.b.WriteString(strings.TrimSpace(n.HeaderComment.Text))
		}
		f.b.WriteByte('\n')
		for _, body := range n.Body {
			f.item(body)
		}
	case *Assignment:
		f.b.WriteString(n.Property)
		f.b.WriteString(" = ")
		f.value(n.Value)
		if n.TrailingComment != nil {
			f.b.WriteString(" ;")
			f.b.WriteString(strings.TrimSpace(n.TrailingComment.Text))
		}
		f.b.WriteByte('\n')
	}
}
func (f *formatter) comment(c *Comment) {
	f.writeIndent()
	f.b.WriteByte(';')
	f.b.WriteString(c.Text)
	f.b.WriteByte('\n')
}
func (f *formatter) value(value Value) {
	if value == nil {
		f.b.WriteString("null")
		return
	}
	switch n := value.(type) {
	case *NullValue:
		f.b.WriteString("null")
	case *BoolValue:
		if n.Value {
			f.b.WriteString("true")
		} else {
			f.b.WriteString("false")
		}
	case *IntegerValue:
		if n.Suffix == "U" || n.Suffix == "UL" {
			f.b.WriteString(strconv.FormatUint(n.UnsignedValue, 10))
		} else {
			f.b.WriteString(strconv.FormatInt(n.Value, 10))
		}
		f.b.WriteString(n.Suffix)
	case *FloatValue:
		switch {
		case math.IsNaN(n.Value):
			f.b.WriteString("nan")
		case math.IsInf(n.Value, 1):
			f.b.WriteString("inf")
		case math.IsInf(n.Value, -1):
			f.b.WriteString("-inf")
		default:
			s := strconv.FormatFloat(n.Value, 'g', -1, 64)
			if !strings.ContainsAny(s, ".eE") {
				s += ".0"
			}
			f.b.WriteString(s)
		}
	case *StringValue:
		if n.Kind == StringName {
			f.b.WriteByte('&')
		} else if n.Kind == NodePath {
			f.b.WriteByte('^')
		}
		writeQuoted(&f.b, n.Value)
	case *IdentifierValue:
		f.b.WriteString(n.Name)
	case *ArrayValue:
		f.array(n)
	case *DictionaryValue:
		f.dictionary(n)
	case *CallValue:
		f.call(n)
	case *TypedArrayValue:
		f.b.WriteString("Array[")
		f.typeRef(n.ElementType)
		f.b.WriteString("](")
		if n.Array == nil {
			f.b.WriteString("[]")
		} else {
			f.array(n.Array)
		}
		f.b.WriteByte(')')
	case *TypedDictionaryValue:
		f.b.WriteString("Dictionary[")
		f.typeRef(n.KeyType)
		f.b.WriteString(", ")
		f.typeRef(n.ValueType)
		f.b.WriteString("](")
		if n.Dictionary == nil {
			f.b.WriteString("{}")
		} else {
			f.dictionary(n.Dictionary)
		}
		f.b.WriteByte(')')
	}
}
func (f *formatter) array(array *ArrayValue) {
	if !compositeHasComments(array.Items) {
		f.b.WriteByte('[')
		f.compositeInline(array.Items)
		f.b.WriteByte(']')
		return
	}
	f.b.WriteString("[\n")
	f.indent++
	for _, item := range array.Items {
		f.writeIndent()
		if c, ok := item.(*Comment); ok {
			f.b.WriteByte(';')
			f.b.WriteString(c.Text)
			f.b.WriteByte('\n')
			continue
		}
		if value, ok := item.(Value); ok {
			f.value(value)
			f.b.WriteString(",\n")
		}
	}
	f.indent--
	f.writeIndent()
	f.b.WriteByte(']')
}
func (f *formatter) dictionary(dictionary *DictionaryValue) {
	comments := false
	for _, item := range dictionary.Items {
		if _, ok := item.(*Comment); ok {
			comments = true
			break
		}
	}
	if !comments {
		f.b.WriteByte('{')
		for i, item := range dictionary.Items {
			entry, ok := item.(*DictionaryEntry)
			if !ok || entry == nil {
				continue
			}
			if i > 0 {
				f.b.WriteString(", ")
			}
			f.value(entry.Key)
			f.b.WriteString(": ")
			f.value(entry.Value)
		}
		f.b.WriteByte('}')
		return
	}
	f.b.WriteString("{\n")
	f.indent++
	for _, item := range dictionary.Items {
		f.writeIndent()
		switch n := item.(type) {
		case *Comment:
			f.b.WriteByte(';')
			f.b.WriteString(n.Text)
			f.b.WriteByte('\n')
		case *DictionaryEntry:
			f.value(n.Key)
			f.b.WriteString(": ")
			f.value(n.Value)
			f.b.WriteString(",\n")
		}
	}
	f.indent--
	f.writeIndent()
	f.b.WriteByte('}')
}
func (f *formatter) call(call *CallValue) {
	f.b.WriteString(call.Name)
	f.b.WriteByte('(')
	if compositeHasComments(call.Arguments) {
		f.b.WriteByte('\n')
		f.indent++
		for _, item := range call.Arguments {
			f.writeIndent()
			if c, ok := item.(*Comment); ok {
				f.b.WriteByte(';')
				f.b.WriteString(c.Text)
				f.b.WriteByte('\n')
			} else if v, ok := item.(Value); ok {
				f.value(v)
				f.b.WriteString(",\n")
			}
		}
		f.indent--
		f.writeIndent()
	} else {
		f.compositeInline(call.Arguments)
	}
	f.b.WriteByte(')')
}
func (f *formatter) compositeInline(items []CompositeItem) {
	written := false
	for _, item := range items {
		value, ok := item.(Value)
		if !ok {
			continue
		}
		if written {
			f.b.WriteString(", ")
		}
		f.value(value)
		written = true
	}
}
func compositeHasComments(items []CompositeItem) bool {
	for _, item := range items {
		if _, ok := item.(*Comment); ok {
			return true
		}
	}
	return false
}
func (f *formatter) typeRef(ref *TypeRef) {
	if ref == nil {
		f.b.WriteString("Variant")
		return
	}
	if ref.Constructor != nil {
		f.call(ref.Constructor)
		return
	}
	f.b.WriteString(ref.Name)
	if len(ref.Arguments) > 0 {
		f.b.WriteByte('[')
		for i, arg := range ref.Arguments {
			if i > 0 {
				f.b.WriteString(", ")
			}
			f.typeRef(arg)
		}
		f.b.WriteByte(']')
	}
}
func (f *formatter) writeIndent() { f.b.WriteString(strings.Repeat("  ", f.indent)) }
func writeQuoted(b *strings.Builder, value string) {
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '"':
			b.WriteString("\\\"")
		case '\b':
			b.WriteString("\\b")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
		default:
			if unicode.IsControl(r) {
				if r <= 0xffff {
					b.WriteString("\\u")
					b.WriteString(hex4(uint16(r)))
				} else {
					b.WriteString("\\U")
					s := strconv.FormatInt(int64(r), 16)
					b.WriteString(strings.Repeat("0", 6-len(s)))
					b.WriteString(s)
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
func hex4(v uint16) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>12], digits[v>>8&15], digits[v>>4&15], digits[v&15]})
}
