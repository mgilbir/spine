package xlsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStyleManager_NewCellStyle_Font(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Font: &FontStyle{Name: "Arial", Size: 14, Bold: true, Color: "FF0000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if idx == 0 {
		t.Error("expected non-zero index for custom style")
	}

	// Retrieve and verify
	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Font == nil {
		t.Fatal("expected font to be set")
	}
	if cs.Font.Name != "Arial" {
		t.Errorf("expected Arial, got %s", cs.Font.Name)
	}
	if cs.Font.Size != 14 {
		t.Errorf("expected size 14, got %f", cs.Font.Size)
	}
	if !cs.Font.Bold {
		t.Error("expected bold")
	}
	if cs.Font.Color != "FF0000" {
		t.Errorf("expected color FF0000, got %s", cs.Font.Color)
	}
}

func TestStyleManager_NewCellStyle_Fill(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Fill: &FillStyle{Pattern: "solid", FgColor: "4472C4"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Fill == nil {
		t.Fatal("expected fill to be set")
	}
	if cs.Fill.Pattern != "solid" {
		t.Errorf("expected solid, got %s", cs.Fill.Pattern)
	}
	if cs.Fill.FgColor != "4472C4" {
		t.Errorf("expected 4472C4, got %s", cs.Fill.FgColor)
	}
}

func TestStyleManager_NewCellStyle_Border(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Border: &BorderStyle{
			Bottom: &BorderSide{Style: "thin", Color: "000000"},
			Top:    &BorderSide{Style: "medium", Color: "FF0000"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Border == nil {
		t.Fatal("expected border to be set")
	}
	if cs.Border.Bottom == nil {
		t.Fatal("expected bottom border")
	}
	if cs.Border.Bottom.Style != "thin" {
		t.Errorf("expected thin, got %s", cs.Border.Bottom.Style)
	}
	if cs.Border.Bottom.Color != "000000" {
		t.Errorf("expected 000000, got %s", cs.Border.Bottom.Color)
	}
	if cs.Border.Top == nil {
		t.Fatal("expected top border")
	}
	if cs.Border.Top.Style != "medium" {
		t.Errorf("expected medium, got %s", cs.Border.Top.Style)
	}
}

func TestStyleManager_NewCellStyle_Alignment(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Alignment: &AlignmentStyle{
			Horizontal: "center",
			Vertical:   "bottom",
			WrapText:   true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Alignment == nil {
		t.Fatal("expected alignment to be set")
	}
	if cs.Alignment.Horizontal != "center" {
		t.Errorf("expected center, got %s", cs.Alignment.Horizontal)
	}
	if cs.Alignment.Vertical != "bottom" {
		t.Errorf("expected bottom, got %s", cs.Alignment.Vertical)
	}
	if !cs.Alignment.WrapText {
		t.Error("expected wrap text")
	}
}

func TestStyleManager_NumberFormat_Builtin(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{Format: "0.00"})
	if err != nil {
		t.Fatal(err)
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Format != "0.00" {
		t.Errorf("expected 0.00, got %s", cs.Format)
	}
}

func TestStyleManager_NumberFormat_Custom(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	customFmt := `#,##0.00" USD"`
	idx, err := sm.NewCellStyle(CellStyle{Format: customFmt})
	if err != nil {
		t.Fatal(err)
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Format != customFmt {
		t.Errorf("expected %s, got %s", customFmt, cs.Format)
	}
}

func TestStyleManager_Deduplication(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	style := CellStyle{
		Font: &FontStyle{Name: "Arial", Size: 12, Bold: true},
		Fill: &FillStyle{Pattern: "solid", FgColor: "FFFF00"},
	}

	idx1, err := sm.NewCellStyle(style)
	if err != nil {
		t.Fatal(err)
	}
	idx2, err := sm.NewCellStyle(style)
	if err != nil {
		t.Fatal(err)
	}

	if idx1 != idx2 {
		t.Errorf("expected same index for identical styles, got %d and %d", idx1, idx2)
	}
}

func TestCell_SetStyle(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Test")

	cell, err := sheet.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	cell.SetValue("Hello")

	err = cell.SetStyle(CellStyle{
		Font: &FontStyle{Name: "Calibri", Size: 12, Bold: true, Color: "FFFFFF"},
		Fill: &FillStyle{Pattern: "solid", FgColor: "4472C4"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify style index is set
	si := cell.StyleIndex()
	if si == nil {
		t.Fatal("expected style index to be set")
	}
	if *si == 0 {
		t.Error("expected non-zero style index")
	}
}

func TestStyleManager_CompleteStyle(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Font:      &FontStyle{Name: "Calibri", Size: 12, Bold: true, Italic: true, Underline: true, Color: "FFFFFF"},
		Fill:      &FillStyle{Pattern: "solid", FgColor: "4472C4"},
		Border:    &BorderStyle{Bottom: &BorderSide{Style: "thin", Color: "000000"}},
		Alignment: &AlignmentStyle{Horizontal: "center", WrapText: true},
		Format:    "#,##0.00",
	})
	if err != nil {
		t.Fatal(err)
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}

	if cs.Font == nil || !cs.Font.Bold || !cs.Font.Italic || !cs.Font.Underline {
		t.Error("font properties not preserved")
	}
	if cs.Fill == nil || cs.Fill.FgColor != "4472C4" {
		t.Error("fill not preserved")
	}
	if cs.Border == nil || cs.Border.Bottom == nil {
		t.Error("border not preserved")
	}
	if cs.Alignment == nil || cs.Alignment.Horizontal != "center" || !cs.Alignment.WrapText {
		t.Error("alignment not preserved")
	}
	if cs.Format != "#,##0.00" {
		t.Errorf("format not preserved, got %s", cs.Format)
	}
}

func TestStyle_SaveAndReopen(t *testing.T) {
	// Create workbook with styled cells
	wb := Create()
	sheet := wb.AddSheet("Styled")

	cell, err := sheet.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	cell.SetValue("Header")
	if err := cell.SetStyle(CellStyle{
		Font: &FontStyle{Name: "Arial", Size: 14, Bold: true, Color: "FFFFFF"},
		Fill: &FillStyle{Pattern: "solid", FgColor: "4472C4"},
		Alignment: &AlignmentStyle{Horizontal: "center"},
	}); err != nil {
		t.Fatal(err)
	}

	cell2, err := sheet.Cell("B2")
	if err != nil {
		t.Fatal(err)
	}
	cell2.SetValue(42195.50)
	if err := cell2.SetStyle(CellStyle{
		Format: "#,##0.00",
		Border: &BorderStyle{Bottom: &BorderSide{Style: "thin", Color: "000000"}},
	}); err != nil {
		t.Fatal(err)
	}

	// Save
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "styled.xlsx")
	if err := wb.Save(path); err != nil {
		t.Fatal(err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Reopen and verify styles
	wb2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer wb2.Close() //nolint:errcheck

	sm := wb2.Styles()

	// Check cell A1 style
	s1, err := wb2.SheetByName("Styled")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := s1.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	if c1.StyleIndex() == nil {
		t.Fatal("expected style index on A1")
	}
	cs1, err := sm.GetCellStyle(*c1.StyleIndex())
	if err != nil {
		t.Fatal(err)
	}
	if cs1.Font == nil {
		t.Fatal("expected font on A1")
	}
	if !cs1.Font.Bold {
		t.Error("expected bold on A1")
	}
	if cs1.Font.Name != "Arial" {
		t.Errorf("expected Arial, got %s", cs1.Font.Name)
	}
	if cs1.Fill == nil {
		t.Fatal("expected fill on A1")
	}
	if cs1.Fill.FgColor != "4472C4" {
		t.Errorf("expected fill color 4472C4, got %s", cs1.Fill.FgColor)
	}

	// Check cell B2 style
	c2, err := s1.Cell("B2")
	if err != nil {
		t.Fatal(err)
	}
	if c2.StyleIndex() == nil {
		t.Fatal("expected style index on B2")
	}
	cs2, err := sm.GetCellStyle(*c2.StyleIndex())
	if err != nil {
		t.Fatal(err)
	}
	if cs2.Format != "#,##0.00" {
		t.Errorf("expected format #,##0.00, got %s", cs2.Format)
	}
	if cs2.Border == nil || cs2.Border.Bottom == nil {
		t.Fatal("expected bottom border on B2")
	}
	if cs2.Border.Bottom.Style != "thin" {
		t.Errorf("expected thin border, got %s", cs2.Border.Bottom.Style)
	}
}

func TestNormalizeHexColor(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"FF0000", "FFFF0000"},
		{"ff0000", "FFFF0000"},
		{"#FF0000", "FFFF0000"},
		{"FFFF0000", "FFFF0000"},
		{"4472C4", "FF4472C4"},
	}

	for _, tt := range tests {
		got := normalizeHexColor(tt.input)
		if got != tt.want {
			t.Errorf("normalizeHexColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripAlphaFromRGB(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"FFFF0000", "FF0000"},
		{"FF4472C4", "4472C4"},
		{"FF0000", "FF0000"},
	}

	for _, tt := range tests {
		got := stripAlphaFromRGB(tt.input)
		if got != tt.want {
			t.Errorf("stripAlphaFromRGB(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// C133: negative indent/rotation must be rejected instead of wrapping to huge
// unsigned values; rotation is limited to 0-180 or the special 255.
func TestNewCellStyle_RejectsInvalidAlignment(t *testing.T) {
	sm := newStyleManager(nil, nil)

	if _, err := sm.NewCellStyle(CellStyle{Alignment: &AlignmentStyle{Indent: -1}}); err == nil {
		t.Error("negative indent accepted")
	}
	if _, err := sm.NewCellStyle(CellStyle{Alignment: &AlignmentStyle{Rotation: -90}}); err == nil {
		t.Error("negative rotation accepted")
	}
	if _, err := sm.NewCellStyle(CellStyle{Alignment: &AlignmentStyle{Rotation: 200}}); err == nil {
		t.Error("rotation 200 accepted (valid range is 0-180 or 255)")
	}
	if _, err := sm.NewCellStyle(CellStyle{Alignment: &AlignmentStyle{Rotation: 255}}); err != nil {
		t.Errorf("rotation 255 (vertical text) rejected: %v", err)
	}
	if _, err := sm.NewCellStyle(CellStyle{Alignment: &AlignmentStyle{Indent: 2, Rotation: 45}}); err != nil {
		t.Errorf("valid alignment rejected: %v", err)
	}
}
