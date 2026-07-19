package crypto

import "testing"

func TestLegacyPasswordHash(t *testing.T) {
	// Known values from Excel's documented legacy algorithm.
	cases := map[string]uint16{
		"":     0x0000,
		"test": 0xCBEB,
	}
	for in, want := range cases {
		if got := LegacyPasswordHash(in); got != want {
			t.Errorf("LegacyPasswordHash(%q) = %04X, want %04X", in, got, want)
		}
	}
}
