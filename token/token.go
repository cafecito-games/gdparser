// Package token defines the lexical tokens and source locations used by gdparser.
package token

import "fmt"

// Position is a zero-based byte offset and one-based line and column.
type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (p Position) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Column) }

// Span is a half-open source range [Start, End).
type Span struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Type identifies a lexical token.
type Type string

const (
	Illegal Type = "ILLEGAL"
	EOF     Type = "EOF"
	Newline Type = "NEWLINE"
	Indent  Type = "INDENT"
	Dedent  Type = "DEDENT"

	Identifier Type = "IDENTIFIER"
	Integer    Type = "INTEGER"
	Float      Type = "FLOAT"
	String     Type = "STRING"
	Comment    Type = "COMMENT"

	LParen    Type = "("
	RParen    Type = ")"
	LBracket  Type = "["
	RBracket  Type = "]"
	LBrace    Type = "{"
	RBrace    Type = "}"
	Comma     Type = ","
	Semicolon Type = ";"
	Colon     Type = ":"
	Dot       Type = "."
	// DotDot is the ".." that stands for the elements an array or dictionary
	// pattern does not list. Godot calls it PERIOD_PERIOD and keeps it apart from
	// the "..." of a rest parameter; GDScript has no range operator.
	DotDot   Type = ".."
	Ellipsis Type = "..."
	At       Type = "@"
	Dollar   Type = "$"
	Percent  Type = "%"
	Arrow    Type = "->"

	Assign           Type = "="
	InferAssign      Type = ":="
	Plus             Type = "+"
	Minus            Type = "-"
	Star             Type = "*"
	Slash            Type = "/"
	DoubleStar       Type = "**"
	DoubleStarAssign Type = "**="
	Equal            Type = "=="
	NotEqual         Type = "!="
	Less             Type = "<"
	LessEqual        Type = "<="
	Greater          Type = ">"
	GreaterEqual     Type = ">="
	ShiftLeft        Type = "<<"
	ShiftRight       Type = ">>"
	Ampersand        Type = "&"
	Pipe             Type = "|"
	Caret            Type = "^"
	Tilde            Type = "~"
	Bang             Type = "!"
	PlusAssign       Type = "+="
	MinusAssign      Type = "-="
	StarAssign       Type = "*="
	SlashAssign      Type = "/="
	PercentAssign    Type = "%="
	AmpAssign        Type = "&="
	PipeAssign       Type = "|="
	CaretAssign      Type = "^="
	ShiftLeftAssign  Type = "<<="
	ShiftRightAssign Type = ">>="

	And        Type = "and"
	As         Type = "as"
	Assert     Type = "assert"
	Await      Type = "await"
	Break      Type = "break"
	Breakpoint Type = "breakpoint"
	Class      Type = "class"
	ClassName  Type = "class_name"
	Const      Type = "const"
	Continue   Type = "continue"
	Elif       Type = "elif"
	Else       Type = "else"
	Enum       Type = "enum"
	Extends    Type = "extends"
	False      Type = "false"
	For        Type = "for"
	Func       Type = "func"
	If         Type = "if"
	In         Type = "in"
	Is         Type = "is"
	Match      Type = "match"
	Namespace  Type = "namespace"
	Not        Type = "not"
	Null       Type = "null"
	Or         Type = "or"
	Pass       Type = "pass"
	Preload    Type = "preload"
	Return     Type = "return"
	Self       Type = "self"
	Signal     Type = "signal"
	Static     Type = "static"
	Super      Type = "super"
	Trait      Type = "trait"
	True       Type = "true"
	Underscore Type = "_"
	Var        Type = "var"
	Void       Type = "void"
	When       Type = "when"
	While      Type = "while"
	Yield      Type = "yield"
)

