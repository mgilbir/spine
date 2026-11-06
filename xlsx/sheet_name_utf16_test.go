package xlsx

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestSheetNameLimitCountsUTF16Units confirms the 31-character sheet name limit
// counts UTF-16 code units, as Excel does ([MS-XLS] §2.4.28: the name's cch is
// at most 31). It counted runes, so a name of 16 emoji — 32 units — was
// accepted and written, and UniqueSheetName could return one (C618).
func TestSheetNameLimitCountsUTF16Units(t *testing.T) {
	const emoji = "\U0001F600" // outside the BMP: two UTF-16 units
	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{strings.Repeat("a", 31), true},
		{strings.Repeat("a", 32), false},
		{strings.Repeat("é", 31), true}, // two UTF-8 bytes, one UTF-16 unit
		{strings.Repeat(emoji, 15) + "a", true},
		{strings.Repeat(emoji, 16), false},
		{strings.Repeat("a", 30) + emoji, false},
	} {
		if err := ValidateSheetName(tc.name); (err == nil) != tc.ok {
			t.Errorf("ValidateSheetName(%d runes, %d units) = %v, want ok=%v", utf8.RuneCountInString(tc.name), utf16Len(tc.name), err, tc.ok)
		}
		if _, err := Create().AddSheet(tc.name); (err == nil) != tc.ok {
			t.Errorf("AddSheet(%d units) = %v, want ok=%v", utf16Len(tc.name), err, tc.ok)
		}
	}

	w := Create()
	long := strings.Repeat(emoji, 20)
	first := w.UniqueSheetName(long)
	if first != strings.Repeat(emoji, 15) {
		t.Fatalf("UniqueSheetName = %q (%d units), want 15 emoji", first, utf16Len(first))
	}
	if _, err := w.AddSheet(first); err != nil {
		t.Fatal(err)
	}
	// A collision appends " (2)", which must still fit and must not split a
	// surrogate pair: 26 units of name leave room for 13 emoji.
	second := w.UniqueSheetName(long)
	if second != strings.Repeat(emoji, 13)+" (2)" {
		t.Fatalf("UniqueSheetName after collision = %q (%d units)", second, utf16Len(second))
	}
	if err := ValidateSheetName(second); err != nil {
		t.Fatal(err)
	}
	// In 32 units, "a" and 15 emoji fill 31; the 16th emoji needs two more,
	// and half of one is not a character.
	if got := truncateUTF16("a"+strings.Repeat(emoji, 20), 32); got != "a"+strings.Repeat(emoji, 15) || !utf8.ValidString(got) {
		t.Fatalf("truncateUTF16 = %q", got)
	}
}
