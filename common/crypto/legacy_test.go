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

// TestLegacyPasswordHashIsPerCharacter pins the hash to ECMA-376 §18.3.1.75,
// which defines it over the password's characters. Iterating a Go string's
// UTF-8 bytes instead — and mixing in the byte count — silently produced a
// different value for every non-ASCII password, so the protection element
// spine wrote did not match the one Excel writes for the same password.
func TestLegacyPasswordHashIsPerCharacter(t *testing.T) {
	// Expected values recomputed from the specified algorithm over UTF-16 code
	// units. Each differs from the byte-wise value listed alongside it, which is
	// what the buggy implementation produced.
	cases := []struct {
		password        string
		want, wantBytes uint16
	}{
		{"café", 0xC2AD, 0xD52C},   // 4 characters, 5 UTF-8 bytes
		{"naïve", 0xC3AE, 0xD47D},  // 5 characters, 6 UTF-8 bytes
		{"Straße", 0xC81B, 0xC79A}, // 6 characters, 7 UTF-8 bytes
		{"密码", 0x99C3, 0xF33B},     // 2 characters, 6 UTF-8 bytes
	}
	for _, c := range cases {
		got := LegacyPasswordHash(c.password)
		if got != c.want {
			t.Errorf("LegacyPasswordHash(%q) = %04X, want %04X", c.password, got, c.want)
		}
		if got == c.wantBytes {
			t.Errorf("LegacyPasswordHash(%q) still hashes UTF-8 bytes (%04X)", c.password, got)
		}
	}

	// ASCII is unaffected: bytes and characters coincide.
	if got := LegacyPasswordHash("test"); got != 0xCBEB {
		t.Errorf("ASCII regression: LegacyPasswordHash(\"test\") = %04X, want CBEB", got)
	}
}
