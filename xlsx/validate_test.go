package xlsx

import (
	"testing"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

func hasCode(r validate.Report, code string, sev validate.Severity) bool {
	for _, e := range r {
		if e.Code == code && e.Severity == sev {
			return true
		}
	}
	return false
}

func u32(v uint32) *uint32 { return &v }

// A shared-formula follower with no master defining its si is an orphan (C176).
func TestValidate_SharedFormulaOrphan(t *testing.T) {
	ws := &oxml.CT_Worksheet{
		SheetData: oxml.CT_SheetData{
			Row: []oxml.CT_Row{
				{C: []*oxml.CT_Cell{
					{R: "A1", F: &oxml.CT_CellFormula{T: "shared", Si: u32(3), Ref: ""}}, // follower, no master
				}},
			},
		},
	}
	w := &Workbook{sheets: []*Sheet{{partName: "/xl/worksheets/sheet1.xml", worksheet: ws}}}
	if r := w.Validate(); !hasCode(r, codeSharedFormulaOrphan, validate.SeverityError) {
		t.Fatalf("expected shared-formula-orphan error, got: %v", r)
	}

	// Add the master for si=3: no longer orphaned.
	ws.SheetData.Row[0].C = append(ws.SheetData.Row[0].C,
		&oxml.CT_Cell{R: "B1", F: &oxml.CT_CellFormula{T: "shared", Si: u32(3), Ref: "B1:B5", Value: "B1+1"}})
	if r := w.Validate(); hasCode(r, codeSharedFormulaOrphan, validate.SeverityError) {
		t.Fatalf("did not expect orphan after adding master, got: %v", r)
	}
}

// Duplicate sheetId is an error.
func TestValidate_DuplicateSheetID(t *testing.T) {
	wb := &oxml.CT_Workbook{Sheets: oxml.CT_Sheets{Sheet: []oxml.CT_Sheet{
		{Name: "One", SheetId: 1, RID: "rId1"},
		{Name: "Two", SheetId: 1, RID: "rId2"},
	}}}
	w := &Workbook{workbook: wb, sheets: []*Sheet{{}, {}}}
	if r := w.Validate(); !hasCode(r, codeSheetIDDup, validate.SeverityError) {
		t.Fatalf("expected sheet-id-dup error, got: %v", r)
	}
	wb.Sheets.Sheet[1].SheetId = 2
	if r := w.Validate(); hasCode(r, codeSheetIDDup, validate.SeverityError) {
		t.Fatalf("did not expect sheet-id-dup after fixing, got: %v", r)
	}
}

// definedName localSheetId out of range is an error.
func TestValidate_DefinedNameScope(t *testing.T) {
	wb := &oxml.CT_Workbook{
		Sheets:       oxml.CT_Sheets{Sheet: []oxml.CT_Sheet{{Name: "One", SheetId: 1}}},
		DefinedNames: &oxml.CT_DefinedNames{DefinedName: []oxml.CT_DefinedName{{Name: "X", LocalSheetId: u32(5)}}},
	}
	w := &Workbook{workbook: wb, sheets: []*Sheet{{}}}
	if r := w.Validate(); !hasCode(r, codeDefinedNameScope, validate.SeverityError) {
		t.Fatalf("expected defined-name-scope error, got: %v", r)
	}
}

// Overlapping merged ranges are an error (C128).
func TestValidate_MergeOverlap(t *testing.T) {
	ws := &oxml.CT_Worksheet{MergeCells: &oxml.CT_MergeCells{MergeCell: []oxml.CT_MergeCell{
		{Ref: "A1:B2"},
		{Ref: "B2:C3"}, // overlaps at B2
	}}}
	w := &Workbook{sheets: []*Sheet{{partName: "/xl/worksheets/sheet1.xml", worksheet: ws}}}
	if r := w.Validate(); !hasCode(r, codeMergeOverlap, validate.SeverityError) {
		t.Fatalf("expected merge-overlap error, got: %v", r)
	}
	// Non-overlapping ranges are clean.
	ws.MergeCells.MergeCell[1].Ref = "D4:E5"
	if r := w.Validate(); hasCode(r, codeMergeOverlap, validate.SeverityError) {
		t.Fatalf("did not expect merge-overlap for disjoint ranges, got: %v", r)
	}

	// Adjacent-but-not-overlapping ranges must NOT be flagged: the inclusive
	// intersection test only fires when the rectangles share at least one cell.
	adjacency := []struct{ a, b string }{
		{"A1:A2", "A3:A4"}, // vertically adjacent (row edge touches)
		{"A1:B1", "C1:D1"}, // horizontally adjacent (col edge touches)
		{"A1:B2", "C3:D4"}, // corner-adjacent (diagonal touch)
	}
	for _, tc := range adjacency {
		ws.MergeCells.MergeCell[0].Ref = tc.a
		ws.MergeCells.MergeCell[1].Ref = tc.b
		if r := w.Validate(); hasCode(r, codeMergeOverlap, validate.SeverityError) {
			t.Errorf("adjacent ranges %s / %s wrongly flagged as overlap: %v", tc.a, tc.b, r)
		}
	}
}
