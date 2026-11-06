package xlsx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// TestSharedStringIndexReadsAgree confirms every reader of a shared-string cell
// parses its index the same way. String used strconv.Atoi, RichText (and
// ReplaceText through it) a digit loop with no overflow check, and the copy
// path Atoi after trimming. So <v>18446744073709551618</v> read as "" through
// String but wrapped to entry 2 ("Gamma") through RichText, and ReplaceText
// then wrote that entry's edited text into the cell; <v> 2</v> read as "" but
// copied as "Gamma" (C614).
func TestSharedStringIndexReadsAgree(t *testing.T) {
	base := buildXlsxPartsFixture()
	orig := string(fuzzseed.ZipEntry(base, partSheet1))
	const cell = `<c r="B1" t="s"><v>2</v></c>`
	if !strings.Contains(orig, cell) {
		t.Fatal("fixture no longer has B1 as shared string 2")
	}
	for _, tc := range []struct{ v, want string }{
		{"2", "Gamma"},
		{" 2\t", "Gamma"},
		{"18446744073709551618", ""}, // 2^64+2: wrapped to 2 in the digit loop
		{"9223372036854775810", ""},  // 2^63+2: wraps to a negative int
		{"2147483650", ""},
		{"-1", ""},
		{"+2", "Gamma"},
		{"+-2", ""},
		{"++2", ""},
		{"", ""},
		{"x", ""},
	} {
		t.Run(tc.v, func(t *testing.T) {
			mod := strings.Replace(orig, cell, `<c r="B1" t="s"><v>`+tc.v+`</v></c>`, 1)
			pkg := fuzzseed.EditZip(base, [][2]string{{partSheet1, mod}})
			open := func() *Cell {
				t.Helper()
				w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
				if err != nil {
					t.Fatalf("open: %v", err)
				}
				s, _ := w.SheetByName("Data")
				c, err := s.Cell("B1")
				if err != nil {
					t.Fatal(err)
				}
				return c
			}

			c := open()
			if got := c.String(); got != tc.want {
				t.Errorf("String = %q, want %q", got, tc.want)
			}
			var rich strings.Builder
			for _, r := range c.RichText() {
				rich.WriteString(r.Text)
			}
			if rich.Len() > 0 && rich.String() != tc.want {
				t.Errorf("RichText = %q, String = %q", rich.String(), tc.want)
			}

			// ReplaceText must not write another entry's text into the cell.
			c = open()
			c.sheet.workbook.ReplaceText(map[string]string{"Gam": "XX"})
			want := strings.ReplaceAll(tc.want, "Gam", "XX")
			if got := c.String(); got != want {
				t.Errorf("after ReplaceText: %q, want %q", got, want)
			}

			// A copy into another workbook resolves the same entry.
			c = open()
			sheets, err := Create().AppendSheetsFrom(c.sheet.workbook)
			if err != nil {
				t.Fatal(err)
			}
			copied := false
			for _, s := range sheets {
				if s.Name() != "Data" {
					continue
				}
				copied = true
				dc, err := s.Cell("B1")
				if err != nil {
					t.Fatal(err)
				}
				if got := dc.String(); got != tc.want {
					t.Errorf("copied = %q, want %q", got, tc.want)
				}
			}
			if !copied {
				t.Fatal("no copied sheet named Data")
			}
		})
	}
}
