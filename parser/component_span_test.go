package parser_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
	"github.com/cafecito-games/gdparser/token"
)

func TestDeclarationComponentSpans(t *testing.T) {
	source := []byte(`@export_category("café")
class_name Niño extends Base.Inner
static var value: Array[Dictionary[String, int]] = source.member:
	get:
		return value
	set(next_value):
		value = next_value
static func transform(input: Dictionary[String, Array[int]] = defaults) -> Array[Vector2]:
	var callback = func(item: Array[int] = defaults) -> Dictionary[String, int]: return item
class Inner extends Base.Generic:
	signal changed(payload: int)
`)
	file := parseSpans(t, source)

	annotation := file.Statements[0].(*ast.Annotation)
	assertSpan(t, source, annotation.NameSpan, "export_category")

	directive := file.Statements[1].(*ast.Directive)
	assertSpan(t, source, directive.KeywordSpan, "class_name")
	assertSpan(t, source, directive.ExtendsSpan, "extends")

	variable := file.Statements[2].(*ast.VariableDeclaration)
	assertSpan(t, source, variable.StaticSpan, "static")
	assertSpan(t, source, variable.KeywordSpan, "var")
	assertSpan(t, source, variable.NameSpan, "value")
	assertSpan(t, source, variable.TypeSpan, "Array[Dictionary[String, int]]")
	assertSpan(t, source, variable.OperatorSpan, "=")
	assertSpan(t, source, variable.GetterKeywordSpan, "get")
	assertSpan(t, source, variable.GetterSpan, "get:\n\t\treturn value")
	assertSpan(t, source, variable.Setter.KeywordSpan, "set")
	assertSpan(t, source, variable.Setter.ParameterSpan, "next_value")
	assertSpan(t, source, variable.Setter.Span(), "set(next_value):\n\t\tvalue = next_value")
	getterReturn := variable.Getter[0].(*ast.ReturnStatement)
	assertSpan(t, source, getterReturn.KeywordSpan, "return")
	setterAssignment := variable.Setter.Body[0].(*ast.Assignment)
	assertSpan(t, source, setterAssignment.OperatorSpan, "=")

	function := file.Statements[3].(*ast.FunctionDeclaration)
	assertSpan(t, source, function.StaticSpan, "static")
	assertSpan(t, source, function.KeywordSpan, "func")
	assertSpan(t, source, function.NameSpan, "transform")
	assertSpan(t, source, function.ReturnArrowSpan, "->")
	assertSpan(t, source, function.ReturnTypeSpan, "Array[Vector2]")
	parameter := function.Parameters[0]
	assertSpan(t, source, parameter.Span(), "input: Dictionary[String, Array[int]] = defaults")
	assertSpan(t, source, parameter.NameSpan, "input")
	assertSpan(t, source, parameter.TypeSpan, "Dictionary[String, Array[int]]")
	assertSpan(t, source, parameter.DefaultOperatorSpan, "=")

	lambdaVariable := function.Body[0].(*ast.VariableDeclaration)
	lambda := lambdaVariable.Value.(*ast.LambdaExpression)
	assertSpan(t, source, lambda.KeywordSpan, "func")
	assertSpan(t, source, lambda.ReturnArrowSpan, "->")
	assertSpan(t, source, lambda.ReturnTypeSpan, "Dictionary[String, int]")
	lambdaParameter := lambda.Parameters[0]
	assertSpan(t, source, lambdaParameter.Span(), "item: Array[int] = defaults")
	assertSpan(t, source, lambdaParameter.NameSpan, "item")
	assertSpan(t, source, lambdaParameter.TypeSpan, "Array[int]")
	assertSpan(t, source, lambdaParameter.DefaultOperatorSpan, "=")

	class := file.Statements[4].(*ast.ClassDeclaration)
	assertSpan(t, source, class.KeywordSpan, "class")
	assertSpan(t, source, class.NameSpan, "Inner")
	assertSpan(t, source, class.ExtendsSpan, "extends")
	assertSpan(t, source, class.BaseTypeSpan, "Base.Generic")
	signal := class.Body[0].(*ast.SignalDeclaration)
	assertSpan(t, source, signal.KeywordSpan, "signal")
	assertSpan(t, source, signal.NameSpan, "changed")
	assertSpan(t, source, signal.Parameters[0].Span(), "payload: int")
}

