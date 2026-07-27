package xlsx

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C176: overwriting a shared-formula master must not orphan the group's
// followers as `<f t="shared" si="N"/>` stubs with no master.

func TestTranslateFormula(t *testing.T) {
	tests := []struct {
		name       string
		formula    string
		dRow, dCol int
		want       string
	}{
		{"no offset", "A1*2", 0, 0, "A1*2"},
		{"row shift", "A1*2", 1, 0, "A2*2"},
		{"col shift", "A1*2", 0, 2, "C1*2"},
		{"both axes", "A1+B2", 3, 1, "B4+C5"},
		{"negative shift", "C3", -2, -2, "A1"},
		{"fully absolute is fixed", "$A$1*2", 5, 5, "$A$1*2"},
		{"col-absolute shifts row only", "$A1", 2, 3, "$A3"},
		{"row-absolute shifts col only", "A$1", 2, 3, "D$1"},
		{"range", "SUM(A1:B2)", 1, 0, "SUM(A2:B3)"},
		{"mixed range", "SUM($A$1:A2)", 1, 1, "SUM($A$1:B3)"},
		{"string literal untouched", `CONCATENATE("A1",A1)`, 1, 0, `CONCATENATE("A1",A2)`},
		{"escaped quote in literal", `IF(A1=1,"say ""A1"" here",B1)`, 1, 0, `IF(A2=1,"say ""A1"" here",B2)`},
		{"quoted sheet name untouched", "'A1 data'!A1+A1", 1, 0, "'A1 data'!A2+A2"},
		{"unquoted sheet name untouched", "Sheet1!A1", 1, 0, "Sheet1!A2"},
		{"ref-shaped sheet name untouched", "AB1!C1", 1, 0, "AB1!C2"},
		{"function name not a ref", "LOG10(A1)", 1, 0, "LOG10(A2)"},
		{"defined name untouched", "MyName1+A1", 1, 0, "MyName1+A2"},
		{"name with dot untouched", "My.Name+A1", 1, 0, "My.Name+A2"},
		{"four letters is a name", "ABCD1", 1, 0, "ABCD1"},
		{"col out of grid is a name", "XFE1", 1, 1, "XFE1"},
		{"row out of grid is a name", "A1048577", 1, 0, "A1048577"},
		{"leading zero row is a name", "A01", 1, 0, "A01"},
		{"error literal untouched", "A1+#REF!", 1, 0, "A2+#REF!"},
		{"shift off top of grid", "A1-1", -1, 0, "#REF!-1"},
		{"shift off left of grid", "A1", 0, -1, "#REF!"},
		{"shift off bottom of grid", "A1048576", 1, 0, "#REF!"},
		{"boolean literal untouched", "IF(TRUE,A1,B1)", 1, 0, "IF(TRUE,A2,B2)"},
		// C285: a scientific-notation exponent must not be lexed as a cell ref.
		{"sci notation exponent not a ref", "1.5E2+B1", 1, 0, "1.5E2+B2"},
		{"sci notation lowercase e", "1.5e2+B1", 1, 0, "1.5e2+B2"},
		{"sci notation signed exponent", "1.5E+2+B1", 1, 0, "1.5E+2+B2"},
		{"sci notation negative exponent", "2E-3+A1", 1, 0, "2E-3+A2"},
		{"integer sci notation", "3E4+A1", 0, 1, "3E4+B1"},
		{"leading-dot sci notation", ".5E2+A1", 1, 0, ".5E2+A2"},
		{"real ref E-column still shifts", "E2+1", 1, 0, "E3+1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := translateFormula(tc.formula, tc.dRow, tc.dCol); got != tc.want {
				t.Errorf("translateFormula(%q, %d, %d) = %q, want %q",
					tc.formula, tc.dRow, tc.dCol, got, tc.want)
			}
		})
	}
}

// sharedFormulaSheet has B1 as the master of shared group 0 (ref B1:B3) with
// B2 and B3 as followers; B3 carries its own cached formula text.
const sharedFormulaSheet = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
	`<row r="1"><c r="A1"><v>1</v></c><c r="B1"><f t="shared" ref="B1:B3" si="0">A1*2</f><v>2</v></c></row>` +
	`<row r="2"><c r="A2"><v>3</v></c><c r="B2"><f t="shared" si="0"/><v>6</v></c></row>` +
	`<row r="3"><c r="A3"><v>5</v></c><c r="B3"><f t="shared" si="0">A3*2</f><v>10</v></c></row>` +
	`</sheetData></worksheet>`

// TestSetValueOnSharedFormulaMasterMaterializesFollowers is the P15 scenario:
// overwriting the master with a plain value must leave every follower with a
// plain, correctly translated formula and remove all group bookkeeping.
func TestSetValueOnSharedFormulaMasterMaterializesFollowers(t *testing.T) {
	data := buildMutatorTestXlsx(t, sharedFormulaSheet)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := wb.Sheets()[0].SetCellValue("B1", 42); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	if strings.Contains(sheet, `si="0"`) || strings.Contains(sheet, `t="shared"`) {
		t.Errorf("shared-formula bookkeeping left behind after master overwrite:\n%s", sheet)
	}
	// B2 had an empty stub: it gets the master's formula shifted by +1 row.
	if !strings.Contains(sheet, `<c r="B2"><f>A2*2</f>`) {
		t.Errorf("follower B2 not materialized as plain translated formula:\n%s", sheet)
	}
	// B3 carried its own cached text: it must be kept, not re-derived.
	if !strings.Contains(sheet, `<c r="B3"><f>A3*2</f>`) {
		t.Errorf("follower B3 lost its cached formula text:\n%s", sheet)
	}

	// The saved sheet must still parse, and the master must hold the value.
	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(sheet), &ws); err != nil {
		t.Fatalf("saved sheet does not parse: %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got, _ := reopened.Sheets()[0].GetCellValue("B1"); got != "42" {
		t.Errorf("master value after overwrite = %q, want 42", got)
	}
}

