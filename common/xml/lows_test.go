package xml

import "testing"

// TestLexTagAttrs_TrailingWhitespaceForcesVerbatimFallback covers C347: a start
// tag with whitespace before '>' (e.g. `<a foo="1" >`) has no attribute to
// attach the trailing space to. lexTagAttrs must report ok=false so callers
// keep the verbatim source rather than silently dropping the space on replay.
func TestLexTagAttrs_TrailingWhitespaceForcesVerbatimFallback(t *testing.T) {
	if _, ok := lexTagAttrs([]byte(`a foo="1" `)); ok {
		t.Fatal("expected ok=false for a tag with trailing whitespace before '>' (the space would otherwise be dropped on replay)")
	}
	// The clean, space-free form must still lex to one attribute.
	raws, ok := lexTagAttrs([]byte(`a foo="1"`))
	if !ok || len(raws) != 1 || raws[0] != ` foo="1"` {
		t.Fatalf("clean tag: ok=%v raws=%v", ok, raws)
	}
}
