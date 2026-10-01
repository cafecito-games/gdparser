package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/parser"
)

// Godot keeps a registry of the annotations it knows, built by the
// register_annotation calls in parse_annotation's setup, and refuses a name that
// is not in it. The three names it retired answer with the documentation comment
// that replaced them.
func TestAnnotationMustExist(t *testing.T) {
	for _, test := range []struct{ source, rule string }{
		{"@hello_world\nvar a = 1\n", `unrecognized annotation "@hello_world"`},
		{"@deprecated\nvar a = 1\n", `"## @deprecated: Reason here."`},
		{"@experimental\nvar a = 1\n", `"## @experimental: Reason here."`},
		{"@tutorial\nvar a = 1\n", `"## @tutorial(Title): https://example.com"`},
	} {
		_, err := parser.Parse("annotation.gd", []byte(test.source))
		if err == nil {
			t.Errorf("%q was accepted", test.source)
			continue
		}
		if !strings.Contains(err.Error(), test.rule) {
			t.Errorf("%q: error does not name the rule: %v", test.source, err)
		}
	}
}

// An annotation may only be written against what it is registered for, which
// Godot checks twice: once for the level it stands at, and once for the kind of
// declaration it decorates.
func TestAnnotationMustSuitWhatItDecorates(t *testing.T) {
	for _, test := range []struct{ name, source, rule string }{
		{
			"an export on a function",
			"@export\nfunc f():\n\tpass\n",
			`"@export" annotation is not allowed at this level`,
		},
		{
			"an rpc on a variable",
			"@rpc\nvar a = 1\n",
			`"@rpc" annotation is not allowed at this level`,
		},
		{
			"an onready on a signal",
			"@onready\nsignal done\n",
			`"@onready" annotation is not allowed at this level`,
		},
		{
			// An annotation of the script itself belongs before class_name and
			// extends, which close the head of the script.
			"an icon after class_name",
			"class_name A\n@icon(\"res://icon.svg\")\nvar a = 1\n",
			`"@icon" annotation belongs at the top of the script`,
		},
		{
			"a tool after extends",
			"extends Node\n@tool\nvar a = 1\n",
			`"@tool" annotation belongs at the top of the script`,
		},
		{
			"an export inside a function",
			"func f():\n\t@export\n\tvar a = 1\n",
			`"@export" annotation is not allowed at this level`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parser.Parse("annotation.gd", []byte(test.source))
			if err == nil {
				t.Fatal("expected a parse error")
			}
			if !strings.Contains(err.Error(), test.rule) {
				t.Fatalf("error does not name the rule: %v", err)
			}
		})
	}
}

// The positions Godot does accept, including the prologue where a string stands
// in for a comment and an annotation of the script may still come.
func TestAnnotationPositionsGodotAccepts(t *testing.T) {
	for _, source := range []string{
		"@tool\n@icon(\"res://icon.svg\")\n@static_unload\nextends Node\nclass_name A\n",
		// A string written on its own is a comment, so the head stays open.
		"\"\"\"a comment\"\"\"\n@tool\nextends Node\n",
		"@export var a := 1\n@onready var b := 1\n",
		"@export_range(0, 10) var c := 1\n",
		"@rpc(\"any_peer\")\nfunc f():\n\tpass\n",
		"@warning_ignore(\"unused_signal\")\nsignal done\n",
		"func f(v):\n\t@warning_ignore(\"unsafe_cast\")\n\tvar a = v as int\n\tprint(a)\n",
		// @abstract reaches the script, a class and a function alike.
		"@abstract\nclass_name A\n",
		"@abstract\nclass Inner:\n\tpass\n",
		"@abstract func f() -> int\n",
		// The group markers decorate nothing and stand on their own.
		"@export_category(\"Move\")\n@export_group(\"Speed\")\n@export_subgroup(\"Fine\")\n@export var a := 1\n",
		"@warning_ignore_start(\"unused_variable\")\nvar a = 1\n@warning_ignore_restore(\"unused_variable\")\n",
	} {
		if _, err := parser.Parse("annotation.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}
