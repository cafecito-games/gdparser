# gdparser

[![CI](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml/badge.svg)](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/cafecito-games/gdparser.svg)](https://pkg.go.dev/github.com/cafecito-games/gdparser)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`gdparser` is an open-source Godot 4 GDScript parser, transformable AST, and
source emitter written in Go 1.26. It is primarily a Go library; the included
CLI makes the parser easy to inspect and script.

The project takes its initial AST vocabulary and fixture coverage from
[`csueiras/gdast`](https://github.com/csueiras/gdast), a Python AST builder for
GDScript. Unlike that builder, `gdparser` supports both directions: GDScript to
AST and AST back to canonical GDScript.

> [!NOTE]
> This is the initial development release. The core syntax emitted by `gdast`
> is supported, but complete parity with every Godot parser extension is an
> ongoing goal. Unsupported syntax returns a positioned error.

## Install

The module requires Go 1.26 or newer.

```sh
go get github.com/cafecito-games/gdparser
go install github.com/cafecito-games/gdparser/cmd/gdparser@latest
```

## Library

Parse a source file:

```go
package main

import (
	"fmt"
	"log"

	"github.com/cafecito-games/gdparser"
	"github.com/cafecito-games/gdparser/ast"
)

func main() {
	file, err := gdparser.ParseFile("player.gd", []byte(`
extends CharacterBody2D

func damage(amount: int) -> bool:
	health -= amount
	return health <= 0
`))
	if err != nil {
		log.Fatal(err)
	}

	ast.Inspect(file, func(node ast.Node) bool {
		if identifier, ok := node.(*ast.Identifier); ok && identifier.Name == "health" {
			identifier.Name = "hit_points"
		}
		return true
	})

	fmt.Print(gdparser.Format(file))
}
```

The public packages have distinct responsibilities:

- `gdparser` provides the small top-level parse and format API.
- `ast` defines typed, mutable nodes plus `Walk`, `Inspect`, `Dump`, and JSON
  projection helpers.
- `lexer`, `token`, `parser`, and `format` expose the lower-level pipeline for
  tools that need it.

Every AST node includes a byte offset and one-based line/column span. Concrete
pointer nodes are intentionally mutable, making identifier rewrites and other
source-to-source transformations straightforward.

## CLI

Print a readable tree (the default):

```sh
gdparser player.gd
```

```text
File "player.gd"
  Directive extends
    Identifier CharacterBody2D
  FunctionDeclaration damage
    Assignment -=
      Identifier health
      Identifier amount
    ReturnStatement
      BinaryExpression <=
        Identifier health
        Literal 0
```

Machine-readable JSON includes a `kind` discriminator for every node:

```sh
gdparser -format json player.gd
```

Parse and emit canonical GDScript:

```sh
gdparser -format gdscript player.gd
cat player.gd | gdparser -format tree -
```

## Current syntax coverage

- Indentation-sensitive blocks and source spans
- Comments, documentation comments, annotations, `class_name`, and `extends`
- `var`, `const`, typed/inferred declarations, assignments, signals, and enums
- Functions, parameters/defaults, return types, static functions, and inner
  classes
- `if`/`elif`/`else`, `while`, `for`, `match`, `return`, `pass`, `break`, and
  `continue`
- Scalar, array, and dictionary literals; calls; member/subscript access;
  node paths; unary, binary, cast, membership, and ternary expressions
- Single-, double-, and triple-quoted strings

Near-term work includes lambdas, property getter/setter blocks, richer match
patterns, multiline continuation edge cases, and full conformance testing
against Godot's own parser. Syntax choices follow the official
[GDScript reference](https://docs.godotengine.org/en/stable/tutorials/scripting/gdscript/gdscript_basics.html).

## Development

```sh
go test -race ./...
go vet ./...
go build ./cmd/gdparser
```

Contributions should include parser and formatter round-trip coverage for new
syntax. This project is available under the [MIT License](LICENSE).
