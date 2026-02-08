package xlsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreezePanes(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	if err := sheet.FreezePanes("B2"); err != nil {
		t.Fatal(err)
	}

	sv := sheet.worksheet.SheetViews
	if sv == nil || len(sv.SheetView) == 0 {
		t.Fatal("expected SheetViews to be set")
	}

	pane := sv.SheetView[0].Pane
	if pane == nil {
		t.Fatal("expected Pane to be set")
	}
	if pane.State != "frozen" {
		t.Errorf("State = %s, want frozen", pane.State)
	}
	if pane.TopLeftCell != "B2" {
		t.Errorf("TopLeftCell = %s, want B2", pane.TopLeftCell)
	}
	if pane.XSplit == nil || *pane.XSplit != 1 {
		t.Errorf("XSplit = %v, want 1", pane.XSplit)
	}
	if pane.YSplit == nil || *pane.YSplit != 1 {
		t.Errorf("YSplit = %v, want 1", pane.YSplit)
	}
	if pane.ActivePane != "bottomRight" {
		t.Errorf("ActivePane = %s, want bottomRight", pane.ActivePane)
	}

	// Selection should be set
	if len(sv.SheetView[0].Selection) != 1 {
		t.Fatalf("expected 1 selection, got %d", len(sv.SheetView[0].Selection))
	}
}

func TestFreezePanesRowOnly(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	if err := sheet.FreezePanes("A3"); err != nil {
		t.Fatal(err)
	}

	pane := sheet.worksheet.SheetViews.SheetView[0].Pane
	if pane.XSplit != nil {
		t.Errorf("XSplit should be nil for row-only freeze, got %v", *pane.XSplit)
	}
	if pane.YSplit == nil || *pane.YSplit != 2 {
		t.Errorf("YSplit = %v, want 2", pane.YSplit)
	}
	if pane.ActivePane != "bottomLeft" {
		t.Errorf("ActivePane = %s, want bottomLeft", pane.ActivePane)
	}
}

func TestFreezePanesColOnly(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	if err := sheet.FreezePanes("C1"); err != nil {
		t.Fatal(err)
	}

	pane := sheet.worksheet.SheetViews.SheetView[0].Pane
	if pane.XSplit == nil || *pane.XSplit != 2 {
		t.Errorf("XSplit = %v, want 2", pane.XSplit)
	}
	if pane.YSplit != nil {
		t.Errorf("YSplit should be nil for col-only freeze, got %v", *pane.YSplit)
	}
	if pane.ActivePane != "topRight" {
		t.Errorf("ActivePane = %s, want topRight", pane.ActivePane)
	}
}

func TestUnfreezePanes(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	_ = sheet.FreezePanes("B2")
	sheet.UnfreezePanes()

	if sheet.worksheet.SheetViews.SheetView[0].Pane != nil {
		t.Error("expected Pane to be nil after unfreeze")
	}
}

func TestSetZoom(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	sheet.SetZoom(150)

	sv := sheet.worksheet.SheetViews.SheetView[0]
	if sv.ZoomScale == nil || *sv.ZoomScale != 150 {
		t.Errorf("ZoomScale = %v, want 150", sv.ZoomScale)
	}
	if sv.ZoomScaleNormal == nil || *sv.ZoomScaleNormal != 150 {
		t.Errorf("ZoomScaleNormal = %v, want 150", sv.ZoomScaleNormal)
	}
}

func TestSetShowGridLines(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	sheet.SetShowGridLines(false)

	sv := sheet.worksheet.SheetViews.SheetView[0]
	if sv.ShowGridLines == nil || *sv.ShowGridLines != false {
		t.Errorf("ShowGridLines = %v, want false", sv.ShowGridLines)
	}
}

func TestSetTabColor(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	sheet.SetTabColor("FF0000")

	if sheet.worksheet.SheetPr == nil || sheet.worksheet.SheetPr.TabColor == nil {
		t.Fatal("expected TabColor to be set")
	}
	if sheet.worksheet.SheetPr.TabColor.Rgb != "FF0000" {
		t.Errorf("Rgb = %s, want FF0000", sheet.worksheet.SheetPr.TabColor.Rgb)
	}
}

func TestSetAutoFilter(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	if err := sheet.SetAutoFilter("a1:f1"); err != nil {
		t.Fatal(err)
	}

	if sheet.worksheet.AutoFilter == nil {
		t.Fatal("expected AutoFilter to be set")
	}
	if sheet.worksheet.AutoFilter.Ref != "A1:F1" {
		t.Errorf("Ref = %s, want A1:F1", sheet.worksheet.AutoFilter.Ref)
	}
}

func TestRemoveAutoFilter(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	_ = sheet.SetAutoFilter("A1:F1")
	sheet.RemoveAutoFilter()

	if sheet.worksheet.AutoFilter != nil {
		t.Error("expected AutoFilter to be nil")
	}
}

func TestAddDataValidation(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	err := sheet.AddDataValidation(DataValidation{
		Range:        "c2:c100",
		Type:         "list",
		Formula1:     `"Yes,No,Maybe"`,
		ShowDropDown: true,
		AllowBlank:   true,
		ErrorTitle:   "Invalid",
		ErrorMessage: "Please select from the list",
	})
	if err != nil {
		t.Fatal(err)
	}

	dv := sheet.worksheet.DataValidations
	if dv == nil {
		t.Fatal("expected DataValidations to be set")
	}
	if dv.Count == nil || *dv.Count != 1 {
		t.Errorf("Count = %v, want 1", dv.Count)
	}

	v := dv.DataValidation[0]
	if v.Sqref != "C2:C100" {
		t.Errorf("Sqref = %s, want C2:C100", v.Sqref)
	}
	if v.Type != "list" {
		t.Errorf("Type = %s, want list", v.Type)
	}
	if v.Formula1 == nil || *v.Formula1 != `"Yes,No,Maybe"` {
		t.Errorf("Formula1 = %v, want \"Yes,No,Maybe\"", v.Formula1)
	}
	if v.AllowBlank == nil || *v.AllowBlank != true {
		t.Errorf("AllowBlank = %v, want true", v.AllowBlank)
	}
	// ShowDropDown in OOXML is counterintuitive: false = show
	if v.ShowDropDown == nil || *v.ShowDropDown != false {
		t.Errorf("ShowDropDown = %v, want false (OOXML semantics)", v.ShowDropDown)
	}
	if v.ErrorTitle != "Invalid" {
		t.Errorf("ErrorTitle = %s, want Invalid", v.ErrorTitle)
	}
	if v.Error != "Please select from the list" {
		t.Errorf("Error = %s, want 'Please select from the list'", v.Error)
	}
}

func TestSheetViewSaveAndReopen(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")

	// Set various view properties
	_ = sheet.FreezePanes("B2")
	sheet.SetZoom(125)
	sheet.SetShowGridLines(false)
	_ = sheet.SetAutoFilter("A1:D1")

	// Set cell values
	cell, _ := sheet.Cell("A1")
	cell.SetValue("Header")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sheetview.xlsx")
	if err := wb.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Reopen and verify
	wb2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer wb2.Close() //nolint:errcheck

	if wb2.SheetCount() != 1 {
		t.Fatalf("expected 1 sheet, got %d", wb2.SheetCount())
	}

	s2, _ := wb2.Sheet(0)
	val, _ := s2.GetCellValue("A1")
	if val != "Header" {
		t.Errorf("cell A1 = %s, want Header", val)
	}
}
