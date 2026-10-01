package ast_test

import (
	"testing"

	"github.com/cafecito-games/gdparser"
	"github.com/cafecito-games/gdparser/ast"
)

func TestInspectSupportsTransformations(t *testing.T) {
	file, err := gdparser.ParseString("var old_name = old_name + 1\n")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if identifier, ok := node.(*ast.Identifier); ok && identifier.Name == "old_name" {
			identifier.Name = "new_name"
			count++
		}
		return true
	})
	if count != 1 {
		t.Fatalf("renamed %d identifiers", count)
	}
	if got := gdparser.Format(file); got != "var old_name = new_name + 1\n" {
		t.Fatalf("formatted = %q", got)
	}
}

func TestJSONValueIncludesNodeKinds(t *testing.T) {
	file, err := gdparser.ParseString("return 1\n")
	if err != nil {
		t.Fatal(err)
	}
	value := ast.JSONValue(file).(map[string]any)
	if value["kind"] != "File" {
		t.Fatalf("file kind = %#v", value["kind"])
	}
	statement := value["statements"].([]any)[0].(map[string]any)
	if statement["kind"] != "ReturnStatement" {
		t.Fatalf("statement kind = %#v", statement["kind"])
	}
}

func TestJSONValueIncludesTriviaAndAnnotations(t *testing.T) {
	file, err := gdparser.ParseString("@export var speed := 1.0  # why\n")
	if err != nil {
		t.Fatal(err)
	}
	statement := ast.JSONValue(file).(map[string]any)["statements"].([]any)[0].(map[string]any)
	annotations, ok := statement["annotations"].([]any)
	if !ok || len(annotations) != 1 {
		t.Fatalf("annotations = %#v", statement["annotations"])
	}
	if name := annotations[0].(map[string]any)["name"]; name != "export" {
		t.Fatalf("annotation name = %#v", name)
	}
	trailing, ok := statement["trailing_comment"].(map[string]any)
	if !ok || trailing["text"] != "# why" {
		t.Fatalf("trailing comment = %#v", statement["trailing_comment"])
	}
}

// A statement that carries neither trivia value omits both fields.
func TestJSONValueOmitsEmptyTrivia(t *testing.T) {
	file, err := gdparser.ParseString("pass\n")
	if err != nil {
		t.Fatal(err)
	}
	statement := ast.JSONValue(file).(map[string]any)["statements"].([]any)[0].(map[string]any)
	for _, key := range []string{"annotations", "trailing_comment", "blank_lines_before"} {
		if _, present := statement[key]; present {
			t.Errorf("%s should be omitted when empty", key)
		}
	}
}

func TestTriviaOfReportsNilForUnsupportedStatements(t *testing.T) {
	if got := ast.TriviaOf(nil); got != nil {
		t.Fatalf("TriviaOf(nil) = %#v", got)
	}
}

func TestAnnotationsReportsNilForOtherNodes(t *testing.T) {
	if got := ast.Annotations(&ast.Identifier{Name: "x"}); got != nil {
		t.Fatalf("Annotations(identifier) = %#v", got)
	}
}