// keywords holds every word GDScript reserves, mirroring the KEYWORDS list in
// Godot's modules/gdscript/gdscript_tokenizer.cpp. "tool" is deliberately
// absent: Godot 4 replaced it with the @tool annotation, so it is an ordinary
// identifier. PI, INF, NAN and TAU are keyword tokens for Godot, but it accepts
// them wherever an identifier or a node name may appear and nowhere else, so
// leaving them as identifiers here gives the same grammar with fewer tokens.
var keywords = map[string]Type{
	"and": And, "as": As, "assert": Assert, "await": Await, "break": Break,
	"breakpoint": Breakpoint, "class": Class, "class_name": ClassName,
	"const": Const, "continue": Continue, "elif": Elif, "else": Else,
	"enum": Enum, "extends": Extends, "false": False, "for": For, "func": Func,
	"if": If, "in": In, "is": Is, "match": Match, "namespace": Namespace,
	"not": Not, "null": Null, "or": Or, "pass": Pass, "preload": Preload,
	"return": Return, "self": Self, "signal": Signal, "static": Static,
	"super": Super, "trait": Trait, "true": True, "var": Var, "void": Void,
	"when": When, "while": While, "yield": Yield,
}

// keywordTypes holds the type of every keyword, so a token can be recognized
// as one without its text.
var keywordTypes = func() map[Type]bool {
	types := make(map[Type]bool, len(keywords))
	for _, typ := range keywords {
		types[typ] = true
	}
	return types
}()

// IsKeyword reports whether typ is one of the GDScript keywords.
func IsKeyword(typ Type) bool { return keywordTypes[typ] }

// LookupIdentifier returns the token type of a word read from source: a keyword
// when the word is reserved, Underscore for a lone underscore, and Identifier
// otherwise. Godot gives the lone underscore its own token, which is why it may
// stand for a match wildcard and for a node name but not for a declared name.
func LookupIdentifier(text string) Type {
	if text == "_" {
		return Underscore
	}
	if typ, ok := keywords[text]; ok {
		return typ
	}
	return Identifier
}

// identifierTokens lists the tokens GDScript accepts wherever it expects an
// identifier, mirroring Token::is_identifier in Godot's
// modules/gdscript/gdscript_tokenizer.cpp. Godot keeps "match" usable because of
// String.match, and "when" because it was reserved after code already used it.
var identifierTokens = map[Type]bool{
	Identifier: true, Match: true, When: true,
}

// nodeNameTokens lists the tokens that may name a node, mirroring
// Token::is_node_name in Godot's modules/gdscript/gdscript_tokenizer.cpp. Godot
// uses the same predicate for a "$" or "%" node path and for the member name
// after a dot, and it holds for nearly every keyword but not for a literal.
var nodeNameTokens = map[Type]bool{
	Identifier: true, And: true, As: true, Assert: true, Await: true,
	Break: true, Breakpoint: true, ClassName: true, Class: true, Const: true,
	Continue: true, Elif: true, Else: true, Enum: true, Extends: true,
	For: true, Func: true, If: true, In: true, Is: true, Match: true,
	Namespace: true, Not: true, Or: true, Pass: true, Preload: true,
	Return: true, Self: true, Signal: true, Static: true, Super: true,
	Trait: true, Underscore: true, Var: true, Void: true, While: true,
	When: true, Yield: true,
}

// IsIdentifier reports whether typ may stand where GDScript expects an
// identifier, such as a declared name or a parameter.
func IsIdentifier(typ Type) bool { return identifierTokens[typ] }

// IsNodeName reports whether typ may name a node in a "$" or "%" path, or a
// member after a dot.
func IsNodeName(typ Type) bool { return nodeNameTokens[typ] }

// Token is a lexeme and its source location.
type Token struct {
	Type   Type   `json:"type"`
	Lexeme string `json:"lexeme,omitempty"`
	Span   Span   `json:"span"`
}

func (t Token) String() string {
	if t.Lexeme == "" {
		return string(t.Type)
	}
	return fmt.Sprintf("%s(%q)", t.Type, t.Lexeme)
}
