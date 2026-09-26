// Command gdparser parses GDScript and prints its AST or canonical source.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/cafecito-games/gdparser"
	"github.com/cafecito-games/gdparser/ast"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gdparser", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outputFormat := flags.String("format", "tree", "output format: tree, json, or gdscript")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: gdparser [-format tree|json|gdscript] [file.gd|-]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		flags.Usage()
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
	file, err := gdparser.ParseFile(filename, source)
	if err != nil {
		fmt.Fprintf(stderr, "gdparser: %v\n", err)
		return 1
	}

	switch *outputFormat {
	case "tree":
		if err := ast.Dump(stdout, file); err != nil {
			fmt.Fprintf(stderr, "gdparser: %v\n", err)
			return 1
		}
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(ast.JSONValue(file)); err != nil {
			fmt.Fprintf(stderr, "gdparser: %v\n", err)
			return 1
		}
	case "gdscript":
		if _, err := io.WriteString(stdout, gdparser.Format(file)); err != nil {
			fmt.Fprintf(stderr, "gdparser: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "gdparser: unknown format %q\n", *outputFormat)
		return 2
	}
	return 0
}
