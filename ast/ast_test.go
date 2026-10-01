package ast_test

import (
	"strings"
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
	file, err := gdparser.ParseString("func f():\n\treturn 1\n")
	if err != nil {
		t.Fatal(err)
	}
	value := ast.JSONValue(file).(map[string]any)
	if value["kind"] != "File" {
		t.Fatalf("file kind = %#v", value["kind"])
	}
	function := value["statements"].([]any)[0].(map[string]any)
	if function["kind"] != "FunctionDeclaration" {
		t.Fatalf("statement kind = %#v", function["kind"])
	}
	statement := function["body"].([]any)[0].(map[string]any)
	if statement["kind"] != "ReturnStatement" {
		t.Fatalf("body statement kind = %#v", statement["kind"])
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

func TestJSONValueIncludesCollectionComments(t *testing.T) {
	file, err := gdparser.ParseString("var x = [\n\t1,  # one\n\t# two\n\t2,\n]\n")
	if err != nil {
		t.Fatal(err)
	}
	statement := ast.JSONValue(file).(map[string]any)["statements"].([]any)[0].(map[string]any)
	array := statement["value"].(map[string]any)
	comments, ok := array["comments"].([]any)
	if !ok || len(comments) != 2 {
		t.Fatalf("comments = %#v", array["comments"])
	}
	first := comments[0].(map[string]any)
	if text := first["comment"].(map[string]any)["text"]; text != "# one" {
		t.Errorf("comment 0 text = %#v, want \"# one\"", text)
	}
	if index := first["index"]; index != int64(1) {
		t.Errorf("comment 0 index = %#v, want 1", index)
	}
	if trailing := first["trailing"]; trailing != true {
		t.Errorf("comment 0 trailing = %#v, want true", trailing)
	}
	second := comments[1].(map[string]any)
	if _, present := second["trailing"]; present {
		t.Errorf("trailing should be omitted for an own-line comment: %#v", second)
	}
}

func TestDumpIncludesCollectionComments(t *testing.T) {
	file, err := gdparser.ParseString("var x = [\n\t# c\n\t1,\n]\n")
	if err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	if err := ast.Dump(&builder, file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(builder.String(), "Comment # c") {
		t.Fatalf("dump omitted the comment:\n%s", builder.String())
	}
}
func TestNamedLambdaIsDumpedAndSerialized(t *testing.T) {
	file, err := gdparser.ParseString("func a():\n\tvar f = func named(): pass\n")
	if err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	if err := ast.Dump(&builder, file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(builder.String(), "LambdaExpression named") {
		t.Fatalf("dump omitted the lambda name:\n%s", builder.String())
	}
	statement := ast.JSONValue(file).(map[string]any)["statements"].([]any)[0].(map[string]any)
	lambda := statement["body"].([]any)[0].(map[string]any)["value"].(map[string]any)
	if lambda["kind"] != "LambdaExpression" || lambda["name"] != "named" {
		t.Fatalf("lambda JSON = %#v", lambda)
	}
	span, ok := lambda["name_span"].(map[string]any)
	if !ok {
		t.Fatalf("name span JSON = %#v", lambda["name_span"])
	}
	start := span["start"].(map[string]any)
	if start["line"] != int64(2) || start["column"] != int64(15) {
		t.Errorf("name span start = %#v, want line 2 column 15", start)
	}
}

// An anonymous lambda carries no name, so the field is omitted.
func TestAnonymousLambdaOmitsItsName(t *testing.T) {
	file, err := gdparser.ParseString("func a():\n\tvar f = func(): pass\n")
	if err != nil {
		t.Fatal(err)
	}
	statement := ast.JSONValue(file).(map[string]any)["statements"].([]any)[0].(map[string]any)
	lambda := statement["body"].([]any)[0].(map[string]any)["value"].(map[string]any)
	for _, key := range []string{"name", "name_span"} {
		if _, present := lambda[key]; present {
			t.Errorf("%s should be omitted for an anonymous lambda", key)
		}
	}
}

func TestBindingPatternIsDumpedAndSerialized(t *testing.T) {
	file, err := gdparser.ParseString("func a(x):\n\tmatch x:\n\t\tvar captured:\n\t\t\tpass\n")
	if err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	if err := ast.Dump(&builder, file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(builder.String(), "BindingPattern var captured") {
		t.Fatalf("dump omitted the binding:\n%s", builder.String())
	}
	var found bool
	ast.Inspect(file, func(node ast.Node) bool {
		if binding, ok := node.(*ast.BindingPattern); ok && binding.Name == "captured" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal("traversal did not reach the binding")
	}
	statement := ast.JSONValue(file).(map[string]any)["statements"].([]any)[0].(map[string]any)
	match := statement["body"].([]any)[0].(map[string]any)
	pattern := match["cases"].([]any)[0].(map[string]any)["patterns"].([]any)[0].(map[string]any)
	if pattern["kind"] != "BindingPattern" || pattern["name"] != "captured" {
		t.Fatalf("pattern JSON = %#v", pattern)
	}
}
