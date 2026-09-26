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
