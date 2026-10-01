package parser_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/parser"
)

// Godot's parse_statement acts on only two of the standalone annotations, the
// warning-region markers, and answers "Unexpected standalone annotation." for
// every other one. The export group markers reach a class through
// add_member_group, which no function body has, so they are out of place among
// statements even though they stand alone there just as well.
func TestStandaloneAnnotationAmongStatements(t *testing.T) {
	for _, test := range []struct{ name, source, rule string }{
		{
			"an export category in a function body",
			"func f():\n\t@export_category(\"A\")\n\tpass\n",
			`"@export_category" annotation is not allowed inside a function body`,
		},
		{
			"an export group in a function body",
			"func f():\n\t@export_group(\"A\")\n\tpass\n",
			`"@export_group" annotation is not allowed inside a function body`,
		},
		{
			"an export subgroup in a function body",
			"func f():\n\t@export_subgroup(\"A\")\n\tpass\n",
			`"@export_subgroup" annotation is not allowed inside a function body`,
		},
		{
			// A block written on one line reads its body where it stands, so the
			// marker is rejected there as well.
			"an export category as a one-line block body",
			"func f(x):\n\tif x: @export_category(\"A\")\n\telse: pass\n",
			`"@export_category" annotation is not allowed inside a function body`,
		},
		{
			"an export group inside a lambda body",
			"func f():\n\tvar g = func():\n\t\t@export_group(\"A\")\n\t\tpass\n",
			`"@export_group" annotation is not allowed inside a function body`,
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

// The two warning-region markers are the standalone annotations a statement may
// hold, and every one of the five still stands at class level, where Godot's
// parse_program and parse_class_body take the wider set.
func TestStandaloneAnnotationPositionsGodotAccepts(t *testing.T) {
	for _, source := range []string{
		"func f():\n\t@warning_ignore_start(\"unused_variable\")\n\tvar a = 1\n\t@warning_ignore_restore(\"unused_variable\")\n",
		"func f(x):\n\tif x: @warning_ignore_start(\"unused_variable\")\n\telse: pass\n",
		"func f():\n\tvar g = func():\n\t\t@warning_ignore_start(\"unused_variable\")\n\t\tvar a = 1\n",
		"@export_category(\"A\")\n@export_group(\"B\")\n@export_subgroup(\"C\")\n@warning_ignore_start(\"unused_variable\")\n@export var a := 1\n@warning_ignore_restore(\"unused_variable\")\n",
		"class Inner:\n\t@export_category(\"A\")\n\t@export_group(\"B\")\n\t@export_subgroup(\"C\")\n\t@warning_ignore_start(\"unused_variable\")\n\t@export var a := 1\n\t@warning_ignore_restore(\"unused_variable\")\n",
		"class Inner: @export_subgroup(\"C\")\nvar v = 1\n",
	} {
		if _, err := parser.Parse("annotation.gd", []byte(source)); err != nil {
			t.Errorf("parse %q: %v", source, err)
		}
	}
}
