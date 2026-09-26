package configfile_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/configfile"
	"github.com/cafecito-games/gdparser/configfile/ast"
	configparser "github.com/cafecito-games/gdparser/configfile/parser"
)

const representativeConfig = `; Engine configuration file.
# Hash comments are accepted as well.
config_version=5

[application] ; inline section comment
config/name="Example"
config/features=PackedStringArray("4.4", "GL Compatibility")
display/title=&"Main Window"
startup/path=^"Root/Startup"
description="""first line
second line"""

[input]
move_left={
"deadzone": 0.5,
"events": [Object(InputEventKey, "resource_local_to_scene": false, "device": -1, "physical_keycode": 65),],
}
limits=[null, true, false, -inf, nan, SOME_SETTING]
`

func TestParseFormatReparseRepresentativeProject(t *testing.T) {
	file, err := configfile.ParseFile("project.godot", []byte(representativeConfig))
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "project.godot" {
		t.Fatalf("file name = %q", file.Name)
	}
	if got := len(file.Sections); got != 2 {
		t.Fatalf("sections = %d", got)
	}
	if file.Sections[0].Name != "application" || file.Sections[1].Name != "input" {
		t.Fatalf("section order = %q, %q", file.Sections[0].Name, file.Sections[1].Name)
	}
	if got := len(file.Preamble); got != 3 {
		t.Fatalf("preamble statements = %d", got)
	}
	inputAssignment := file.Sections[1].Statements[0].(*ast.Assignment)
	dictionary, ok := inputAssignment.Value.(*ast.DictionaryLiteral)
	if !ok || len(dictionary.Items) != 2 {
		t.Fatalf("input value = %#v", inputAssignment.Value)
	}
	eventsEntry := dictionary.Items[1].(*ast.DictionaryEntry)
	events := eventsEntry.Value.(*ast.ArrayLiteral)
	object := events.Items[0].(*ast.ArrayElement).Value.(*ast.ConstructorCall)
	secondArgument := object.Items[1].(*ast.ConstructorArgument)
	if object.Name != "Object" || len(object.Items) != 4 || secondArgument.Key == nil {
		t.Fatalf("Object constructor = %#v", object)
	}

	formatted := configfile.Format(file)
	for _, want := range []string{
		"config_version=5\n",
		"[application]\n",
		`display/title=&"Main Window"`,
		`startup/path=^"Root/Startup"`,
		`move_left={"deadzone": 0.5, "events": [Object(InputEventKey, "resource_local_to_scene": false, "device": -1, "physical_keycode": 65)]}`,
	} {
		if !strings.Contains(formatted, want) {
			t.Errorf("formatted output missing %q:\n%s", want, formatted)
		}
	}
	reparsed, err := configfile.ParseString(formatted)
	if err != nil {
		t.Fatalf("reparse canonical output: %v\n%s", err, formatted)
	}
	if len(reparsed.Sections) != len(file.Sections) {
		t.Fatalf("reparsed sections = %d", len(reparsed.Sections))
	}
	if again := configfile.Format(reparsed); again != formatted {
		t.Fatalf("format is not idempotent\nfirst:\n%s\nsecond:\n%s", formatted, again)
	}
}

func TestSpansAreByteBasedAndOneBased(t *testing.T) {
	source := "# café\n[rénder]\nwindow/size=Vector2i(1280, 720)\n"
	file, err := configfile.ParseString(source)
	if err != nil {
		t.Fatal(err)
	}
	section := file.Sections[0]
	if got := section.Span().Start; got.Offset != len("# café\n") || got.Line != 2 || got.Column != 1 {
		t.Fatalf("section start = %#v", got)
	}
	assignment := section.Statements[0].(*ast.Assignment)
	if got := assignment.Span().Start; got.Line != 3 || got.Column != 1 {
		t.Fatalf("assignment start = %#v", got)
	}
	if section.Span().End != assignment.Span().End {
		t.Fatalf("section end = %#v, assignment end = %#v", section.Span().End, assignment.Span().End)
	}
	call := assignment.Value.(*ast.ConstructorCall)
	if got := call.Span().Start.Column; got != 13 {
		t.Fatalf("constructor column = %d", got)
	}
}

