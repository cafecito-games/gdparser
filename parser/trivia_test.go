package parser_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/parser"
)

func parseSource(t *testing.T, source string) *ast.File {
	t.Helper()
	file, err := parser.Parse("test.gd", []byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return file
}

func TestBlankLinesBeforeAreCounted(t *testing.T) {
	file := parseSource(t, "var a := 1\n\n\nvar b := 2\nvar c := 3\n")
	want := []int{0, 2, 0}
	for index, expected := range want {
		if got := ast.TriviaOf(file.Statements[index]).BlankLinesBefore; got != expected {
			t.Errorf("statement %d blank lines = %d, want %d", index, got, expected)
		}
	}
}

// Blank lines written after a nested block belong to the statement that follows
// the block, even though the block's own parse consumed them.
func TestBlankLinesSurviveADedent(t *testing.T) {
	file := parseSource(t, "func f(a):\n\tif a:\n\t\tpass\n\n\tvar b := 2\n")
	body := file.Statements[0].(*ast.FunctionDeclaration).Body
	if got := ast.TriviaOf(body[1]).BlankLinesBefore; got != 1 {
		t.Fatalf("blank lines after block = %d, want 1", got)
	}
}

func TestTrailingCommentAttachesToItsStatement(t *testing.T) {
	file := parseSource(t, "var a := 1  # why\n# own line\n")
	if len(file.Statements) != 2 {
		t.Fatalf("got %d statements", len(file.Statements))
	}
	trailing := ast.TriviaOf(file.Statements[0]).TrailingComment
	if trailing == nil || trailing.Text != "# why" {
		t.Fatalf("trailing comment = %#v", trailing)
	}
	if _, ok := file.Statements[1].(*ast.Comment); !ok {
		t.Fatalf("statement 1 = %T, want a standalone comment", file.Statements[1])
	}
}

func TestTrailingCommentIsAChild(t *testing.T) {
	file := parseSource(t, "var a := 1  # why\n")
	children := ast.Children(file.Statements[0])
	if len(children) == 0 {
		t.Fatal("declaration has no children")
	}
	last, ok := children[len(children)-1].(*ast.Comment)
	if !ok || last.Text != "# why" {
		t.Fatalf("last child = %#v, want the trailing comment", children[len(children)-1])
	}
}

// A standalone annotation decorates nothing, so it stands as a statement of its
// own rather than attaching to the declaration that follows it. Godot adds the
// export group markers to the class rather than to the member below them.
func TestStandaloneAnnotationDecoratesNothing(t *testing.T) {
	file := parseSource(t, "@export_group(\"Move\")\n@export var jump := 2.0\n")
	if len(file.Statements) != 2 {
		t.Fatalf("got %d statements, want the group marker and the declaration", len(file.Statements))
	}
	group, ok := file.Statements[0].(*ast.Annotation)
	if !ok || group.Name != "export_group" {
		t.Fatalf("statement 0 = %#v, want the export_group annotation", file.Statements[0])
	}
	attached := ast.Annotations(file.Statements[1])
	if len(attached) != 1 || attached[0].Name != "export" {
		t.Fatalf("the declaration carries %d annotations, want just export", len(attached))
	}
}

func TestAnnotationsAttachToDeclarations(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		names   []string
		ownLine []bool
	}{
		{"inline", "@export var speed := 1.0\n", []string{"export"}, []bool{false}},
		{"own line", "@onready\nvar node := $Sprite2D\n", []string{"onready"}, []bool{true}},
		{"several", "@onready\n@export var jump := 2.0\n", []string{"onready", "export"}, []bool{true, false}},
		{"on a function", "@rpc(\"any_peer\")\nfunc f() -> void:\n\tpass\n", []string{"rpc"}, []bool{true}},
		{"on a signal", "@warning_ignore(\"unused\") signal done()\n", []string{"warning_ignore"}, []bool{false}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			file := parseSource(t, testCase.source)
			if len(file.Statements) != 1 {
				t.Fatalf("got %d statements, want the declaration alone", len(file.Statements))
			}
			annotations := ast.Annotations(file.Statements[0])
			if len(annotations) != len(testCase.names) {
				t.Fatalf("got %d annotations, want %d", len(annotations), len(testCase.names))
			}
			for index, name := range testCase.names {
				if annotations[index].Name != name {
					t.Errorf("annotation %d = %q, want %q", index, annotations[index].Name, name)
				}
				if annotations[index].OwnLine != testCase.ownLine[index] {
					t.Errorf("annotation %d own line = %v, want %v", index, annotations[index].OwnLine, testCase.ownLine[index])
				}
			}
			for _, annotation := range ast.Children(file.Statements[0]) {
				if _, ok := annotation.(*ast.Annotation); ok {
					return
				}
			}
			t.Error("annotations are not reported as children")
		})
	}
}

