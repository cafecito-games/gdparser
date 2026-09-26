package shader_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/shader"
	"github.com/cafecito-games/gdparser/shader/ast"
)

const comprehensive = `#define MAX_LIGHTS 4
shader_type spatial;
render_mode blend_mix, depth_draw_opaque;
group_uniforms Lighting.Main;
uniform highp vec4 tint : source_color = vec4(1.0);
uniform vec3 light_source[MAX_LIGHTS];
group_uniforms;
const vec2 OFFSETS[2] = {vec2(-1.0), vec2(1.0)};

struct Surface {
	vec3 normal;
	float roughness;
};

float saturate(in float value) {
	return value < 0.0 ? 0.0 : value > 1.0 ? 1.0 : value;
}

void fragment() {
	vec3 result = tint.rgb;
	for (int i = 0; i < MAX_LIGHTS; i++) {
		if (light_source[i].x > 0.0 && i != 2) {
			result += light_source[i];
		} else if (i == 3) {
			continue;
		}
	}
	int n = 0;
	do { n++; } while (n < 2);
	while (n > 0) { --n; }
	switch (n) {
	case 0:
		break;
	default:
		discard;
	}
	ALBEDO = result * (tint.rgb + vec3(0.25));
}
`

func TestParseFormatRoundTrip(t *testing.T) {
	file, err := shader.ParseFile("lighting.gdshader", []byte(comprehensive))
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "lighting.gdshader" || len(file.Items) < 10 {
		t.Fatalf("unexpected file: %#v", file)
	}
	formatted := shader.Format(file)
	reparsed, err := shader.ParseString(formatted)
	if err != nil {
		t.Fatalf("formatted output did not parse: %v\n%s", err, formatted)
	}
	second := shader.Format(reparsed)
	if second != formatted {
		t.Fatalf("formatter is not idempotent:\n--- first ---\n%s--- second ---\n%s", formatted, second)
	}
	if !strings.Contains(formatted, "uniform vec3 light_source[MAX_LIGHTS];") || !strings.Contains(formatted, "n++;") {
		t.Fatalf("lost array or postfix expression:\n%s", formatted)
	}
}

func TestTraversalDumpAndJSON(t *testing.T) {
	file, err := shader.ParseString("shader_type canvas_item;\nvoid fragment() { COLOR = vec4(1.0); }\n")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if n != nil {
			count++
		}
		return true
	})
	if count < 8 {
		t.Fatalf("walked only %d nodes", count)
	}
	var dump bytes.Buffer
	if err := ast.Dump(&dump, file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump.String(), "FunctionDeclaration fragment") {
		t.Fatalf("unexpected dump: %s", dump.String())
	}
	data, err := json.Marshal(ast.JSONValue(file))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"kind":"CallExpression"`)) {
		t.Fatalf("missing JSON node kinds: %s", data)
	}
}

func TestExpressionParenthesesPreserveSemantics(t *testing.T) {
	file, err := shader.ParseString("void fragment() { float x = a * (b + c); float y = a - (b - c); }\n")
	if err != nil {
		t.Fatal(err)
	}
	got := shader.Format(file)
	if !strings.Contains(got, "a * (b + c)") || !strings.Contains(got, "a - (b - c)") {
		t.Fatalf("parentheses lost:\n%s", got)
	}
}

func TestPositionedErrors(t *testing.T) {
	_, err := shader.ParseFile("bad.gdshader", []byte("void fragment() {\n  vec3 x = ;\n}"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "bad.gdshader:2:12") {
		t.Fatalf("error is not positioned: %v", err)
	}
}
