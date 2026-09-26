package textresource

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSceneParseFormatRoundTrip(t *testing.T) {
	source := `[gd_scene load_steps=3 format=3 uid="uid://abc"]

; external script
[ext_resource type="Script" path="res://player.gd" id="1_script"]

[sub_resource type="Curve" id="Curve_speed"]
_data = {"limits": [-1.25e-3, 4.0], &"enabled": true}

[node name="Player" type="Node2D"] # root node
script = ExtResource("1_script")
theme_override/font_sizes/font_size = 14
target = ^"Camera/Target"
labels = Array[StringName]([&"one", &"two"])
`
	document, err := ParseFile("player.tscn", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Items) != 4 {
		t.Fatalf("got %d document items", len(document.Items))
	}
	formatted := Format(document)
	reparsed, err := ParseString(formatted)
	if err != nil {
		t.Fatalf("formatted output did not parse:\n%s\nerror: %v", formatted, err)
	}
	if second := Format(reparsed); second != formatted {
		t.Fatalf("format is not idempotent:\nfirst:\n%s\nsecond:\n%s", formatted, second)
	}
	var stringNames, nodePaths int
	Inspect(reparsed, func(n Node) bool {
		if value, ok := n.(*StringValue); ok {
			if value.Kind == StringName {
				stringNames++
			}
			if value.Kind == NodePath {
				nodePaths++
			}
		}
		return true
	})
	if stringNames != 3 || nodePaths != 1 {
		t.Fatalf("StringName=%d NodePath=%d", stringNames, nodePaths)
	}
}

func TestResourceTypedDictionaryAndComments(t *testing.T) {
	source := `[gd_resource type="Resource" script_class="Rates" load_steps=2 format=3]

[ext_resource type="Script" path="res://rates.gd" id="1"]

[resource]
script = ExtResource("1")
rates = Dictionary[String, int]({
; preserved in dictionary scope
"first": 1000000,
"second": -2,
})
fallbacks = Array[ExtResource("1")]([])
`
	document, err := ParseString(source)
	if err != nil {
		t.Fatal(err)
	}
	formatted := Format(document)
	if _, err := ParseString(formatted); err != nil {
		t.Fatalf("round trip failed: %v\n%s", err, formatted)
	}
	encoded, err := json.Marshal(JSONValue(document))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"kind":"TypedDictionaryValue"`) {
		t.Fatalf("JSON lacks node discriminator: %s", encoded)
	}
}

func TestPositionedError(t *testing.T) {
	_, err := ParseFile("broken.tres", []byte("[gd_resource format=3]\n[resource]\nvalue = [1, nope(]\n"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "broken.tres:3:") {
		t.Fatalf("error lacks filename and position: %v", err)
	}
}

func TestSpansAndWalk(t *testing.T) {
	document, err := ParseString("[gd_resource format=3]\n[resource]\nvalue = Vector2(1, 2)\n")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	Inspect(document, func(n Node) bool {
		if n == nil {
			return true
		}
		count++
		span := n.Span()
		if span.Start.Line < 1 || span.Start.Column < 1 || span.End.Offset < span.Start.Offset {
			t.Fatalf("bad span on %T: %+v", n, span)
		}
		return true
	})
	if count < 8 {
		t.Fatalf("walk visited only %d nodes", count)
	}
}

func TestCorpus(t *testing.T) {
	root := os.Getenv("GDPARSER_TEXTRESOURCE_CORPUS")
	if root == "" {
		t.Skip("set GDPARSER_TEXTRESOURCE_CORPUS to a Godot project")
	}
	var files int
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".tscn" && ext != ".tres" && ext != ".escn" {
			return nil
		}
		files++
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		document, err := ParseFile(path, source)
		if err != nil {
			return err
		}
		formatted := Format(document)
		if _, err = ParseFile(path, []byte(formatted)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("no text resource files found")
	}
	t.Logf("parsed and reparsed %d files", files)
}
