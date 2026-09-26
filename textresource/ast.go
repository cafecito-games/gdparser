// Package textresource parses and formats Godot 4 text scenes and resources.
//
// The package supports the shared format used by .tscn, .tres, and .escn
// files. Its syntax tree is mutable and retains source spans and comments.
package textresource

import "github.com/cafecito-games/gdparser/token"

// Node is implemented by every syntax-tree node.
type Node interface {
	Span() token.Span
	node()
}

// Item is a document or section body item.
type Item interface {
	Node
	item()
}

// Value is a serialized Godot Variant value.
type Value interface {
	Node
	value()
}

// CompositeItem is an item in an array or call argument list.
type CompositeItem interface {
	Node
	compositeItem()
}

// DictionaryItem is an entry or comment in a dictionary.
type DictionaryItem interface {
	Node
	dictionaryItem()
}

// Base stores source location data embedded by concrete nodes.
type Base struct {
	SourceSpan token.Span `json:"span"`
}

// Span returns the node's half-open source range.
func (b Base) Span() token.Span { return b.SourceSpan }

// Document is a parsed .tscn, .tres, or .escn file.
type Document struct {
	Base
	Name  string `json:"name,omitempty"`
	Items []Item `json:"items"`
}

func (*Document) node() {}

// Section is a bracketed section header followed by assignments and comments.
type Section struct {
	Base
	Type          string       `json:"type"`
	Attributes    []*Attribute `json:"attributes,omitempty"`
	HeaderComment *Comment     `json:"header_comment,omitempty"`
	Body          []Item       `json:"body,omitempty"`
}

func (*Section) node() {}
func (*Section) item() {}

// Attribute is an ordered name=value pair in a section header.
type Attribute struct {
	Base
	Name  string `json:"name"`
	Value Value  `json:"value"`
}

func (*Attribute) node() {}

// Assignment sets a property in a section body.
type Assignment struct {
	Base
	Property        string   `json:"property"`
	Value           Value    `json:"value"`
	TrailingComment *Comment `json:"trailing_comment,omitempty"`
}

func (*Assignment) node() {}
func (*Assignment) item() {}

// Comment preserves a single-line ; or # comment. Text excludes the marker.
type Comment struct {
	Base
	Text string `json:"text"`
}

func (*Comment) node()           {}
func (*Comment) item()           {}
func (*Comment) compositeItem()  {}
func (*Comment) dictionaryItem() {}

// NullValue is the null Variant value.
type NullValue struct{ Base }

func (*NullValue) node()          {}
func (*NullValue) value()         {}
func (*NullValue) compositeItem() {}

// BoolValue is a Boolean Variant value.
type BoolValue struct {
	Base
	Value bool `json:"value"`
}

func (*BoolValue) node()          {}
func (*BoolValue) value()         {}
func (*BoolValue) compositeItem() {}

// IntegerValue is a signed integer Variant value.
type IntegerValue struct {
	Base
	Value         int64  `json:"value"`
	UnsignedValue uint64 `json:"unsigned_value,omitempty"`
	Suffix        string `json:"suffix,omitempty"`
}

func (*IntegerValue) node()          {}
func (*IntegerValue) value()         {}
func (*IntegerValue) compositeItem() {}

// FloatValue is a floating-point Variant value.
type FloatValue struct {
	Base
	Value float64 `json:"value"`
}

func (*FloatValue) node()          {}
func (*FloatValue) value()         {}
func (*FloatValue) compositeItem() {}

// StringKind identifies the serialized meaning of a quoted string.
type StringKind string

const (
	String     StringKind = "string"
	StringName StringKind = "string_name"
	NodePath   StringKind = "node_path"
)

// StringValue is a string, StringName (&"x"), or NodePath (^"x").
type StringValue struct {
	Base
	Value string     `json:"value"`
	Kind  StringKind `json:"string_kind,omitempty"`
}

func (*StringValue) node()          {}
func (*StringValue) value()         {}
func (*StringValue) compositeItem() {}

// IdentifierValue is a named constant such as inf, nan, or Color.RED.
type IdentifierValue struct {
	Base
	Name string `json:"name"`
}

func (*IdentifierValue) node()          {}
func (*IdentifierValue) value()         {}
func (*IdentifierValue) compositeItem() {}

// ArrayValue is an ordered Variant array. Items may include comments.
type ArrayValue struct {
	Base
	Items []CompositeItem `json:"items,omitempty"`
}

func (*ArrayValue) node()          {}
func (*ArrayValue) value()         {}
func (*ArrayValue) compositeItem() {}

// DictionaryEntry is one key:value pair.
type DictionaryEntry struct {
	Base
	Key   Value `json:"key"`
	Value Value `json:"value"`
}

func (*DictionaryEntry) node()           {}
func (*DictionaryEntry) dictionaryItem() {}

// DictionaryValue is an ordered Variant dictionary. Items may include comments.
type DictionaryValue struct {
	Base
	Items []DictionaryItem `json:"items,omitempty"`
}

func (*DictionaryValue) node()          {}
func (*DictionaryValue) value()         {}
func (*DictionaryValue) compositeItem() {}

// CallValue is a serialized constructor or resource reference.
type CallValue struct {
	Base
	Name      string          `json:"name"`
	Arguments []CompositeItem `json:"arguments,omitempty"`
}

func (*CallValue) node()          {}
func (*CallValue) value()         {}
func (*CallValue) compositeItem() {}

// TypeRef describes a type argument in Array[T] or Dictionary[K, V].
type TypeRef struct {
	Base
	Name        string     `json:"name"`
	Arguments   []*TypeRef `json:"arguments,omitempty"`
	Constructor *CallValue `json:"constructor,omitempty"`
}

func (*TypeRef) node() {}

// TypedArrayValue is the Array[T]([...]) serialization form.
type TypedArrayValue struct {
	Base
	ElementType *TypeRef    `json:"element_type"`
	Array       *ArrayValue `json:"array"`
}

func (*TypedArrayValue) node()          {}
func (*TypedArrayValue) value()         {}
func (*TypedArrayValue) compositeItem() {}

// TypedDictionaryValue is the Dictionary[K, V]({...}) serialization form.
type TypedDictionaryValue struct {
	Base
	KeyType    *TypeRef         `json:"key_type"`
	ValueType  *TypeRef         `json:"value_type"`
	Dictionary *DictionaryValue `json:"dictionary"`
}

func (*TypedDictionaryValue) node()          {}
func (*TypedDictionaryValue) value()         {}
func (*TypedDictionaryValue) compositeItem() {}
