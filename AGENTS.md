# AGENTS.md

## Scope

These instructions apply to the entire repository.

`gdparser` is a Go 1.26 library and CLI for parsing Godot 4 source and text
formats into typed, mutable syntax trees and emitting canonical source. It
supports GDScript, text scenes/resources (`.tscn`, `.tres`, and `.escn`),
`project.godot`, and the Godot shading language (`.gdshader` and
`.gdshaderinc`). Treat the library API as the primary product; the CLI is a
thin user-facing adapter.

## Repository map

- `gdparser.go`: backward-compatible GDScript convenience API.
- `token/`, `lexer/`, `ast/`, `parser/`, `format/`: the GDScript pipeline.
- `textresource/`: text scene/resource parsing and canonical emission.
- `configfile/`: ConfigFile parsing and canonical emission for `project.godot`,
  `.cfg`, `.gdextension`, `.import`, and `.remap` files.
- `shader/`: Godot shading-language parsing and canonical emission.
- `uidfile/`: resource UID sidecar parsing and canonical emission.
- `cmd/gdparser/`: command-line interface.
- `corpus_test.go`: opt-in external corpus validation.

The primary data flow is:

```text
Godot source -> format-specific lexer/parser -> typed tree -> tree, JSON, or canonical source
```

## Core invariants

- Accepted syntax must be represented by typed AST nodes. Do not add raw-source
  or opaque fallback nodes to make unsupported syntax appear to parse.
- Every node must carry a useful source span with byte offsets and one-based
  line and column positions.
- Keep AST definitions, `Children`, traversal, tree dumps, JSON conversion, and
  formatting in sync whenever a node or field changes.
- The parser must return positioned errors for invalid or unsupported input; it
  must not panic on user-provided source.
- The formatter must support every public AST node and produce source that the
  parser can read again.
- Preserve expression semantics. In particular, emit parentheses whenever the
  AST's precedence or associativity requires them.
- GDScript indentation is syntax. Keep indent/dedent handling, multiline
  expressions, comments, and suite boundaries correct.
- Text resources and project settings are ordered document formats. Do not use
  maps where doing so would discard source order or conceal duplicate entries.
- Keep the grammars separate. Similar-looking constructs may share carefully
  factored implementation utilities, but GDScript, Variant text values,
  ConfigFile documents, and shader code are not interchangeable languages.
- Comments must remain in their semantic scope after formatting, even though
  exact whitespace and blank-line preservation is not required.
- Prefer the standard library. Add a dependency only when its maintenance and
  behavior are clearly justified.
- Do not hard-code a developer's local corpus path or require an external
  project for the default test suite.

## Change workflow

For syntax work, update the full pipeline rather than only the parser:

1. Add a focused regression test that demonstrates the new construct or bug.
2. Add or adjust typed AST nodes and their traversal/serialization support.
3. Update tokenization and parsing as required.
4. Update canonical formatting.
5. Test parse, format/reparse, and structural behavior.

Keep fixtures small and intentional. When a failure is discovered in a large
external project, reduce it to the smallest useful test case instead of copying
generated or unrelated project files into this repository.

## Required checks

Run these before considering a change complete:

```sh
gofmt -w .
go test -race ./...
go vet ./...
go build ./...
git diff --check
```

When a representative Godot project is available, also run:

```sh
GDPARSER_CORPUS=/path/to/godot/project go test -run TestCorpus -count=1 -v .
```

The corpus is an additional compatibility gate, not a substitute for focused
unit tests. Treat it as read-only and do not modify it to make a test pass.

## Testing guidance

- Lexer changes should cover indentation, token positions, continuations, and
  newline-sensitive behavior.
- Expression changes should cover precedence, associativity, grouping, and
  their formatted output.
- Statement changes should cover nested suites and adjacent comments.
- Text-resource changes should cover scene and resource headers, ordered
  sections and properties, resource references, and recursive Variant values.
- ConfigFile changes should cover preamble properties, sections,
  slash-delimited keys, comments, multiline strings, and recursive Variant
  values.
- UID-sidecar changes should cover validation, positions, traversal, and
  canonical emission.
- Shader changes should cover declarations, blocks, control flow, preprocessing,
  precedence, associativity, grouping, and formatted output.
- AST changes should cover traversal and JSON output as well as parsing.
- Formatter changes should be idempotent where practical and must always
  produce parseable output for supported ASTs.
- CLI changes should exercise all affected output modes and error exits.

## Go and API style

- Use idiomatic Go and run `gofmt` over every changed Go file.
- Document exported identifiers and keep public names explicit and stable.
- Preserve backward compatibility unless a breaking change is deliberate and
  documented.
- Keep the top-level package convenient; specialized controls belong in the
  lower-level packages.
- Return errors with enough filename and position context to diagnose the
  source without reproducing it in a debugger.

## Repository hygiene

- Preserve unrelated work in a dirty worktree.
- Do not commit binaries, generated corpus output, editor state, or local paths.
- Do not modify external compatibility projects while testing this repository.
- Avoid destructive Git operations and force pushes.
- Do not push or publish changes unless the user explicitly requests it.
