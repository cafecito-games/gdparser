# gdparser

[![Go Reference](https://pkg.go.dev/badge/github.com/cafecito-games/gdparser.svg)](https://pkg.go.dev/github.com/cafecito-games/gdparser)
[![CI](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml/badge.svg)](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml)

`gdparser` is an open-source Godot 4 GDScript parser, mutable abstract syntax
tree, and canonical source emitter written in Go 1.26. It is designed primarily
as a Go library for tools that need to inspect or transform GDScript. The
repository also includes a CLI for printing an AST or reformatted source.

## Status

The parser currently handles the language used by the Uzir client compatibility
corpus: 2,000 GDScript files totaling roughly 414,000 lines. Every file in that
corpus passes parsing, canonical formatting, reparsing, and normalized AST
comparison.

That result is a compatibility milestone, not a claim of complete equivalence
with Godot's parser. GDScript and Godot continue to evolve, and unsupported
syntax returns a positioned error rather than being silently accepted.

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

The root package exposes the common entry points:

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

## CLI

Print a readable AST tree:

```sh
gdparser player.gd
gdparser -format tree player.gd
```

Print JSON:

```sh
gdparser -format json player.gd
```

Emit canonical GDScript:

```sh
gdparser -format gdscript player.gd
```

Pass `-` or omit the path to read from standard input:

```sh
printf 'var answer = 42\n' | gdparser -format json
```

Diagnostics include the filename, line, and column. The CLI exits non-zero for
invalid input or an unsupported output format.

## Language coverage

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

## AST and formatting model

Every AST node has a source span with byte offsets and one-based line and column
positions. Nodes are mutable Go structs and can be traversed with `ast.Walk` or
`ast.Inspect`. `ast.Dump` produces the CLI tree representation, while
`ast.JSONValue` provides a stable JSON-friendly representation with explicit
node-kind discriminators.

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

An external GDScript tree can be used as an opt-in compatibility corpus:

```sh
GDPARSER_CORPUS=/path/to/gdscript/project go test -run TestCorpus -count=1 -v .
```

The corpus test discovers `.gd` files recursively and verifies parse,
format/reparse, and normalized AST structural equality. The external corpus is
read-only and is not included in this repository.

See [AGENTS.md](AGENTS.md) for the repository architecture, invariants, and
contribution workflow.

## License

MIT. See [LICENSE](LICENSE).
