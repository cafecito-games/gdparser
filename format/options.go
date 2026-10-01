package format

import "strings"

// Indent selects the character used for one level of indentation.
type Indent int

const (
	// Tabs indents with one tab per level, as the Godot style guide requires.
	Tabs Indent = iota
	// Spaces indents with TabWidth spaces per level.
	Spaces
)

// QuoteStyle selects the preferred quote character for string literals.
type QuoteStyle int

const (
	// DoubleQuotes rewrites single-quoted strings that need no extra escaping.
	DoubleQuotes QuoteStyle = iota
	// SingleQuotes rewrites double-quoted strings that need no extra escaping.
	SingleQuotes
	// PreserveQuotes emits every string literal exactly as it was written.
	PreserveQuotes
)

// CommentSpacing selects whether comment bodies are normalized.
type CommentSpacing int

const (
	// NormalizeComments ensures one space follows the # or ## marker. Region
	// markers are always left alone.
	NormalizeComments CommentSpacing = iota
	// PreserveComments emits every comment exactly as it was written.
	PreserveComments
)

// OperatorStyle selects the spelling of the boolean operators.
type OperatorStyle int

const (
	// WordOperators emits and, or, and not in place of &&, ||, and !.
	WordOperators OperatorStyle = iota
	// PreserveOperators emits boolean operators as they were written.
	PreserveOperators
)

// NumberStyle selects whether numeric literals are normalized.
type NumberStyle int

const (
	// NormalizeNumbers restores omitted leading and trailing zeros in floats
	// and lowercases radix prefixes and hexadecimal digits.
	NormalizeNumbers NumberStyle = iota
	// PreserveNumbers emits every numeric literal exactly as it was written.
	PreserveNumbers
)

// TrailingCommaStyle selects whether broken collections end with a comma.
type TrailingCommaStyle int

const (
	// TrailingCommasWhenBroken appends a comma to the last element of an array,
	// dictionary, or enum that is split over multiple lines.
	TrailingCommasWhenBroken TrailingCommaStyle = iota
	// NoTrailingCommas never appends a trailing comma, except after an item
	// that holds the line it ends on, where the comma is what lets the
	// construct's bracket leave the block rather than a style. See
	// closingComma for which shapes need it.
	NoTrailingCommas
)

// BlankLineLimits controls blank line placement. TopLevel is the exact number
// of blank lines placed around top-level function and class declarations.
// Nested is the maximum number of consecutive blank lines kept anywhere else.
type BlankLineLimits struct {
	TopLevel int `json:"top_level,omitempty"`
	Nested   int `json:"nested,omitempty"`
}

// Options configures canonical GDScript emission. The zero value is equivalent
// to GodotStyle, so a partially populated Options inherits the rest of the
// Godot style guide defaults.
type Options struct {
	// LineWidth is the column budget a line is kept within where possible.
	LineWidth int `json:"line_width,omitempty"`
	// TabWidth is the number of columns a tab occupies when measuring a line,
	// and the number of spaces per level when Indent is Spaces.
	TabWidth       int                `json:"tab_width,omitempty"`
	Indent         Indent             `json:"indent,omitempty"`
	QuoteStyle     QuoteStyle         `json:"quote_style,omitempty"`
	CommentSpacing CommentSpacing     `json:"comment_spacing,omitempty"`
	Operators      OperatorStyle      `json:"operators,omitempty"`
	Numbers        NumberStyle        `json:"numbers,omitempty"`
	TrailingCommas TrailingCommaStyle `json:"trailing_commas,omitempty"`
	BlankLines     BlankLineLimits    `json:"blank_lines,omitzero"`
}

// GodotStyle returns the options described by the Godot GDScript style guide.
func GodotStyle() Options {
	return Options{
		LineWidth:      100,
		TabWidth:       4,
		Indent:         Tabs,
		QuoteStyle:     DoubleQuotes,
		CommentSpacing: NormalizeComments,
		Operators:      WordOperators,
		Numbers:        NormalizeNumbers,
		TrailingCommas: TrailingCommasWhenBroken,
		BlankLines:     BlankLineLimits{TopLevel: 2, Nested: 1},
	}
}

// normalized replaces unset numeric fields with their GodotStyle values.
func (o Options) normalized() Options {
	defaults := GodotStyle()
	if o.LineWidth <= 0 {
		o.LineWidth = defaults.LineWidth
	}
	if o.TabWidth <= 0 {
		o.TabWidth = defaults.TabWidth
	}
	if o.BlankLines.TopLevel <= 0 {
		o.BlankLines.TopLevel = defaults.BlankLines.TopLevel
	}
	if o.BlankLines.Nested <= 0 {
		o.BlankLines.Nested = defaults.BlankLines.Nested
	}
	return o
}

// indentUnit returns the text of one indentation level.
func (o Options) indentUnit() string {
	if o.Indent == Spaces {
		return strings.Repeat(" ", o.TabWidth)
	}
	return "\t"
}

// indentColumns returns the number of columns one indentation level occupies.
func (o Options) indentColumns() int { return o.TabWidth }