// TestSharedFormulaSciNotationPreservedInFollowers is the C285 regression:
// materializing a shared-formula follower must keep a scientific-notation
// constant (1.5E2) intact instead of mistaking its exponent for a cell ref
// (which produced 1.5E3, i.e. 150 -> 1500).
func TestSharedFormulaSciNotationPreservedInFollowers(t *testing.T) {
	w := Create()
	s := addSheetT(w, "S")
	a1, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	if err := a1.SetSharedFormula("1.5E2+B1", "A1:A3"); err != nil {
		t.Fatalf("SetSharedFormula: %v", err)
	}
	// Overwrite the master to materialize the followers as plain formulas.
	a1.SetFormula("999")

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	// Follower A2 is shifted by +1 row: only the real ref B1 becomes B2; the
	// numeric constant 1.5E2 stays exactly as written.
	if !strings.Contains(sheet, `<c r="A2"><f>1.5E2+B2</f>`) {
		t.Errorf("follower A2 formula corrupted (want 1.5E2+B2):\n%s", sheet)
	}
	if strings.Contains(sheet, "1.5E3") {
		t.Errorf("scientific-notation exponent shifted as a cell ref (1.5E3):\n%s", sheet)
	}
}

// TestSetFormulaOnSharedFormulaMasterMaterializesFollowers verifies that
// REPLACING the master's formula (not just clearing it) also detaches the
// group first.
func TestSetFormulaOnSharedFormulaMasterMaterializesFollowers(t *testing.T) {
	data := buildMutatorTestXlsx(t, sharedFormulaSheet)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cell, err := wb.Sheets()[0].Cell("B1")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	cell.SetFormula("A1+100")
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	if strings.Contains(sheet, `t="shared"`) || strings.Contains(sheet, `si="0"`) {
		t.Errorf("shared-formula bookkeeping left behind after master replace:\n%s", sheet)
	}
	for _, want := range []string{
		`<c r="B1"><f>A1+100</f>`, // master got the new plain formula
		`<c r="B2"><f>A2*2</f>`,   // stub follower translated from the OLD master formula
		`<c r="B3"><f>A3*2</f>`,   // cached follower text kept
	} {
		if !strings.Contains(sheet, want) {
			t.Errorf("saved sheet is missing %q:\n%s", want, sheet)
		}
	}
}

// TestSetValueOnSharedFormulaFollowerKeepsGroupIntact verifies that
// overwriting a FOLLOWER only drops that follower's stub: the master keeps
// its group definition and the remaining followers keep their stubs.
func TestSetValueOnSharedFormulaFollowerKeepsGroupIntact(t *testing.T) {
	data := buildMutatorTestXlsx(t, sharedFormulaSheet)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := wb.Sheets()[0].SetCellValue("B2", 5); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	for _, want := range []string{
		`<c r="B1"><f t="shared" ref="B1:B3" si="0">A1*2</f>`, // master untouched
		`<c r="B3"><f t="shared" si="0">A3*2</f>`,             // other follower untouched
		`<c r="B2" t="n"><v>5</v></c>`,                        // overwritten follower has no formula
	} {
		if !strings.Contains(sheet, want) {
			t.Errorf("saved sheet is missing %q:\n%s", want, sheet)
		}
	}
}

// TestSharedFormulaAbsoluteRefsAnchored verifies absolute and mixed reference
// handling when a master over a 2D range is overwritten.
func TestSharedFormulaAbsoluteRefsAnchored(t *testing.T) {
	const sheet2D = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
		`<row r="1"><c r="B1"><f t="shared" ref="B1:C2" si="7">$A$1+A1+$A1+A$1</f><v>0</v></c><c r="C1"><f t="shared" si="7"/><v>0</v></c></row>` +
		`<row r="2"><c r="B2"><f t="shared" si="7"/><v>0</v></c><c r="C2"><f t="shared" si="7"/><v>0</v></c></row>` +
		`</sheetData></worksheet>`

	data := buildMutatorTestXlsx(t, sheet2D)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := wb.Sheets()[0].SetCellValue("B1", "done"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	for _, want := range []string{
		`<c r="C1"><f>$A$1+B1+$A1+B$1</f>`, // +1 col
		`<c r="B2"><f>$A$1+A2+$A2+A$1</f>`, // +1 row
		`<c r="C2"><f>$A$1+B2+$A2+B$1</f>`, // +1 row, +1 col
	} {
		if !strings.Contains(sheet, want) {
			t.Errorf("saved sheet is missing %q:\n%s", want, sheet)
		}
	}
	if strings.Contains(sheet, `t="shared"`) {
		t.Errorf("shared bookkeeping left behind:\n%s", sheet)
	}
}
