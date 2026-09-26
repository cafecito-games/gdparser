package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

const representativeScript = `@tool
class_name Player
extends CharacterBody2D

signal health_changed(value: int)
enum State { IDLE, RUNNING = 3 }
const MAX_HEALTH: int = 100
@export var speed := 5.0

func damage(amount: int = 1) -> bool:
	if amount <= 0:
		return false
	health -= amount
	return health == 0

func label(value):
	match value:
		0:
			return "zero"
		1, 2:
			return "small"
		_:
			return "other"
`

func TestParseRepresentativeScript(t *testing.T) {
	file, err := parser.Parse("player.gd", []byte(representativeScript))
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "player.gd" {
		t.Fatalf("name = %q", file.Name)
	}
	if len(file.Statements) != 10 {
		t.Fatalf("got %d top-level statements", len(file.Statements))
	}
	function, ok := file.Statements[8].(*ast.FunctionDeclaration)
	if !ok {
		t.Fatalf("statement 8 = %T", file.Statements[8])
	}
	if function.Name != "damage" || function.ReturnType != "bool" || len(function.Body) != 3 {
		t.Fatalf("unexpected function: %#v", function)
	}
	conditional, ok := function.Body[0].(*ast.IfStatement)
	if !ok || len(conditional.Branches) != 1 {
		t.Fatalf("unexpected if: %#v", function.Body[0])
	}
}

func TestFormatRoundTrip(t *testing.T) {
	first, err := parser.Parse("test.gd", []byte(representativeScript))
	if err != nil {
		t.Fatal(err)
	}
	formatted := gdformat.File(first)
	second, err := parser.Parse("test.gd", []byte(formatted))
	if err != nil {
		t.Fatalf("formatted output did not parse: %v\n%s", err, formatted)
	}
	if again := gdformat.File(second); again != formatted {
		t.Fatalf("format is not idempotent:\n--- first ---\n%s--- second ---\n%s", formatted, again)
	}
}

func TestExpressionPrecedence(t *testing.T) {
	source := "var result = 2 ** 3 ** 4 + 5 * (6 - 1)\nvar flag = not result == 0 and true\n"
	file, err := parser.Parse("", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	formatted := gdformat.File(file)
	for _, want := range []string{"2 ** 3 ** 4 + 5 * (6 - 1)", "not result == 0 and true"} {
		if !strings.Contains(formatted, want) {
			t.Errorf("formatted source missing %q:\n%s", want, formatted)
		}
	}
}

func TestCommentsBeforeFirstBlockStatement(t *testing.T) {
	file, err := parser.Parse("", []byte("func ready():\n\t# Kept in the AST.\n\tpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	function := file.Statements[0].(*ast.FunctionDeclaration)
	if len(function.Body) != 2 {
		t.Fatalf("body length = %d", len(function.Body))
	}
	if _, ok := function.Body[0].(*ast.Comment); !ok {
		t.Fatalf("first body node = %T", function.Body[0])
	}
}

func TestSyntaxErrorIncludesFilenameAndPosition(t *testing.T) {
	_, err := parser.Parse("broken.gd", []byte("func nope(\n"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "broken.gd:2:1") {
		t.Fatalf("error = %q", err)
	}
}

func TestModernGodotSyntax(t *testing.T) {
	source := `@abstract
class_name Example extends RefCounted

enum State {
	## Waiting state.
	IDLE,
	RUNNING,
}

static var enabled: bool = true
var title: StringName = &"title"
var property: int = 1:
	get():
		return property
	set(value): property = value; changed.emit()

signal changed

func transform(values: Array[Dictionary]) -> Array:
	var callbacks: Array[Callable] = [
		func(value: int) -> int:
			if value > 0:
				return value
			return 0,
	]
	for value: Dictionary in values:
		if value is not Dictionary:
			continue
		callbacks.append(func() -> void: print(^"position:x"))
	return callbacks

@abstract func execute(value: Variant) -> void
`
	file, err := parser.Parse("modern.gd", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	formatted := gdformat.File(file)
	if _, err := parser.Parse("modern.gd", []byte(formatted)); err != nil {
		t.Fatalf("formatted modern syntax did not parse: %v\n%s", err, formatted)
	}
}

func TestTopLevelCommentAfterFunctionIsNotInFunctionBody(t *testing.T) {
	file, err := parser.Parse("", []byte("func first():\n\tpass\n\n## Documents second.\nfunc second():\n\tpass\n"))
	if err != nil {
		t.Fatal(err)
	}
	first := file.Statements[0].(*ast.FunctionDeclaration)
	if len(first.Body) != 1 {
		t.Fatalf("first function body contains %d nodes, want 1", len(first.Body))
	}
	if _, ok := file.Statements[1].(*ast.Comment); !ok {
		t.Fatalf("top-level statement 1 = %T, want *ast.Comment", file.Statements[1])
	}
}
