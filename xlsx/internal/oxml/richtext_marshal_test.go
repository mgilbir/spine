package oxml

import "testing"

func TestNeedsSpacePreserve(t *testing.T) {
	cases := map[string]bool{
		"":          false,
		"x":         false,
		"hello":     false,
		" leading":  true,
		"trailing ": true,
		"a\tb":      false,
		"a\n":       true,
	}
	for in, want := range cases {
		if got := needsSpacePreserve(in); got != want {
			t.Errorf("needsSpacePreserve(%q) = %v, want %v", in, got, want)
		}
	}
}