func TestTraversalJSONDumpAndMutation(t *testing.T) {
	file, err := configfile.ParseString("config_version=5\n\n[application]\nconfig/name=\"Old\"\n")
	if err != nil {
		t.Fatal(err)
	}
	visited := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		visited++
		if literal, ok := node.(*ast.StringLiteral); ok {
			literal.Value = "New"
		}
		return true
	})
	if visited != 6 { // File, section, two assignments, and their values.
		t.Fatalf("visited = %d", visited)
	}
	if got := configfile.Format(file); !strings.Contains(got, `config/name="New"`) {
		t.Fatalf("mutation was not formatted:\n%s", got)
	}
	json := ast.JSONValue(file).(map[string]any)
	if json["kind"] != "File" {
		t.Fatalf("JSON file kind = %#v", json["kind"])
	}
	section := json["sections"].([]any)[0].(map[string]any)
	if section["kind"] != "Section" {
		t.Fatalf("JSON section kind = %#v", section["kind"])
	}
	var dump bytes.Buffer
	if err := ast.Dump(&dump, file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump.String(), "Section application\n") || !strings.Contains(dump.String(), "StringLiteral \"New\"") {
		t.Fatalf("dump:\n%s", dump.String())
	}
}

func TestQuotedKeysEscapedSectionsAndStrings(t *testing.T) {
	source := "[section\\]name with spaces]\n\"key with spaces\"=\"line\\nquote: \\\" slash: \\\\ snowman: ☃\"\n"
	file, err := configfile.ParseString(source)
	if err != nil {
		t.Fatal(err)
	}
	if file.Sections[0].Name != "section]name with spaces" {
		t.Fatalf("section name = %q", file.Sections[0].Name)
	}
	assignment := file.Sections[0].Statements[0].(*ast.Assignment)
	if assignment.Key != "key with spaces" {
		t.Fatalf("key = %q", assignment.Key)
	}
	formatted := configfile.Format(file)
	if _, err := configfile.ParseString(formatted); err != nil {
		t.Fatalf("parse escaped canonical output: %v\n%s", err, formatted)
	}
}

