package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTreeFromStandardInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := run(nil, strings.NewReader("var answer: int = 42\n"), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "VariableDeclaration var answer") || !strings.Contains(stdout.String(), "Literal 42") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := run([]string{"-format", "json"}, strings.NewReader("pass\n"), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kind": "KeywordStatement"`) {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunRejectsUnknownFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := run([]string{"-format", "yaml"}, strings.NewReader("pass\n"), &stdout, &stderr)
	if status != 2 || !strings.Contains(stderr.String(), "unknown format") {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
}

func TestRunRejectsUnknownInputType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := run([]string{"-type", "scene"}, strings.NewReader("pass\n"), &stdout, &stderr)
	if status != 2 || !strings.Contains(stderr.String(), "unknown input type") {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
}

func TestRunTextResourceFromStandardInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	source := "[gd_resource type=\"Resource\" format=3]\n\n[resource]\nvalue = Vector2(1, 2)\n"
	status := run([]string{"-type", "resource", "-format", "source"}, strings.NewReader(source), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "[gd_resource") || !strings.Contains(stdout.String(), "Vector2(1, 2)") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunShaderJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	source := "shader_type canvas_item;\nvoid fragment() { COLOR = vec4(1.0); }\n"
	status := run([]string{"-type", "shader", "-format", "json"}, strings.NewReader(source), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kind": "ShaderType"`) || !strings.Contains(stdout.String(), `"kind": "FunctionDeclaration"`) {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunDetectsConfigFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "plugin.cfg")
	if err := os.WriteFile(path, []byte("config_version=5\n\n[application]\nconfig/name=\"Demo\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := run([]string{"-format", "tree", path}, strings.NewReader(""), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Section application") || !strings.Contains(stdout.String(), "Assignment config/name") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunDetectsUIDFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "player.gd.uid")
	if err := os.WriteFile(path, []byte("uid://c3m2k2i8we5da\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := run([]string{"-format", "json", path}, strings.NewReader(""), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kind": "UID"`) || !strings.Contains(stdout.String(), `"value": "uid://c3m2k2i8we5da"`) {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestInferInputType(t *testing.T) {
	tests := map[string]string{
		"player.gd":          "gdscript",
		"level.tscn":         "resource",
		"theme.tres":         "resource",
		"import.escn":        "resource",
		"project.godot":      "config",
		"export_presets.cfg": "config",
		"plugin.cfg":         "config",
		"native.gdextension": "config",
		"icon.svg.import":    "config",
		"main.tscn.remap":    "config",
		"player.gd.uid":      "uid",
		"water.gdshader":     "shader",
		"common.gdshaderinc": "shader",
		"-":                  "gdscript",
	}
	for path, want := range tests {
		if got := inferInputType(path); got != want {
			t.Errorf("inferInputType(%q) = %q, want %q", path, got, want)
		}
	}
}
