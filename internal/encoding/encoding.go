// Package encoding holds the text-encoding helpers shared by the lexers. What
// lives here describes how bytes become text, never what a grammar means.
package encoding

import "bytes"

// ByteOrderMark is the UTF-8 encoding of U+FEFF.
const ByteOrderMark = "\xef\xbb\xbf"

// SkipByteOrderMark returns the offset at which source begins once a leading
// byte order mark is passed over, which is zero when there is none.
//
// Godot strips the mark while decoding a file, so no Godot parser ever sees one
// and a lexer here must not read it as syntax. It is skipped rather than sliced
// away so that every offset stays absolute against the bytes the caller read,
// and it is given no column of its own so that the first real character of the
// file still reports as 1:1.
func SkipByteOrderMark(source []byte) int {
	if bytes.HasPrefix(source, []byte(ByteOrderMark)) {
		return len(ByteOrderMark)
	}
	return 0
}