func TestEnumComponentSpans(t *testing.T) {
	source := []byte(`enum State {
	## Waiting.
	IDLE,
	RUNNING = 3,
}
enum { SOLO }
`)
	file := parseSpans(t, source)
	named := file.Statements[0].(*ast.EnumDeclaration)
	assertSpan(t, source, named.KeywordSpan, "enum")
	assertSpan(t, source, named.NameSpan, "State")
	assertSpan(t, source, named.Members[0].Span(), "IDLE")
	assertSpan(t, source, named.Members[0].NameSpan, "IDLE")
	assertZeroSpan(t, named.Members[0].OperatorSpan)
	assertSpan(t, source, named.Members[1].Span(), "RUNNING = 3")
	assertSpan(t, source, named.Members[1].NameSpan, "RUNNING")
	assertSpan(t, source, named.Members[1].OperatorSpan, "=")

	anonymous := file.Statements[1].(*ast.EnumDeclaration)
	assertSpan(t, source, anonymous.KeywordSpan, "enum")
	assertZeroSpan(t, anonymous.NameSpan)
	assertSpan(t, source, anonymous.Members[0].Span(), "SOLO")
}

func TestControlFlowComponentSpans(t *testing.T) {
	source := []byte(`func flow(items: Array[int]):
	if first:
		pass
	elif second:
		breakpoint
	elif third:
		pass
	else:
		pass
	while running:
		break
	for item: Dictionary[String, int] in items:
		continue
	match item:
		1 when ready:
			return item
		_:
			pass
`)
	file := parseSpans(t, source)
	function := file.Statements[0].(*ast.FunctionDeclaration)

	conditional := function.Body[0].(*ast.IfStatement)
	branchWants := []struct {
		keyword string
		overall string
	}{
		{"if", "if first:\n\t\tpass"},
		{"elif", "elif second:\n\t\tbreakpoint"},
		{"elif", "elif third:\n\t\tpass"},
	}
	for i, want := range branchWants {
		assertSpan(t, source, conditional.Branches[i].KeywordSpan, want.keyword)
		assertSpan(t, source, conditional.Branches[i].Span(), want.overall)
	}
	assertSpan(t, source, conditional.ElseKeywordSpan, "else")
	assertSpan(t, source, conditional.ElseSpan, "else:\n\t\tpass")
	assertSpan(t, source, conditional.Branches[1].Body[0].(*ast.KeywordStatement).KeywordSpan, "breakpoint")

	while := function.Body[1].(*ast.WhileStatement)
	assertSpan(t, source, while.KeywordSpan, "while")
	loop := function.Body[2].(*ast.ForStatement)
	assertSpan(t, source, loop.KeywordSpan, "for")
	assertSpan(t, source, loop.VariableSpan, "item")
	assertSpan(t, source, loop.TypeSpan, "Dictionary[String, int]")
	assertSpan(t, source, loop.InSpan, "in")

	match := function.Body[3].(*ast.MatchStatement)
	assertSpan(t, source, match.KeywordSpan, "match")
	assertSpan(t, source, match.Cases[0].Span(), "1 when ready:\n\t\t\treturn item")
	assertSpan(t, source, match.Cases[0].WhenSpan, "when")
	assertZeroSpan(t, match.Cases[1].WhenSpan)
	assertSpan(t, source, match.Cases[1].Span(), "_:\n\t\t\tpass")
	assertSpan(t, source, match.Cases[0].Body[0].(*ast.ReturnStatement).KeywordSpan, "return")
}

func TestExpressionComponentSpans(t *testing.T) {
	source := []byte(`func expressions():
	var unary = -source.member
	var membership = left not  in right
	var type_check = left is   not Right
	var binary = one + two
	var ternary = yes if cond else no
	target.member += value
`)
	file := parseSpans(t, source)
	body := file.Statements[0].(*ast.FunctionDeclaration).Body

	unary := body[0].(*ast.VariableDeclaration).Value.(*ast.UnaryExpression)
	assertSpan(t, source, unary.OperatorSpan, "-")
	member := unary.Operand.(*ast.MemberExpression)
	assertSpan(t, source, member.PropertySpan, "member")

	membership := body[1].(*ast.VariableDeclaration).Value.(*ast.BinaryExpression)
	assertSpan(t, source, membership.OperatorSpan, "not  in")
	typeCheck := body[2].(*ast.VariableDeclaration).Value.(*ast.BinaryExpression)
	assertSpan(t, source, typeCheck.OperatorSpan, "is   not")
	binary := body[3].(*ast.VariableDeclaration).Value.(*ast.BinaryExpression)
	assertSpan(t, source, binary.OperatorSpan, "+")
	ternary := body[4].(*ast.VariableDeclaration).Value.(*ast.TernaryExpression)
	assertSpan(t, source, ternary.IfSpan, "if")
	assertSpan(t, source, ternary.ElseSpan, "else")
	assignment := body[5].(*ast.Assignment)
	assertSpan(t, source, assignment.OperatorSpan, "+=")
	assertSpan(t, source, assignment.Target.(*ast.MemberExpression).PropertySpan, "member")
}

