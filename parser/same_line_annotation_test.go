package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

// An annotation of the script itself may share its line with what follows it.
// Godot's parse_program applies one where it reads it and asks for a line break
// only after a standalone annotation. Where the annotation was written is kept,
// since the style guide does not prescribe it, so the tree read back from the
// formatted source says the same.
func TestScriptAnnotationSharingALine(t *testing.T) {
	for _, source := range []string{
		"@tool extends Node\n",
		"@icon(\"res://i.svg\") class_name X extends Node\n",
		"@tool @icon(\"res://i.svg\") extends Node\n",
		"@static_unload class_name X\n",
		"@abstract class_name X extends Node\n",
		"@tool\nextends Node\n",
		"@tool  # c\nextends Node\n",
		"@tool\n@icon(\"res://i.svg\")\nclass_name X\nextends Node\n",
	} {
		file, err := parser.Parse("script.gd", []byte(source))
		if err != nil {
			t.Errorf("parse %q: %v", source, err)
			continue
		}
		got := gdformat.File(file)
		if got != source {
			t.Errorf("formatted %q, want %q", got, source)
		}
		reparsed, err := parser.Parse("script.gd", []byte(got))
		if err != nil {
			t.Errorf("reparse %q: %v", got, err)
			continue
		}
		var before, after strings.Builder
		for index, statement := range file.Statements {
			annotation, ok := statement.(*ast.Annotation)
			if !ok {
				continue
			}
			again, ok := reparsed.Statements[index].(*ast.Annotation)
			if !ok || again.OwnLine != annotation.OwnLine {
				t.Errorf("%q: annotation %d changed where it stands", source, index)
			}
		}
		if err := ast.Dump(&before, file); err != nil {
			t.Fatal(err)
		}
		if err := ast.Dump(&after, reparsed); err != nil {
			t.Fatal(err)
		}
		if before.String() != after.String() {
			t.Errorf("%q: the tree changed", source)
		}
	}
}

func TestScriptAnnotationOnASharedLineIsNotOwnLine(t *testing.T) {
	file, err := parser.Parse("script.gd", []byte("@tool extends Node\n"))
	if err != nil {
		t.Fatal(err)
	}
	annotation := file.Statements[0].(*ast.Annotation)
	if annotation.Name != "tool" || annotation.OwnLine {
		t.Fatalf("annotation = %#v, want @tool sharing its line", annotation)
	}
	if directive, ok := file.Statements[1].(*ast.Directive); !ok || directive.Name != "extends" {
		t.Fatalf("statement after the annotation = %#v, want extends", file.Statements[1])
	}
}

// A standalone annotation must end its line.
func TestStandaloneAnnotationStillEndsItsLine(t *testing.T) {
	_, err := parser.Parse("script.gd", []byte("@export_category(\"a\") var x\n"))
	if err == nil || !strings.Contains(err.Error(), "expected end of line after the annotation") {
		t.Fatalf("error = %v, want the end-of-line rule", err)
	}
}
