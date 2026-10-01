package parser_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/ast"
	gdformat "github.com/cafecito-games/gdparser/format"
	"github.com/cafecito-games/gdparser/parser"
)

func TestNamedLambda(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "an inline named lambda",
			source: "func a():\n\tvar f = func named(): pass\n",
			want:   "func a():\n\tvar f = func named(): pass\n",
		},
		{
			name:   "a multiline named lambda",
			source: "func a():\n\tvar f = func named():\n\t\tpass\n",
			want:   "func a():\n\tvar f = func named():\n\t\tpass\n",
		},
		{
			name:   "parameters and a return type",
			source: "func a():\n\tvar f = func named(x: int) -> int:\n\t\treturn x\n",
			want:   "func a():\n\tvar f = func named(x: int) -> int:\n\t\treturn x\n",
		},
		{
			name:   "a named lambda inside a collection",
			source: "var x = [func named():\n\t\tpass\n]\n",
			want:   "var x = [\n\tfunc named():\n\t\tpass,\n]\n",
		},
		{
			name:   "an anonymous lambda is unchanged",
			source: "func a():\n\tvar f = func(): pass\n",
			want:   "func a():\n\tvar f = func(): pass\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.Parse("lambda.gd", []byte(test.source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			formatted := gdformat.File(file)
			if formatted != test.want {
				t.Errorf("formatted = %q, want %q", formatted, test.want)
			}
			again, err := parser.Parse("lambda.gd", []byte(formatted))
			if err != nil {
				t.Fatalf("formatted source did not parse: %v\n%s", err, formatted)
			}
			if reformatted := gdformat.File(again); reformatted != formatted {
				t.Errorf("formatting is not idempotent:\n%s\n--- became ---\n%s", formatted, reformatted)
			}
		})
	}
}

func TestNamedLambdaNode(t *testing.T) {
	file, err := parser.Parse("lambda.gd", []byte("func a():\n\tvar f = func named(): pass\n"))
	if err != nil {
		t.Fatal(err)
	}
	body := file.Statements[0].(*ast.FunctionDeclaration).Body
	lambda := body[0].(*ast.VariableDeclaration).Value.(*ast.LambdaExpression)
	if lambda.Name != "named" {
		t.Fatalf("name = %q, want \"named\"", lambda.Name)
	}
	if lambda.NameSpan.Start.Line != 2 || lambda.NameSpan.Start.Column != 15 {
		t.Errorf("name span start = %v, want 2:15", lambda.NameSpan.Start)
	}
	if lambda.NameSpan.End.Column != 20 {
		t.Errorf("name span end = %v, want column 20", lambda.NameSpan.End)
	}
}

func TestLambdaWithoutNameOrParametersIsRejected(t *testing.T) {
	if _, err := parser.Parse("lambda.gd", []byte("func a():\n\tvar f = func: pass\n")); err == nil {
		t.Fatal("expected a parse error")
	}
}