func TestRepeatedUnicodeAndMultilineComponentSpans(t *testing.T) {
	source := []byte("func café(\n\trepeated:\n\t\tArray[\n\t\t\tDictionary[String, int]\n\t\t] = repeated\n\t):\n\tvar repeated = repeated.repeated\n")
	file := parseSpans(t, source)
	function := file.Statements[0].(*ast.FunctionDeclaration)
	assertSpan(t, source, function.NameSpan, "café")
	parameter := function.Parameters[0]
	assertSpan(t, source, parameter.NameSpan, "repeated")
	assertSpan(t, source, parameter.TypeSpan, "Array[\n\t\t\tDictionary[String, int]\n\t\t]")
	assertSpan(t, source, parameter.DefaultOperatorSpan, "=")
	declaration := function.Body[0].(*ast.VariableDeclaration)
	assertSpan(t, source, declaration.NameSpan, "repeated")
	member := declaration.Value.(*ast.MemberExpression)
	assertSpan(t, source, member.PropertySpan, "repeated")
	if declaration.NameSpan.Start.Offset == parameter.NameSpan.Start.Offset || declaration.NameSpan.Start.Offset == member.PropertySpan.Start.Offset {
		t.Fatal("repeated-name spans reused an earlier occurrence")
	}
}

func TestAdjacentScalarAndOptionalComponentSpans(t *testing.T) {
	source := []byte("extends Node\nvar property: int:\n\tset:\n\t\tvar path = %Root/Child\n")
	file := parseSpans(t, source)
	directive := file.Statements[0].(*ast.Directive)
	assertSpan(t, source, directive.KeywordSpan, "extends")
	assertZeroSpan(t, directive.ExtendsSpan)

	declaration := file.Statements[1].(*ast.VariableDeclaration)
	assertSpan(t, source, declaration.Setter.KeywordSpan, "set")
	assertZeroSpan(t, declaration.Setter.ParameterSpan)
	nodePath := declaration.Setter.Body[0].(*ast.VariableDeclaration).Value.(*ast.NodePathExpression)
	assertSpan(t, source, nodePath.PrefixSpan, "%")
	assertSpan(t, source, nodePath.PathSpan, "Root/Child")
}

func TestComponentSpanJSONTraversalFormattingAndManualAST(t *testing.T) {
	source := []byte("func read(arg: int = 1): return obj.member\n")
	file := parseSpans(t, source)
	value := ast.JSONValue(file).(map[string]any)
	functionJSON := value["statements"].([]any)[0].(map[string]any)
	for _, field := range []string{"keyword_span", "name_span"} {
		if _, ok := functionJSON[field]; !ok {
			t.Errorf("function JSON missing %q: %#v", field, functionJSON)
		}
	}
	if _, ok := functionJSON["static_span"]; ok {
		t.Errorf("function JSON includes absent static_span: %#v", functionJSON)
	}
	parameterJSON := functionJSON["parameters"].([]any)[0].(map[string]any)
	for _, field := range []string{"span", "name_span", "type_span", "default_operator_span"} {
		if _, ok := parameterJSON[field]; !ok {
			t.Errorf("parameter JSON missing %q: %#v", field, parameterJSON)
		}
	}

	var kinds []string
	ast.Inspect(file, func(node ast.Node) bool {
		if node != nil {
			kinds = append(kinds, reflect.TypeOf(node).Elem().Name())
		}
		return true
	})
	wantKinds := []string{"File", "FunctionDeclaration", "Literal", "ReturnStatement", "MemberExpression", "Identifier"}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Fatalf("traversal kinds = %v, want %v", kinds, wantKinds)
	}
	var dump bytes.Buffer
	if err := ast.Dump(&dump, file); err != nil {
		t.Fatal(err)
	}
	wantDump := "File \"spans.gd\"\n  FunctionDeclaration read\n    Literal 1\n    ReturnStatement\n      MemberExpression .member\n        Identifier obj\n"
	if dump.String() != wantDump {
		t.Fatalf("tree dump =\n%s\nwant:\n%s", dump.String(), wantDump)
	}

	formatted := gdformat.File(file)
	reparsed := parseSpans(t, []byte(formatted))
	reparsedFunction := reparsed.Statements[0].(*ast.FunctionDeclaration)
	assertSpan(t, []byte(formatted), reparsedFunction.Parameters[0].TypeSpan, "int")
	assertSpan(t, []byte(formatted), reparsedFunction.Body[0].(*ast.ReturnStatement).KeywordSpan, "return")

	manual := &ast.File{Statements: []ast.Statement{&ast.FunctionDeclaration{
		Name: "manual", Parameters: []ast.Parameter{{Name: "value", Type: "int"}},
		Body: []ast.Statement{&ast.ReturnStatement{Value: &ast.Identifier{Name: "value"}}},
	}}}
	if got, want := gdformat.File(manual), "func manual(value: int):\n\treturn value\n"; got != want {
		t.Fatalf("manual zero-span format = %q, want %q", got, want)
	}
	manualJSON := ast.JSONValue(manual).(map[string]any)
	manualFunction := manualJSON["statements"].([]any)[0].(map[string]any)
	if _, ok := manualFunction["keyword_span"]; ok {
		t.Fatalf("manual zero span was not omitted: %#v", manualFunction)
	}
}

