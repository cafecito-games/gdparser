# gdparser

[![Go Reference](https://pkg.go.dev/badge/github.com/cafecito-games/gdparser.svg)](https://pkg.go.dev/github.com/cafecito-games/gdparser)
[![CI](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml/badge.svg)](https://github.com/cafecito-games/gdparser/actions/workflows/ci.yml)

`gdparser` is an open-source Go 1.26 library for parsing and transforming Godot
4 source and text formats. It provides typed, mutable trees and canonical
emitters for GDScript, text scenes and resources, ConfigFile documents,
resource UID sidecars, and the Godot shading language. The repository also
includes a CLI for printing trees, JSON, or canonical source.

## Status

The GDScript parser is validated against a compatibility corpus of 2,000 files
totaling roughly 414,000 lines. The text-resource, ConfigFile, UID-sidecar, and
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

config, err := configfile.ParseFile("project.godot", configSource)
configSource = []byte(configfile.Format(config))

program, err := shader.ParseFile("water.gdshader", shaderSource)
shaderSource = []byte(shader.Format(program))

uid, err := uidfile.ParseFile("player.gd.uid", uidSource)
uidSource = []byte(uidfile.Format(uid))
```

Import these packages from `github.com/cafecito-games/gdparser/textresource`,
`github.com/cafecito-games/gdparser/configfile`,
`github.com/cafecito-games/gdparser/shader`, and
`github.com/cafecito-games/gdparser/uidfile`.

For backward compatibility, the root package exposes the GDScript entry points:

- `Parse(source)` parses an in-memory byte slice.
- `ParseString(source)` parses an in-memory string.
- `ParseFile(filename, source)` parses bytes with a diagnostic filename.
- `Format(file)` emits canonical GDScript from an AST.

Lower-level packages are available when a tool needs more control:

- `lexer` converts source into indentation-aware tokens.
- `parser` builds the typed AST.
- `ast` defines nodes, source spans, traversal, tree dumps, and JSON values.
- `format` emits canonical GDScript, and `format.Options` configures it.
- `token` defines token kinds and source positions.
- `textresource` parses the shared `.tscn`, `.tres`, and `.escn` syntax.
- `configfile` parses ConfigFile syntax used by `project.godot`, `.cfg`,
  `.gdextension`, `.import`, and `.remap` files.
- `shader` parses `.gdshader` and `.gdshaderinc` source.
- `uidfile` parses `.uid` sidecars used by source resources.

## CLI

The CLI infers the input type from `.gd`, `.tscn`, `.tres`, `.escn`, `.cfg`,
`.gdextension`, `.import`, `.remap`, `.gdshader`, `.gdshaderinc`, `.uid`, or
the `project.godot` filename.

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
gdparser -type config -format source < project.godot
gdparser -type shader -format tree < water.gdshader
gdparser -type uid -format json < player.gd.uid
```

The legacy `-format gdscript` spelling remains an alias for `-format source`.

Diagnostics include the filename, line, and column. The CLI exits non-zero for
invalid input or an unsupported output format.

## Format coverage

### GDScript

The implementation supports the Godot 4 syntax exercised by the compatibility
corpus, including:

- classes, annotations, signals, enums, variables, constants, and functions;
- typed declarations, return types, generics, and accessor blocks, typed or
  untyped;
- rest parameters, written `...name`, which `ast.Parameter.Variadic` reports;
- `if`/`elif`/`else`, `for`, `while`, `match`, `break`, `continue`, `pass`,
  `return`, and `assert`;
- `match` patterns, including the wildcard, array and dictionary patterns, and
  the `var name` binding pattern that `ast.BindingPattern` represents;
- both dictionary spellings, `{"key": value}` and `{key = value}`, which
  `ast.DictionaryLiteral.LuaStyle` tells apart;
- literals, collections, calls, subscripts, attributes, lambdas (including the
  named form Godot reports in a stack trace), casts, conditional expressions,
  `await`, and `preload`;
- operators with GDScript precedence and associativity;
- multiline expressions, escaped identifiers, `$`/`%` node paths, and
  statement continuations;
- line and inline comments represented in the AST.

### Text scenes and resources

The `textresource` package supports ordered section headers and attributes,
properties, comments, recursive Variant values, constructors and resource
references, arrays and dictionaries, typed containers, `StringName`,
`NodePath`, special floating-point values, and suffixed integers.

### ConfigFile documents

The `configfile` package supports ordered preamble assignments and sections,
semicolon and hash comments, slash-delimited and quoted keys, escaped section
names, physical multiline strings, and recursive ConfigFile Variant values
including `Object(...)` forms. It covers project settings, export presets,
plugin and GDExtension descriptors, import metadata, and resource remaps.

### Resource UID sidecars

The `uidfile` package validates, represents, traverses, and canonically emits
the single `uid://...` identifier stored in `.uid` sidecars.

### Shaders

The `shader` package supports shader and render declarations, uniforms and
hints, structs, functions, typed variables and arrays, control flow,
preprocessor directives, comments, and precedence-aware expressions.

## AST and formatting model

Every syntax node has a source span with byte offsets and one-based line and
column positions. Nodes are mutable Go structs. Each frontend provides
traversal, tree-dump, and JSON conversion facilities with explicit node-kind
discriminators.

Parsed GDScript nodes also retain precise component spans for authored names,
types, keywords, and operators. These spans always refer to the original input:
mutating an AST does not recompute them, so they may become stale. Trees built
programmatically may leave source and component spans at their zero values.

Formatting is canonical rather than lossless. The emitter preserves program
structure and comments, but it may normalize indentation, spacing, parentheses,
blank lines, and literal spelling. If exact source trivia is required, retain
the original source alongside the AST.

A comment written on a line of its own neither opens nor closes a block, which
is how Godot's tokenizer treats it. Its indentation only chooses the block it
belongs to, and the next line of code settles that: a comment written at an
outer level still closes the blocks the code below it has left, but when that
code stays inside a deeper block the comment belongs there too, whatever its
own indentation. A comment indented to a level matching no block keeps the
scope of the nearest block above it rather than failing to parse.

Statements also record the formatting-relevant context the source gave them:

- `ast.Trivia` holds the blank lines written before a statement and the comment
  written after it on the same line. `ast.TriviaOf` reads it from any statement.
- Annotations are attached to the declaration they decorate, through the
  `Annotations` field on variable, function, class, signal, and enum
  declarations. `ast.Annotations` reads them from any node. An annotation that
  decorates nothing, such as a file-level `@icon`, remains its own statement.
  This is a deliberate change in tree shape: `@export var speed` is one
  statement, not an annotation statement followed by a declaration, and a
  comment written after a statement is that statement's trivia rather than a
  sibling comment statement.
- `ast.CollectionComment` holds a comment written where no statement exists to
  carry it. Arrays, dictionaries, enum bodies, argument lists, and parameter
  lists expose the comments between their delimiters, and a `match` statement
  exposes those between its cases, each anchored to the item it was written
  against and marked when it ended that item's line.
- String literals record their quote character and the triple-quoted and
  `r`-prefixed forms, so they can be requoted without re-lexing their escapes.

## Style guide formatting

`format.File` emits the layout described by the
[Godot GDScript style guide](https://docs.godotengine.org/en/stable/tutorials/scripting/gdscript/gdscript_styleguide.html):
tabs for indentation, lines kept within 100 columns, two blank lines around
function and class declarations, double quotes unless single quotes escape fewer
characters, `and`/`or`/`not` in place of `&&`/`||`/`!`, one space after a comment
marker, and trailing commas in arrays, dictionaries, and enums that span several
lines.

A construct that does not fit the column budget is broken one element per line.
Call arguments, parameter lists, and conditions indent two levels, while arrays,
dictionaries, and enums indent one, as the guide prescribes. Breaking a logical
expression wraps it in parentheses and starts each continuation line with its
`and` or `or` keyword.

`format.FileWithOptions` takes a `format.Options` for tools that need to differ.
Every field's zero value is the style guide default, so a partially populated
`Options` inherits the rest:

```go
source := format.FileWithOptions(file, format.Options{LineWidth: 80})
```

Two rules are worth calling out. Blank lines written inside a function are kept
but clamped, so hand-placed logical breaks survive while runs of blank lines do
not. Comment spacing is normalized even though the guide asks for no space on
commented-out code, because prose and commented-out code cannot be told apart;
`CommentSpacing: PreserveComments` turns that off.

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
format/reparse, normalized structural equality, and, for GDScript, that
formatting is idempotent and leaves no trailing whitespace. The structural
comparison canonicalizes the spellings that formatting is defined to normalize,
so a respelled literal, comment, or boolean operator is not reported as lost
structure. The external corpus is read-only and is not included in this
repository.

See [AGENTS.md](AGENTS.md) for the repository architecture, invariants, and
contribution workflow.

## License

MIT. See [LICENSE](LICENSE).
