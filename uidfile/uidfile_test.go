package uidfile_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/uidfile"
)

func TestParseFormatTraverseJSONAndDump(t *testing.T) {
	file, err := uidfile.ParseFile("player.gd.uid", []byte("uid://c3m2k2i8we5da\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "player.gd.uid" || file.UID.Value != "uid://c3m2k2i8we5da" {
		t.Fatalf("file = %#v", file)
	}
	if got := file.UID.Span(); got.Start.Offset != 0 || got.End.Offset != 19 || got.End.Line != 1 || got.End.Column != 20 {
		t.Fatalf("UID span = %#v", got)
	}
	if got := file.Span().End; got.Offset != 20 || got.Line != 2 || got.Column != 1 {
		t.Fatalf("file end = %#v", got)
	}

	visited := 0
	uidfile.Inspect(file, func(node uidfile.Node) bool {
		if node != nil {
			visited++
		}
		if uid, ok := node.(*uidfile.UID); ok {
			uid.Value = "uid://4g30kasivvy0"
		}
		return true
	})
	if visited != 2 {
		t.Fatalf("visited = %d", visited)
	}
	formatted := uidfile.Format(file)
	if formatted != "uid://4g30kasivvy0\n" {
		t.Fatalf("formatted = %q", formatted)
	}
	if _, err := uidfile.ParseString(formatted); err != nil {
		t.Fatalf("reparse canonical output: %v", err)
	}
	json := uidfile.JSONValue(file).(map[string]any)
	if json["kind"] != "File" || json["uid"].(map[string]any)["kind"] != "UID" {
		t.Fatalf("JSON = %#v", json)
	}
	var dump bytes.Buffer
	if err := uidfile.Dump(&dump, file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump.String(), "UID uid://4g30kasivvy0") {
		t.Fatalf("dump:\n%s", dump.String())
	}
}

func TestCRLFAndMissingFinalNewline(t *testing.T) {
	for _, source := range []string{"uid://abc123", "uid://abc123\r\n", "uid://Di2QVXKyjionN\n", "uid://<invalid>\n"} {
		file, err := uidfile.ParseString(source)
		if err != nil {
			t.Fatalf("ParseString(%q): %v", source, err)
		}
		if uidfile.Format(file) != strings.TrimRight(source, "\r\n")+"\n" {
			t.Fatalf("formatted = %q", uidfile.Format(file))
		}
	}
}

func TestPositionedErrorsDoNotPanic(t *testing.T) {
	tests := []struct {
		source string
		line   int
		column int
	}{
		{"", 1, 1},
		{"abc", 1, 1},
		{"uid://", 1, 7},
		{"uid://abc!", 1, 10},
		{"uid://abc\nextra", 2, 1},
		{"uid://abc\n\n", 2, 1},
	}
	for _, test := range tests {
		t.Run(strings.ReplaceAll(test.source, "\n", `\n`), func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("panic: %v", recovered)
				}
			}()
			_, err := uidfile.ParseFile("broken.uid", []byte(test.source))
			if err == nil {
				t.Fatal("expected an error")
			}
			var parseError *uidfile.Error
			if !errors.As(err, &parseError) {
				t.Fatalf("error type = %T", err)
			}
			if parseError.Position.Line != test.line || parseError.Position.Column != test.column {
				t.Fatalf("position = %#v, want %d:%d", parseError.Position, test.line, test.column)
			}
			if !strings.Contains(err.Error(), "broken.uid:") {
				t.Fatalf("error lacks filename: %v", err)
			}
		})
	}
}