// An annotation that decorates nothing stays a statement of its own.
func TestUnattachableAnnotationRemainsAStatement(t *testing.T) {
	file := parseSource(t, "@icon(\"res://icon.svg\")\nextends Node\n")
	if len(file.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(file.Statements))
	}
	if _, ok := file.Statements[0].(*ast.Annotation); !ok {
		t.Fatalf("statement 0 = %T, want an annotation", file.Statements[0])
	}
}

func TestStringLiteralShapeIsRecorded(t *testing.T) {
	cases := []struct {
		source    string
		quote     byte
		triple    bool
		rawPrefix bool
	}{
		{"var a := \"text\"\n", '"', false, false},
		{"var a := 'text'\n", '\'', false, false},
		{"var a := \"\"\"text\"\"\"\n", '"', true, false},
		{"var a := r\"text\"\n", '"', false, true},
		{"var a := r'''text'''\n", '\'', true, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.source, func(t *testing.T) {
			file := parseSource(t, testCase.source)
			declaration := file.Statements[0].(*ast.VariableDeclaration)
			literal, ok := declaration.Value.(*ast.Literal)
			if !ok {
				t.Fatalf("value = %T", declaration.Value)
			}
			if literal.Quote != testCase.quote {
				t.Errorf("quote = %q, want %q", literal.Quote, testCase.quote)
			}
			if literal.Triple != testCase.triple {
				t.Errorf("triple = %v, want %v", literal.Triple, testCase.triple)
			}
			if literal.RawPrefix != testCase.rawPrefix {
				t.Errorf("raw prefix = %v, want %v", literal.RawPrefix, testCase.rawPrefix)
			}
		})
	}
}

func TestRawStringKeepsItsBackslashes(t *testing.T) {
	file := parseSource(t, "var pattern := r\"\\d+\"\n")
	declaration := file.Statements[0].(*ast.VariableDeclaration)
	literal := declaration.Value.(*ast.Literal)
	if literal.Raw != `r"\d+"` {
		t.Fatalf("raw = %s, want %s", literal.Raw, `r"\d+"`)
	}
}

func TestLeadingDotFloatParses(t *testing.T) {
	file := parseSource(t, "var a := .5\n")
	declaration := file.Statements[0].(*ast.VariableDeclaration)
	literal, ok := declaration.Value.(*ast.Literal)
	if !ok || literal.Kind != ast.FloatLiteral || literal.Raw != ".5" {
		t.Fatalf("value = %#v", declaration.Value)
	}
}

func TestByteOrderMarkIsSkipped(t *testing.T) {
	file := parseSource(t, "\xef\xbb\xbfextends Node\n")
	if len(file.Statements) != 1 {
		t.Fatalf("got %d statements", len(file.Statements))
	}
	directive, ok := file.Statements[0].(*ast.Directive)
	if !ok || directive.Name != "extends" {
		t.Fatalf("statement 0 = %#v", file.Statements[0])
	}
	// The mark is skipped rather than treated as source, so the first token still
	// reports column one of line one.
	if start := directive.KeywordSpan.Start; start.Line != 1 || start.Column != 1 {
		t.Fatalf("keyword starts at %d:%d, want 1:1", start.Line, start.Column)
	}
}

// Blank lines written after a property's accessors belong to the statement that
// follows the declaration.
func TestBlankLinesSurviveAPropertyBlock(t *testing.T) {
	source := "var a: int = 1:\n\tset(value):\n\t\ta = value\n\nvar b := 2\n"
	file := parseSource(t, source)
	if got := ast.TriviaOf(file.Statements[1]).BlankLinesBefore; got != 1 {
		t.Fatalf("blank lines after accessors = %d, want 1", got)
	}
}

// Blank lines written after the last match case belong to the statement that
// follows the match.
func TestBlankLinesSurviveAMatchBlock(t *testing.T) {
	source := "func f(v):\n\tmatch v:\n\t\t0:\n\t\t\tpass\n\n\tvar a := 1\n"
	file := parseSource(t, source)
	function := file.Statements[0].(*ast.FunctionDeclaration)
	if got := ast.TriviaOf(function.Body[1]).BlankLinesBefore; got != 1 {
		t.Fatalf("blank lines after match = %d, want 1", got)
	}
}
