package uidfile_test

import (
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/uidfile"
)

const byteOrderMark = "\xef\xbb\xbf"

// A byte order mark is encoding, not syntax, so a sidecar carrying one holds a
// valid UID. Its bytes still count toward offsets, but not toward columns.
func TestParseSkipsByteOrderMark(t *testing.T) {
	const body = "uid://Di2QVXKyjionN\n"
	file, err := uidfile.ParseString(byteOrderMark + body)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if file.UID.Value != "uid://Di2QVXKyjionN" {
		t.Errorf("UID = %q, want %q", file.UID.Value, "uid://Di2QVXKyjionN")
	}
	start := file.UID.Span().Start
	if start.Offset != len(byteOrderMark) || start.Line != 1 || start.Column != 1 {
		t.Errorf("UID start = %+v, want offset %d at 1:1", start, len(byteOrderMark))
	}
	formatted := uidfile.Format(file)
	if formatted != body {
		t.Errorf("Format = %q, want %q", formatted, body)
	}
	if strings.Contains(formatted, byteOrderMark) {
		t.Error("formatted output still carries a byte order mark")
	}
}

// A mark is skipped, not accepted in place of a UID, so what follows it is still
// held to the sidecar grammar and reported at the first real column.
func TestParseByteOrderMarkOnly(t *testing.T) {
	_, err := uidfile.ParseString(byteOrderMark)
	if err == nil {
		t.Fatal("ParseString succeeded on a mark alone, want an error")
	}
	if !strings.Contains(err.Error(), "1:1") {
		t.Errorf("error = %q, want it reported at 1:1", err)
	}
}
