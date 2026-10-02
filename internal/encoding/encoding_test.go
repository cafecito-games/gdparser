package encoding_test

import (
	"testing"

	"github.com/cafecito-games/gdparser/internal/encoding"
)

func TestSkipByteOrderMark(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   int
	}{
		{"empty source", "", 0},
		{"no mark", "extends Node\n", 0},
		{"a leading mark", "\xef\xbb\xbfextends Node\n", 3},
		{"nothing but a mark", "\xef\xbb\xbf", 3},
		{"a mark that is not at the start", "x\xef\xbb\xbf", 0},
		{"a truncated mark", "\xef\xbb", 0},
		{"a repeated mark", "\xef\xbb\xbf\xef\xbb\xbf", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := encoding.SkipByteOrderMark([]byte(test.source)); got != test.want {
				t.Errorf("SkipByteOrderMark(%q) = %d, want %d", test.source, got, test.want)
			}
		})
	}
}
