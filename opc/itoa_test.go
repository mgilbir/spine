package opc

import (
	"math"
	"strconv"
	"testing"
)

// TestItoaMinInt guards against a regression where the hand-rolled itoa
// infinitely recursed on math.MinInt: it did `return "-" + itoa(-n)`, but
// -math.MinInt overflows back to math.MinInt, so the recursion never
// terminated and overflowed the stack. Values flow here from
// ExtendedProperties.Marshal, whose integer fields originate from
// strconv.Atoi on untrusted core/app XML, so a hostile MinInt is reachable.
func TestItoaMinInt(t *testing.T) {
	cases := []int{0, 1, -1, 10, -10, math.MaxInt, math.MinInt}
	for _, n := range cases {
		got := itoa(n)
		want := strconv.Itoa(n)
		if got != want {
			t.Errorf("itoa(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestExtendedPropertiesMarshalMinInt exercises the exported Marshal path with
// a MinInt-valued integer field to confirm it no longer overflows the stack.
func TestExtendedPropertiesMarshalMinInt(t *testing.T) {
	ep := &ExtendedProperties{Words: math.MinInt}
	out, err := ep.Marshal()
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	want := "<Words>" + strconv.Itoa(math.MinInt) + "</Words>"
	if !containsSub(string(out), want) {
		t.Errorf("Marshal output missing %q; got:\n%s", want, out)
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
