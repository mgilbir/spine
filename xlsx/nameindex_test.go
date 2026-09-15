package xlsx

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestFoldKeyMatchesEqualFold pins the claim that makes the name sets correct:
// keying a map on foldKey reproduces strings.EqualFold exactly. strings.ToLower
// would not — it leaves the long s alone while EqualFold folds it onto s — and
// the difference is a collision the scan used to report and a map would miss.
func TestFoldKeyMatchesEqualFold(t *testing.T) {
	samples := []string{
		"", "a", "A", "Data", "DATA", "dAtA",
		"ſ", "s", "S", // long s folds onto s
		"K", "k", "K", // Kelvin sign folds onto k
		"Σ", "σ", "ς", // sigma, final sigma
		"ß", "ẞ",
		"straße", "STRASSE",
		"Sheet1", "sheet1", "Sheet 1",
		"日本", "日本語",
	}
	for _, a := range samples {
		for _, b := range samples {
			want := strings.EqualFold(a, b)
			got := foldKey(a) == foldKey(b)
			if got != want {
				t.Errorf("foldKey(%q)==foldKey(%q) is %v, but EqualFold is %v", a, b, got, want)
			}
		}
	}
}

// TestAddSheetKeepsNameSetWarm guards the linearity: adding sheets must not
// rebuild the collision set per add, which is the O(sheets) scan it replaced.
func TestAddSheetKeepsNameSetWarm(t *testing.T) {
	const sheets = 400

	wb := Create()
	for i := 0; i < sheets; i++ {
		if _, err := wb.AddSheet(fmt.Sprintf("S%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if wb.nameSetRebuilds > 2 {
		t.Errorf("name set rebuilt %d times while adding %d sheets (want <= 2)",
			wb.nameSetRebuilds, sheets)
	}
	// The check must still work after all that maintenance.
	if _, err := wb.AddSheet("s0"); err == nil {
		t.Error("AddSheet(\"s0\") succeeded; it collides with \"S0\" case-insensitively")
	}
	if _, err := wb.AddSheet("brand-new"); err != nil {
		t.Errorf("AddSheet of a free name failed: %v", err)
	}
}

// TestSheetRenameIsSeenByCollisionCheck covers the one mutation the set's
// count-based staleness check cannot see: a rename leaves the number of sheets
// alone.
func TestSheetRenameIsSeenByCollisionCheck(t *testing.T) {
	wb := Create()
	a, err := wb.AddSheet("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wb.AddSheet("Beta"); err != nil {
		t.Fatal(err)
	}
	// Warm the set.
	if _, err := wb.AddSheet("Gamma"); err != nil {
		t.Fatal(err)
	}

	if err := a.SetName("Delta"); err != nil {
		t.Fatal(err)
	}

	// "Alpha" is free now.
	if _, err := wb.AddSheet("Alpha"); err != nil {
		t.Errorf("AddSheet(\"Alpha\") failed after the sheet was renamed away: %v", err)
	}
	// "Delta" is taken now.
	if _, err := wb.AddSheet("delta"); err == nil {
		t.Error("AddSheet(\"delta\") succeeded; the rename to \"Delta\" should collide")
	}
}

// TestSheetIDsStayUniqueAcrossDelete guards the cached maximum sheet id: a
// delete can lower the maximum, so the cache has to notice. A reused id makes
// two sheets indistinguishable in workbook.xml.
func TestSheetIDsStayUniqueAcrossDelete(t *testing.T) {
	wb := Create()
	for _, n := range []string{"A", "B", "C"} {
		if _, err := wb.AddSheet(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := wb.DeleteSheet(1); err != nil {
		t.Fatal(err)
	}
	if _, err := wb.AddSheet("D"); err != nil {
		t.Fatal(err)
	}

	assertSheetIDsUnique(t, wb)

	// The direction that corrupts is a cached maximum that is too LOW, and a
	// workbook opened from bytes is where it comes from: nothing added its
	// sheets through AddSheet, so a cache that is never computed from the
	// parsed model starts at zero and hands out an id an existing sheet holds.
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.AddSheet("E"); err != nil {
		t.Fatal(err)
	}
	assertSheetIDsUnique(t, reopened)
}

func assertSheetIDsUnique(t *testing.T, wb *Workbook) {
	t.Helper()
	seen := map[uint32]string{}
	for _, s := range wb.workbook.Sheets.Sheet {
		if prev, dup := seen[s.SheetId]; dup {
			t.Errorf("sheet id %d used by both %q and %q", s.SheetId, prev, s.Name)
		}
		seen[s.SheetId] = s.Name
	}
}

// TestDefinedNameCollisionIsScopedAndFolded checks the indexed collision check
// still distinguishes scopes and still folds case.
func TestDefinedNameCollisionIsScopedAndFolded(t *testing.T) {
	wb := Create()
	sh, err := wb.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.SetCellValue("A1", 1); err != nil {
		t.Fatal(err)
	}

	if err := wb.AddDefinedName("Total", "S!$A$1"); err != nil {
		t.Fatal(err)
	}
	if err := wb.AddDefinedName("TOTAL", "S!$A$1"); err == nil {
		t.Error("workbook-scoped \"TOTAL\" was accepted alongside \"Total\"")
	}
	// A different scope is a different name.
	if err := wb.AddDefinedNameScoped("Total", "S!$A$1", 0); err != nil {
		t.Errorf("sheet-scoped \"Total\" rejected though only a workbook-scoped one exists: %v", err)
	}
	if err := wb.AddDefinedNameScoped("total", "S!$A$1", 0); err == nil {
		t.Error("sheet-scoped \"total\" was accepted alongside sheet-scoped \"Total\"")
	}
}

// TestAddDefinedNameKeepsSetWarm guards the linearity of the defined-name path.
func TestAddDefinedNameKeepsSetWarm(t *testing.T) {
	const names = 500

	wb := Create()
	sh, err := wb.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.SetCellValue("A1", 1); err != nil {
		t.Fatal(err)
	}
	before := wb.nameSetRebuilds
	for i := 0; i < names; i++ {
		if err := wb.AddDefinedName(fmt.Sprintf("total_%d", i), "S!$A$1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := wb.nameSetRebuilds - before; got > 2 {
		t.Errorf("name set rebuilt %d times while adding %d defined names (want <= 2)", got, names)
	}
	if err := wb.AddDefinedName("TOTAL_0", "S!$A$1"); err == nil {
		t.Error("\"TOTAL_0\" was accepted alongside \"total_0\"")
	}
}