func TestPhysicalNewlinesInQuotedStrings(t *testing.T) {
	source := `[preset.0.options]
ssh_remote_deploy/run_script="#!/usr/bin/env bash
unzip -o -q \"{archive_name}\"
open \"{exe_name}.app\""
after="value"
`
	file, err := configfile.ParseFile("export_presets.cfg", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	assignment := file.Sections[0].Statements[0].(*ast.Assignment)
	literal := assignment.Value.(*ast.StringLiteral)
	want := "#!/usr/bin/env bash\nunzip -o -q \"{archive_name}\"\nopen \"{exe_name}.app\""
	if literal.Value != want {
		t.Fatalf("multiline value = %q, want %q", literal.Value, want)
	}
	if got := file.Sections[0].Statements[1].Span().Start; got.Line != 5 || got.Column != 1 {
		t.Fatalf("following assignment starts at %#v", got)
	}

	formatted := configfile.Format(file)
	if strings.Contains(formatted, "#!/usr/bin/env bash\nunzip") {
		t.Fatalf("canonical output should escape physical newlines:\n%s", formatted)
	}
	if !strings.Contains(formatted, `run_script="#!/usr/bin/env bash\nunzip`) {
		t.Fatalf("canonical output missing escaped multiline value:\n%s", formatted)
	}
	reparsed, err := configfile.ParseString(formatted)
	if err != nil {
		t.Fatalf("reparse canonical output: %v\n%s", err, formatted)
	}
	got := reparsed.Sections[0].Statements[0].(*ast.Assignment).Value.(*ast.StringLiteral).Value
	if got != want {
		t.Fatalf("reparsed multiline value = %q, want %q", got, want)
	}
}

func TestTypedContainersAndNestedCommentsRoundTrip(t *testing.T) {
	source := `[advanced]
typed_array=Array[Dictionary[String, Array[int]]]([
; before array value
{"one": [1, 2]},
# after array value
])
typed_dictionary=Dictionary[String, Array[int]]({
; before dictionary entry
"one":
# before dictionary value
[1, 2]
})
constructed=Object(
; before object type
InputEventKey,
"device":
; before property value
-1
# after property
)
`
	file, err := configfile.ParseString(source)
	if err != nil {
		t.Fatal(err)
	}
	arrayAssignment := file.Sections[0].Statements[0].(*ast.Assignment)
	typedArray, ok := arrayAssignment.Value.(*ast.TypedArrayLiteral)
	if !ok {
		t.Fatalf("typed array = %T", arrayAssignment.Value)
	}
	if typedArray.ElementType.Name != "Dictionary" || len(typedArray.ElementType.Arguments) != 2 {
		t.Fatalf("array element type = %#v", typedArray.ElementType)
	}
	nestedArrayType := typedArray.ElementType.Arguments[1]
	if nestedArrayType.Name != "Array" || len(nestedArrayType.Arguments) != 1 || nestedArrayType.Arguments[0].Name != "int" {
		t.Fatalf("nested array type = %#v", nestedArrayType)
	}
	dictionaryAssignment := file.Sections[0].Statements[1].(*ast.Assignment)
	if _, ok := dictionaryAssignment.Value.(*ast.TypedDictionaryLiteral); !ok {
		t.Fatalf("typed dictionary = %T", dictionaryAssignment.Value)
	}

	var comments []string
	ast.Inspect(file, func(node ast.Node) bool {
		if comment, ok := node.(*ast.Comment); ok {
			comments = append(comments, comment.Text)
		}
		return true
	})
	wantComments := []string{
		"; before array value", "# after array value",
		"; before dictionary entry", "# before dictionary value",
		"; before object type", "; before property value", "# after property",
	}
	if strings.Join(comments, "|") != strings.Join(wantComments, "|") {
		t.Fatalf("comments in traversal order = %#v", comments)
	}

	formatted := configfile.Format(file)
	for _, comment := range wantComments {
		if strings.Count(formatted, comment) != 1 {
			t.Errorf("formatted comment %q count != 1:\n%s", comment, formatted)
		}
	}
	reparsed, err := configfile.ParseString(formatted)
	if err != nil {
		t.Fatalf("reparse typed/comment output: %v\n%s", err, formatted)
	}
	if again := configfile.Format(reparsed); again != formatted {
		t.Fatalf("typed/comment format is not idempotent\nfirst:\n%s\nsecond:\n%s", formatted, again)
	}
	comments = nil
	ast.Inspect(reparsed, func(node ast.Node) bool {
		if comment, ok := node.(*ast.Comment); ok {
			comments = append(comments, comment.Text)
		}
		return true
	})
	if strings.Join(comments, "|") != strings.Join(wantComments, "|") {
		t.Fatalf("reparsed comments in traversal order = %#v", comments)
	}
}

func TestPositionedErrorsDoNotPanic(t *testing.T) {
	badSources := []string{
		"[application\n",
		"key={\"missing\" 1}\n",
		"key=[1, 2\n",
		"key=\"unterminated\n",
		"key=1e\n",
		"=1\n",
	}
	for _, source := range badSources {
		t.Run(strings.ReplaceAll(source, "\n", `\n`), func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("panic: %v", recovered)
				}
			}()
			_, err := configfile.ParseFile("broken.godot", []byte(source))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "broken.godot:") {
				t.Fatalf("error lacks filename and position: %v", err)
			}
			var parseError *configparser.Error
			if errors.As(err, &parseError) && parseError.Token.Span.Start.Line < 1 {
				t.Fatalf("invalid error position: %#v", parseError.Token.Span.Start)
			}
		})
	}
}