func parseSpans(t *testing.T, source []byte) *ast.File {
	t.Helper()
	file, err := parser.Parse("spans.gd", source)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func assertZeroSpan(t *testing.T, span token.Span) {
	t.Helper()
	if span != (token.Span{}) {
		t.Fatalf("span = %#v, want zero", span)
	}
}

func assertSpan(t *testing.T, source []byte, span token.Span, want string) {
	t.Helper()
	if span == (token.Span{}) {
		t.Fatalf("span for %q is zero", want)
	}
	if span.Start.Offset < 0 || span.End.Offset < span.Start.Offset || span.End.Offset > len(source) {
		t.Fatalf("span for %q has invalid offsets: %#v (source length %d)", want, span, len(source))
	}
	if got := string(source[span.Start.Offset:span.End.Offset]); got != want {
		t.Fatalf("span slice = %q, want %q (span %#v)", got, want, span)
	}
	if got := sourcePosition(source, span.Start.Offset); got != span.Start {
		t.Fatalf("span start for %q = %#v, want %#v", want, span.Start, got)
	}
	if got := sourcePosition(source, span.End.Offset); got != span.End {
		t.Fatalf("span end for %q = %#v, want %#v", want, span.End, got)
	}
}

func sourcePosition(source []byte, offset int) token.Position {
	position := token.Position{Line: 1, Column: 1}
	for i, b := range source[:offset] {
		position.Offset = i + 1
		if b == '\n' {
			position.Line++
			position.Column = 1
		} else {
			position.Column++
		}
	}
	return position
}

func TestComponentSpansAreContainedByOwners(t *testing.T) {
	source := []byte("static var item: Array[int] := source.item\n")
	file := parseSpans(t, source)
	declaration := file.Statements[0].(*ast.VariableDeclaration)
	for name, span := range map[string]token.Span{
		"static":   declaration.StaticSpan,
		"keyword":  declaration.KeywordSpan,
		"name":     declaration.NameSpan,
		"type":     declaration.TypeSpan,
		"operator": declaration.OperatorSpan,
	} {
		if span.Start.Offset < declaration.Span().Start.Offset || span.End.Offset > declaration.Span().End.Offset {
			t.Errorf("%s span %#v is outside declaration %#v", name, span, declaration.Span())
		}
	}
	assertSpan(t, source, declaration.OperatorSpan, ":=")
	if !declaration.Inferred {
		t.Fatal(":= declaration did not preserve inferred semantics")
	}

	withoutValue := parseSpans(t, []byte("var item\n")).Statements[0].(*ast.VariableDeclaration)
	assertZeroSpan(t, withoutValue.TypeSpan)
	assertZeroSpan(t, withoutValue.OperatorSpan)
	if withoutValue.Value != nil {
		t.Fatalf("value = %#v, want nil", withoutValue.Value)
	}
	if strings.TrimSpace(gdformat.File(&ast.File{Statements: []ast.Statement{withoutValue}})) != "var item" {
		t.Fatal("optional metadata changed formatting")
	}
}
