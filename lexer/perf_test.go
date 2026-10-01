package lexer_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/cafecito-games/gdparser/lexer"
)

// Scanning an operator used to copy the rest of the source into a string to match
// a prefix against, which made a file cost time and memory in proportion to the
// square of its length. The match reads the bytes in place now, so eight times the
// source costs about eight times as much rather than sixty-four times.
func TestLexCostGrowsWithTheSourceNotItsSquare(t *testing.T) {
	small := allocatedPerLex(t, operatorSource(200))
	large := allocatedPerLex(t, operatorSource(1600))
	if large > small*25 {
		t.Fatalf("eight times the source allocated %d bytes against %d, which is more than linear", large, small)
	}
}

// operatorSource builds a function body of lines dense with operators, which is
// what the quadratic scan was proportional to.
func operatorSource(lines int) []byte {
	var out strings.Builder
	out.WriteString("func f(a, b, c, d, e, g):\n\tvar value = 0\n")
	for range lines {
		out.WriteString("\tvalue = (a + b) * (c - d) / e % g\n")
	}
	return []byte(out.String())
}

// allocatedPerLex reports the bytes one lex of source allocates, averaged over a
// few runs so that a single collection cannot skew it.
func allocatedPerLex(t *testing.T, source []byte) uint64 {
	t.Helper()
	const runs = 20
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range runs {
		if _, err := lexer.Lex(source); err != nil {
			t.Fatal(err)
		}
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / runs
}
