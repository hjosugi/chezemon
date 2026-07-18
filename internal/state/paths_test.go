package state

import (
	"path/filepath"
	"testing"
)

// testDest returns a destination directory plus a helper that builds absolute
// target paths inside it.
//
// Every test that feeds paths through parseStatus must build them this way
// rather than writing literals such as "/home/test/.bashrc". parseStatus calls
// filepath.Clean on each path and filepath.Join for relative ones, and on
// Windows a slash-separated literal is neither absolute nor separator-correct:
// "/home/test/.real.json" is silently joined onto the destination, becoming
// "\home\test\home\test\.real.json", and metadata lookups keyed on the literal
// miss entirely.
//
// That has broken windows-latest three times — twice with a visible failure,
// and once with a test that passed while asserting nothing, because a missed
// metadata lookup produces the same "not sensitive" result the test expected.
// Build paths here so the next test copied from this package is portable by
// default.
func testDest(t *testing.T) (dest string, target func(segments ...string) string) {
	t.Helper()
	dest = t.TempDir()
	return dest, func(segments ...string) string {
		return filepath.Join(append([]string{dest}, segments...)...)
	}
}
