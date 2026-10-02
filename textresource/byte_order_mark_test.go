package textresource

import (
	"strings"
	"testing"
	"time"

	"github.com/cafecito-games/gdparser/token"
)

const byteOrderMark = "\xef\xbb\xbf"

// A byte order mark is encoding, not syntax. Before it was skipped it was taken
// for the start of a bare word, which read no bytes and hung the scan.
func TestParseSkipsByteOrderMark(t *testing.T) {
	const body = "[gd_resource type=\"Resource\" format=3]\n\n[resource]\nx = 1\n"
	document, err := ParseString(byteOrderMark + body)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if len(document.Items) == 0 {
		t.Fatal("parsed document has no items")
	}
	section, ok := document.Items[0].(*Section)
	if !ok {
		t.Fatalf("first item = %T, want *Section", document.Items[0])
	}
	if section.Type != "gd_resource" {
		t.Errorf("first section type = %q, want %q", section.Type, "gd_resource")
	}
	want := token.Position{Offset: len(byteOrderMark), Line: 1, Column: 1}
	if start := section.Span().Start; start != want {
		t.Errorf("first section start = offset %d at %s, want offset %d at %s",
			start.Offset, start, want.Offset, want)
	}
	formatted := Format(document)
	if formatted != body {
		t.Errorf("Format = %q, want %q", formatted, body)
	}
	if strings.Contains(formatted, byteOrderMark) {
		t.Error("formatted output still carries a byte order mark")
	}
}

func TestParseByteOrderMarkOnly(t *testing.T) {
	document, err := ParseString(byteOrderMark)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}
	if len(document.Items) != 0 {
		t.Errorf("a mark alone parsed to %d items, want none", len(document.Items))
	}
}

// A rune that cannot begin a bare word must be reported where it appears. Every
// byte above ASCII used to be taken for the start of one, and because none of
// these runes can continue one either, the scan read nothing and never ended.
func TestParseRejectsRunesThatCannotStartABareWord(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{"a non-breaking space", "[resource]\n\xc2\xa0 = 1\n"},
		{"a guillemet", "[resource]\nx = \xc2\xbb\n"},
		{"a line separator", "[resource]\n\xe2\x80\xa8x = 1\n"},
		{"an emoji", "[resource]\n\xf0\x9f\x98\x80 = 1\n"},
		{"a mark away from the start", "[resource]\nx = \xef\xbb\xbf\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed := make(chan error, 1)
			go func() {
				_, err := ParseString(test.source)
				parsed <- err
			}()
			select {
			case err := <-parsed:
				if err == nil {
					t.Fatalf("ParseString(%q) succeeded, want an error", test.source)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("ParseString(%q) did not return", test.source)
			}
		})
	}
}
