// Command gdparser parses Godot source and text files and prints their syntax
// tree or canonical source.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cafecito-games/gdparser"
	"github.com/cafecito-games/gdparser/ast"
	"github.com/cafecito-games/gdparser/projectconfig"
	projectast "github.com/cafecito-games/gdparser/projectconfig/ast"
	"github.com/cafecito-games/gdparser/shader"
	shaderast "github.com/cafecito-games/gdparser/shader/ast"
	"github.com/cafecito-games/gdparser/textresource"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gdparser", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outputFormat := flags.String("format", "tree", "output format: tree, json, or source")
	inputType := flags.String("type", "auto", "input type: auto, gdscript, resource, project, or shader")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: gdparser [-type auto|gdscript|resource|project|shader] [-format tree|json|source] [file|-]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return 2
	}
	if !validInputType(*inputType) {
		fmt.Fprintf(stderr, "gdparser: unknown input type %q\n", *inputType)
		return 2
	}

	filename := "-"
	reader := stdin
	if flags.NArg() == 1 && flags.Arg(0) != "-" {
		filename = flags.Arg(0)
		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(stderr, "gdparser: %v\n", err)
			return 1
		}
		defer file.Close()
		reader = file
	}
	source, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintf(stderr, "gdparser: %v\n", err)
		return 1
	}
	document, err := parseInput(*inputType, filename, source)
	if err != nil {
		fmt.Fprintf(stderr, "gdparser: %v\n", err)
		return 1
	}

	switch *outputFormat {
	case "tree":
		if err := document.dump(stdout); err != nil {
			fmt.Fprintf(stderr, "gdparser: %v\n", err)
			return 1
		}
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(document.json()); err != nil {
			fmt.Fprintf(stderr, "gdparser: %v\n", err)
			return 1
		}
	case "source", "gdscript": // gdscript is retained as a compatibility alias.
		if _, err := io.WriteString(stdout, document.source()); err != nil {
			fmt.Fprintf(stderr, "gdparser: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "gdparser: unknown format %q\n", *outputFormat)
		return 2
	}
	return 0
}

type parsedDocument struct {
	dump   func(io.Writer) error
	json   func() any
	source func() string
}

func parseInput(kind, filename string, source []byte) (parsedDocument, error) {
	if kind == "auto" {
		kind = inferInputType(filename)
	}
	switch kind {
	case "gdscript":
		file, err := gdparser.ParseFile(filename, source)
		if err != nil {
			return parsedDocument{}, err
		}
		return parsedDocument{
			dump:   func(writer io.Writer) error { return ast.Dump(writer, file) },
			json:   func() any { return ast.JSONValue(file) },
			source: func() string { return gdparser.Format(file) },
		}, nil
	case "resource", "textresource":
		file, err := textresource.ParseFile(filename, source)
		if err != nil {
			return parsedDocument{}, err
		}
		return parsedDocument{
			dump:   func(writer io.Writer) error { return textresource.Dump(writer, file) },
			json:   func() any { return textresource.JSONValue(file) },
			source: func() string { return textresource.Format(file) },
		}, nil
	case "project", "projectconfig":
		file, err := projectconfig.ParseFile(filename, source)
		if err != nil {
			return parsedDocument{}, err
		}
		return parsedDocument{
			dump:   func(writer io.Writer) error { return projectast.Dump(writer, file) },
			json:   func() any { return projectast.JSONValue(file) },
			source: func() string { return projectconfig.Format(file) },
		}, nil
	case "shader":
		file, err := shader.ParseFile(filename, source)
		if err != nil {
			return parsedDocument{}, err
		}
		return parsedDocument{
			dump:   func(writer io.Writer) error { return shaderast.Dump(writer, file) },
			json:   func() any { return shaderast.JSONValue(file) },
			source: func() string { return shader.Format(file) },
		}, nil
	default:
		return parsedDocument{}, fmt.Errorf("unknown input type %q", kind)
	}
}

func inferInputType(filename string) string {
	if filepath.Base(filename) == "project.godot" {
		return "project"
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".tscn", ".tres", ".escn":
		return "resource"
	case ".gdshader", ".gdshaderinc":
		return "shader"
	default:
		// Standard input and unknown extensions retain the original behavior.
		return "gdscript"
	}
}

func validInputType(kind string) bool {
	switch kind {
	case "auto", "gdscript", "resource", "textresource", "project", "projectconfig", "shader":
		return true
	default:
		return false
	}
}
