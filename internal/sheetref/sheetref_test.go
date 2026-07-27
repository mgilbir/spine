package sheetref

import "testing"

// TestQuoteName covers the whole quoting rule: plain identifiers stay bare,
// names with unsafe characters are quoted, and — the case that motivated this
// package — names that lex as a cell reference or a boolean literal are quoted
// even though every character is safe.
func TestQuoteName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		// Unambiguous identifiers stay bare.
		{"Sheet1", "Sheet1"},
		{"Data", "Data"},
		{"_hidden", "_hidden"},
		{"Q1.summary", "Q1.summary"},
		{"ChartData1", "ChartData1"},
		{"Revenue", "Revenue"},
		{"R2D2", "R2D2"},
		{"CAT", "CAT"},
		{"XFE1", "XFE1"},         // past the last column, so not a reference
		{"A1048577", "A1048577"}, // past the last row
		// Unsafe characters and leading digits.
		{"", "''"},
		{"My Sheet", "'My Sheet'"},
		{"O'Brien", "'O''Brien'"},
		{"2024", "'2024'"},
		{"a-b", "'a-b'"},
		// Cell-reference-shaped names.
		{"A1", "'A1'"},
		{"a1", "'a1'"},
		{"XFD1", "'XFD1'"},
		{"XFD1048576", "'XFD1048576'"},
		{"B12", "'B12'"},
		// R1C1-shaped names.
		{"R1C1", "'R1C1'"},
		{"R", "'R'"},
		{"C", "'C'"},
		{"RC", "'RC'"},
		{"r12c9", "'r12c9'"},
		{"C3", "'C3'"}, // also an A1 reference
		// Boolean literals.
		{"TRUE", "'TRUE'"},
		{"false", "'false'"},
	}
	for _, tc := range cases {
		if got := QuoteName(tc.name); got != tc.want {
			t.Errorf("QuoteName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
