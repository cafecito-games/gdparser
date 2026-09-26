# gdparser

[![Go Reference](https://pkg.go.dev/badge/github.com/cafecito-games/gdparser.svg)](https://pkg.go.dev/github.com/cafecito-games/gdparser)
[![CI](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml/badge.svg)](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml)

`gdparser` is an open-source Go 1.26 library for parsing and transforming Godot
4 source and text formats. It provides typed, mutable trees and canonical
emitters for GDScript, text scenes and resources, project settings, and the
Godot shading language. The repository also includes a CLI for printing trees,
JSON, or canonical source.

## Status

The GDScript parser is validated against a compatibility corpus of 2,000 files
totaling roughly 414,000 lines. The text-resource, project-configuration, and
shader parsers are also exercised against representative Godot projects. Corpus
validation covers parsing, canonical formatting, reparsing, and normalized
structural comparison.

These results are compatibility milestones, not claims of complete equivalence
with Godot's parsers. Godot continues to evolve, and unsupported syntax returns
a positioned error rather than being silently accepted. Binary `.scn` and
`.res` files are intentionally outside the current scope.

## Install

Add the library to a Go module:

```sh
go get github.com/cafecito-games/gdparser
```

Install the CLI:

```sh
go install github.com/cafecito-games/gdparser/cmd/gdparser@latest
```

The project requires Go 1.26 or newer.

## Library usage

Parse a file, inspect or mutate its typed AST, and emit canonical GDScript:

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cafecito-games/gdparser"
	"github.com/cafecito-games/gdparser/ast"
)

func main() {
	source, err := os.ReadFile("player.gd")
	if err != nil {
		log.Fatal(err)
	}
	file, err := gdparser.ParseFile("player.gd", source)
	if err != nil {
		log.Fatal(err)
	}

	ast.Inspect(file, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Identifier); ok && ident.Name == "speed" {
			ident.Name = "movement_speed"
		}
		return true
	})

	fmt.Print(gdparser.Format(file))
}
```

The other formats use explicit packages so their distinct tree types cannot be
accidentally mixed:

```go
scene, err := textresource.ParseFile("level.tscn", sceneSource)
sceneSource = []byte(textresource.Format(scene))

project, err := projectconfig.ParseFile("project.godot", projectSource)
projectSource = []byte(projectconfig.Format(project))

program, err := shader.ParseFile("water.gdshader", shaderSource)
shaderSource = []byte(shader.Format(program))
```

Import these packages from `github.com/cafecito-games/gdparser/textresource`,
`github.com/cafecito-games/gdparser/projectconfig`, and
`github.com/cafecito-games/gdparser/shader`.

For backward compatibility, the root package exposes the GDScript entry points:

- `Parse(source)` parses an in-memory byte slice.
- `ParseString(source)` parses an in-memory string.
- `ParseFile(filename, source)` parses bytes with a diagnostic filename.
- `Format(file)` emits canonical GDScript from an AST.

Lower-level packages are available when a tool needs more control:

- `lexer` converts source into indentation-aware tokens.
- `parser` builds the typed AST.
- `ast` defines nodes, source spans, traversal, tree dumps, and JSON values.
- `format` emits canonical GDScript.
- `token` defines token kinds and source positions.
- `textresource` parses the shared `.tscn`, `.tres`, and `.escn` syntax.
- `projectconfig` parses ConfigFile syntax used by `project.godot`.
- `shader` parses `.gdshader` and `.gdshaderinc` source.

## CLI

The CLI infers the input type from `.gd`, `.tscn`, `.tres`, `.escn`,
`.gdshader`, `.gdshaderinc`, or the `project.godot` filename.

Print a readable syntax tree:

```sh
gdparser player.gd
gdparser level.tscn
gdparser -format tree water.gdshader
```

Print JSON:

```sh
gdparser -format json player.gd
```

Emit canonical source for any supported format:

```sh
gdparser -format source player.gd
gdparser -format source level.tscn
```

Pass `-` or omit the path to read from standard input. Standard input defaults
to GDScript; use `-type` for another format:

```sh
printf 'var answer = 42\n' | gdparser -format json
gdparser -type resource -format json < level.tscn
gdparser -type project -format source < project.godot
gdparser -type shader -format tree < water.gdshader
```

The legacy `-format gdscript` spelling remains an alias for `-format source`.

Diagnostics include the filename, line, and column. The CLI exits non-zero for
invalid input or an unsupported output format.

## Format coverage

### GDScript

The implementation supports the Godot 4 syntax exercised by the compatibility
corpus, including:

- classes, annotations, signals, enums, variables, constants, and functions;
- typed declarations, return types, generics, and accessor blocks;
- `if`/`elif`/`else`, `for`, `while`, `match`, `break`, `continue`, `pass`,
  `return`, and `assert`;
- literals, collections, calls, subscripts, attributes, lambdas, casts,
  conditional expressions, `await`, and `preload`;
- operators with GDScript precedence and associativity;
- multiline expressions, escaped identifiers, `$`/`%` node paths, and
  statement continuations;
- line and inline comments represented in the AST.

### Text scenes and resources

The `textresource` package supports ordered section headers and attributes,
properties, comments, recursive Variant values, constructors and resource
references, arrays and dictionaries, typed containers, `StringName`,
`NodePath`, special floating-point values, and suffixed integers.

### Project configuration

The `projectconfig` package supports ordered preamble assignments and sections,
semicolon and hash comments, slash-delimited and quoted keys, escaped section
names, and recursive ConfigFile Variant values including `Object(...)` forms.

### Shaders

The `shader` package supports shader and render declarations, uniforms and
hints, structs, functions, typed variables and arrays, control flow,
preprocessor directives, comments, and precedence-aware expressions.

## AST and formatting model

Every syntax node has a source span with byte offsets and one-based line and
column positions. Nodes are mutable Go structs. Each frontend provides
traversal, tree-dump, and JSON conversion facilities with explicit node-kind
discriminators.

Formatting is canonical rather than lossless. The emitter preserves program
structure and comments, but it may normalize indentation, spacing, parentheses,
blank lines, and literal spelling. If exact source trivia is required, retain
the original source alongside the AST.

## Development

Run the standard checks from the repository root:

```sh
gofmt -w .
go test -race ./...
go vet ./...
go build ./...
```

An external Godot project can be used as an opt-in compatibility corpus:

```sh
GDPARSER_CORPUS=/path/to/godot/project go test -run TestCorpus -count=1 -v .
```

The corpus test discovers all supported files recursively and verifies parse,
format/reparse, and normalized structural equality. The external corpus is
read-only and is not included in this repository.

See [AGENTS.md](AGENTS.md) for the repository architecture, invariants, and
contribution workflow.

## License

MIT. See [LICENSE](LICENSE).
