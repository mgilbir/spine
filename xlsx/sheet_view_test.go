package xlsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreezePanes(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	if err := sheet.FreezePanes("B2"); err != nil {
		t.Fatal(err)
	}

	sv := sheet.ws().SheetViews
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
	sheet := addSheetT(wb, "Sheet1")

	if err := sheet.FreezePanes("A3"); err != nil {
		t.Fatal(err)
	}

	pane := sheet.ws().SheetViews.SheetView[0].Pane
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
	sheet := addSheetT(wb, "Sheet1")

	if err := sheet.FreezePanes("C1"); err != nil {
		t.Fatal(err)
	}

	pane := sheet.ws().SheetViews.SheetView[0].Pane
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

// C133: FreezePanes("a1") emitted a frozen pane with no splits, no
// activePane, and a lowercase selection ref. A1 freezes nothing, so it must
// remove the pane; a lowercase ref must be canonicalized.
func TestFreezePanesA1RemovesPane(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	// Freeze then re-freeze at A1: pane and pane-scoped selections must go.
	if err := sheet.FreezePanes("B2"); err != nil {
		t.Fatal(err)
	}
	if err := sheet.FreezePanes("a1"); err != nil {
		t.Fatal(err)
	}
	sv := sheet.ws().SheetViews.SheetView[0]
	if sv.Pane != nil {
		t.Errorf("Pane = %+v, want nil for A1 freeze", sv.Pane)
	}
	for _, sel := range sv.Selection {
		if sel.Pane != "" {
			t.Errorf("pane-scoped selection %+v left behind after pane removal", sel)
		}
	}

	// A freshly created sheet frozen at A1 stays pane-free too.
	sheet2 := addSheetT(wb, "Sheet2")
	if err := sheet2.FreezePanes("A1"); err != nil {
		t.Fatal(err)
	}
	if sheet2.ws() != nil && sheet2.ws().SheetViews != nil &&
		len(sheet2.ws().SheetViews.SheetView) > 0 &&
		sheet2.ws().SheetViews.SheetView[0].Pane != nil {
		t.Error("A1 freeze on a fresh sheet created a pane")
	}
}

// C133: a lowercase freeze ref must produce canonical TopLeftCell and
// selection references.
func TestFreezePanesCanonicalizesRef(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	if err := sheet.FreezePanes("b2"); err != nil {
		t.Fatal(err)
	}
	sv := sheet.ws().SheetViews.SheetView[0]
	if sv.Pane == nil || sv.Pane.TopLeftCell != "B2" {
		t.Fatalf("TopLeftCell = %+v, want B2", sv.Pane)
	}
	if sv.Pane.ActivePane != "bottomRight" {
		t.Errorf("ActivePane = %q, want bottomRight", sv.Pane.ActivePane)
	}
	if len(sv.Selection) != 1 || sv.Selection[0].ActiveCell != "B2" || sv.Selection[0].SqRef != "B2" {
		t.Errorf("Selection = %+v, want canonical B2 refs", sv.Selection)
	}
	if sv.Selection[0].Pane != "bottomRight" {
		t.Errorf("Selection pane = %q, want bottomRight", sv.Selection[0].Pane)
	}
}

func TestUnfreezePanes(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	_ = sheet.FreezePanes("B2")
	sheet.UnfreezePanes()

	if sheet.ws().SheetViews.SheetView[0].Pane != nil {
		t.Error("expected Pane to be nil after unfreeze")
	}
}

func TestSetZoom(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	sheet.SetZoom(150)

	sv := sheet.ws().SheetViews.SheetView[0]
	if sv.ZoomScale == nil || *sv.ZoomScale != 150 {
		t.Errorf("ZoomScale = %v, want 150", sv.ZoomScale)
	}
	if sv.ZoomScaleNormal == nil || *sv.ZoomScaleNormal != 150 {
		t.Errorf("ZoomScaleNormal = %v, want 150", sv.ZoomScaleNormal)
	}
}

func TestSetShowGridLines(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	sheet.SetShowGridLines(false)

	sv := sheet.ws().SheetViews.SheetView[0]
	if sv.ShowGridLines == nil || *sv.ShowGridLines != false {
		t.Errorf("ShowGridLines = %v, want false", sv.ShowGridLines)
	}
}

func TestSetTabColor(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	sheet.SetTabColor("FF0000")

	if sheet.ws().SheetPr == nil || sheet.ws().SheetPr.TabColor == nil {
		t.Fatal("expected TabColor to be set")
	}
	if sheet.ws().SheetPr.TabColor.Rgb != "FF0000" {
		t.Errorf("Rgb = %s, want FF0000", sheet.ws().SheetPr.TabColor.Rgb)
	}
}

func TestSetAutoFilter(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	if err := sheet.SetAutoFilter("a1:f1"); err != nil {
		t.Fatal(err)
	}

	if sheet.ws().AutoFilter == nil {
		t.Fatal("expected AutoFilter to be set")
	}
	if sheet.ws().AutoFilter.Ref != "A1:F1" {
		t.Errorf("Ref = %s, want A1:F1", sheet.ws().AutoFilter.Ref)
	}
}

func TestRemoveAutoFilter(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	_ = sheet.SetAutoFilter("A1:F1")
	sheet.RemoveAutoFilter()

	if sheet.ws().AutoFilter != nil {
		t.Error("expected AutoFilter to be nil")
	}
}

func TestAddDataValidation(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	err := sheet.AddDataValidation(DataValidation{
		Range:         "c2:c100",
		Type:          "list",
		Formula1:      `"Yes,No,Maybe"`,
		AllowBlank:    true,
		ErrorTitle:    "Invalid",
		ErrorMessage:  "Please select from the list",
		PromptTitle:   "Pick one",
		PromptMessage: "Choose Yes, No or Maybe",
	})
	if err != nil {
		t.Fatal(err)
	}

	dv := sheet.ws().DataValidations
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
	// C76: with the dropdown not hidden, the suppressing showDropDown
	// attribute must be absent (Excel shows the dropdown by default).
	if v.ShowDropDown != nil {
		t.Errorf("ShowDropDown = %v, want nil (attribute absent)", *v.ShowDropDown)
	}
	if v.ErrorTitle != "Invalid" {
		t.Errorf("ErrorTitle = %s, want Invalid", v.ErrorTitle)
	}
	if v.Error != "Please select from the list" {
		t.Errorf("Error = %s, want 'Please select from the list'", v.Error)
	}
	// C76: error text without showErrorMessage="1" is never displayed.
	if v.ShowErrorMessage == nil || !*v.ShowErrorMessage {
		t.Errorf("ShowErrorMessage = %v, want true (error text present)", v.ShowErrorMessage)
	}
	if v.PromptTitle != "Pick one" || v.Prompt != "Choose Yes, No or Maybe" {
		t.Errorf("prompt = %q/%q, want set", v.PromptTitle, v.Prompt)
	}
	if v.ShowInputMessage == nil || !*v.ShowInputMessage {
		t.Errorf("ShowInputMessage = %v, want true (prompt text present)", v.ShowInputMessage)
	}
}

// C76: HideDropDown must emit showDropDown="1" (the attribute means SUPPRESS
// the dropdown), and a rule without error/prompt text must not emit the
// show* attributes.
func TestAddDataValidationHideDropDown(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

	err := sheet.AddDataValidation(DataValidation{
		Range:        "B2:B10",
		Type:         "list",
		Formula1:     `"a,b"`,
		HideDropDown: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	v := sheet.ws().DataValidations.DataValidation[0]
	if v.ShowDropDown == nil || !*v.ShowDropDown {
		t.Errorf("ShowDropDown = %v, want true (showDropDown=\"1\" suppresses the dropdown)", v.ShowDropDown)
	}
	if v.ShowErrorMessage != nil {
		t.Errorf("ShowErrorMessage = %v, want nil (no error text)", *v.ShowErrorMessage)
	}
	if v.ShowInputMessage != nil {
		t.Errorf("ShowInputMessage = %v, want nil (no prompt text)", *v.ShowInputMessage)
	}
}

func TestSheetViewSaveAndReopen(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")

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
