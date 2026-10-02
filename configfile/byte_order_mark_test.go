package configfile_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/configfile"
	"github.com/cafecito-games/gdparser/configfile/ast"
	"github.com/cafecito-games/gdparser/configfile/token"
)

const byteOrderMark = "\xef\xbb\xbf"

// A byte order mark is encoding, not syntax, so it must not become part of the
// first key. Its bytes still count toward offsets, but not toward columns.
func TestParseSkipsByteOrderMark(t *testing.T) {
	const body = "config_version=5\n\n[application]\nconfig/name=\"x\"\n"
	file, err := configfile.ParseString(byteOrderMark + body)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if len(file.Preamble) == 0 {
		t.Fatal("parsed file has an empty preamble")
	}
	assignment, ok := file.Preamble[0].(*ast.Assignment)
	if !ok {
		t.Fatalf("first preamble statement = %T, want *ast.Assignment", file.Preamble[0])
	}
	if assignment.Key != "config_version" {
		t.Errorf("first key = %q, want %q", assignment.Key, "config_version")
	}
	want := token.Position{Offset: len(byteOrderMark), Line: 1, Column: 1}
	if start := assignment.Span().Start; start != want {
		t.Errorf("first statement start = offset %d at %s, want offset %d at %s",
			start.Offset, start, want.Offset, want)
	}
	formatted := configfile.Format(file)
	if formatted != body {
		t.Errorf("Format = %q, want %q", formatted, body)
	}
	if strings.Contains(formatted, byteOrderMark) {
		t.Error("formatted output still carries a byte order mark")
	}
}

func TestParseByteOrderMarkOnly(t *testing.T) {
	file, err := configfile.ParseString(byteOrderMark)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if len(file.Preamble) != 0 || len(file.Sections) != 0 {
		t.Errorf("a mark alone parsed to %d preamble statements and %d sections, want none",
			len(file.Preamble), len(file.Sections))
	}
}
