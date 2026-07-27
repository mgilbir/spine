package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/opc"
)

// Default chart span (in cells) used when AddChart is given a single anchor
// cell rather than a range. Roughly the size of Excel's default inserted chart.
const (
	defaultChartCols = 8
	defaultChartRows = 15
)

// sheetChart is one chart anchored to a worksheet. It carries the serialized
// chart.xml (with c:f references already pointing at the dedicated data sheet
// written by AddChart) and the two-cell anchor describing its placement.
type sheetChart struct {
	def       *chart.Chart // AddChart's own copy (for the same-session reader)
	chartXML  []byte       // serialized chart.xml part
	dataSheet string       // name of the worksheet holding the chart's data
	fromCol   int          // 0-based column of the top-left anchor cell
	fromRow   int          // 0-based row of the top-left anchor cell
	toCol     int          // 0-based column of the bottom-right anchor cell
	toRow     int          // 0-based row of the bottom-right anchor cell
}

// AddChart anchors a chart on the sheet at the given position. anchor is either
// a single cell (e.g. "E2"), placing a default-sized chart with its top-left
// corner there, or a range (e.g. "E2:L20"), placing the chart to span exactly
// that block of cells.
//
// Data placement: an xlsx chart references cells in the host workbook rather
// than an embedded workbook. AddChart writes the chart's data (categories, and
// each series' name and values) into a dedicated hidden worksheet — one per
// chart — and points the chart's c:f formula references at it, so Excel's
// "Edit Data" opens the real cells while the cached values keep the chart
// rendering standalone. The host sheet's own cells are never touched, so a
// chart can sit next to unrelated data.
//
// AddChart works on both created (Create) and opened (Open/OpenReader)
// workbooks; the chart, drawing and data parts are added on the next save.
//
// The chart is copied, so the caller's *chart.Chart is left untouched (its
// DataRef keeps whatever it had) and one chart value can be added to several
// sheets, workbooks, or documents. Later edits to the caller's chart do not
// change what this sheet saves.
func (s *Sheet) AddChart(anchor string, c *chart.Chart) error {
	if c == nil {
		return fmt.Errorf("xlsx: AddChart: nil chart")
	}
	if s.workbook == nil {
		return fmt.Errorf("xlsx: AddChart: sheet is not attached to a workbook")
	}

	fromCol, fromRow, toCol, toRow, err := parseChartAnchor(anchor)
	if err != nil {
		return fmt.Errorf("xlsx: AddChart: %w", err)
	}

	// Work on a copy: pointing the references at this workbook's data sheet
	// must not rewrite the caller's chart, which may be added to another sheet
	// or another document afterwards (C562).
	c = c.Clone()

	// Point the chart's references at a dedicated hidden data worksheet, but
	// marshal the chart XML BEFORE attaching that sheet to the workbook: a
	// marshaling failure must not leave an orphan empty ChartData sheet behind.
	dataSheetName := s.workbook.uniqueChartDataSheetName()
	c.SetDataRef(dataSheetName)

	chartXML, err := c.MarshalChartXML()
	if err != nil {
		return fmt.Errorf("xlsx: AddChart: %w", err)
	}

	// Now attach the data sheet and write the chart's data into it. If the
	// write fails, remove the sheet so no orphan is left.
	dataSheet := s.workbook.addChartDataSheet()
	if werr := writeChartData(dataSheet, c); werr != nil {
		s.workbook.removeChartDataSheet(dataSheet)
		return fmt.Errorf("xlsx: AddChart: %w", werr)
	}

	s.charts = append(s.charts, sheetChart{
		def:       c,
		chartXML:  chartXML,
		dataSheet: dataSheet.name,
		fromCol:   fromCol,
		fromRow:   fromRow,
		toCol:     toCol,
		toRow:     toRow,
	})
	s.markDirty()
	return nil
}

// parseChartAnchor parses a chart anchor into a 0-based two-cell span. A single
// cell "E2" yields a default-sized span; a range "E2:L20" yields exactly that
// span.
func parseChartAnchor(anchor string) (fromCol, fromRow, toCol, toRow int, err error) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return 0, 0, 0, 0, fmt.Errorf("empty anchor")
	}
	from, to, isRange := strings.Cut(anchor, ":")
	fRow, fCol, err := ParseCellRef(from)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("anchor %q: %w", anchor, err)
	}
	if fCol > maxExcelColumns || fRow > maxExcelRows {
		return 0, 0, 0, 0, fmt.Errorf("anchor %q is out of range", anchor)
	}
	fromCol, fromRow = fCol-1, fRow-1
	if !isRange {
		// Clamp the default span to the grid maxima (0-based XFD / row 1048576)
		// so an anchor near the right/bottom edge (e.g. "XFA1") does not emit an
		// out-of-grid to-marker.
		toCol := fromCol + defaultChartCols
		if toCol > maxExcelColumns-1 {
			toCol = maxExcelColumns - 1
		}
		toRow := fromRow + defaultChartRows
		if toRow > maxExcelRows-1 {
			toRow = maxExcelRows - 1
		}
		return fromCol, fromRow, toCol, toRow, nil
	}
	tRow, tCol, err := ParseCellRef(to)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("anchor %q: %w", anchor, err)
	}
	if tCol > maxExcelColumns || tRow > maxExcelRows {
		return 0, 0, 0, 0, fmt.Errorf("anchor %q is out of range", anchor)
	}
	if tCol < fCol || tRow < fRow {
		return 0, 0, 0, 0, fmt.Errorf("anchor %q: end cell must be at or below and right of the start cell", anchor)
	}
	return fromCol, fromRow, tCol - 1, tRow - 1, nil
}

// addChartDataSheet creates a new hidden worksheet to hold one chart's data and
// returns it. The name is unique within the workbook.
func (w *Workbook) addChartDataSheet() *Sheet {
	sheet := w.AddSheet(w.uniqueChartDataSheetName())
	sheet.state = "hidden"
	// Mirror the visibility onto the workbook model so it survives a save.
	if sheet.index < len(w.workbook.Sheets.Sheet) {
		w.workbook.Sheets.Sheet[sheet.index].State = "hidden"
	}
	return sheet
}

// uniqueChartDataSheetName returns the first "ChartDataN" name not already used
// by a sheet in the workbook.
func (w *Workbook) uniqueChartDataSheetName() string {
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("ChartData%d", i)
		if !w.sheetNameExists(candidate) {
			return candidate
		}
	}
}

// removeChartDataSheet detaches a chart data sheet that was just added, undoing
// addChartDataSheet when a later step of AddChart fails so no orphan hidden
// sheet is left behind.
func (w *Workbook) removeChartDataSheet(sheet *Sheet) {
	for i, s := range w.sheets {
		if s != sheet {
			continue
		}
		w.sheets = append(w.sheets[:i], w.sheets[i+1:]...)
		for j := i; j < len(w.sheets); j++ {
			w.sheets[j].index = j
		}
		if i < len(w.workbook.Sheets.Sheet) {
			w.workbook.Sheets.Sheet = append(w.workbook.Sheets.Sheet[:i], w.workbook.Sheets.Sheet[i+1:]...)
		}
		return
	}
}

// writeChartData writes a chart's data into a worksheet using the fixed layout
// the chart's c:f references are built from. It drives the write from the same
// chart.DataCells layout source the embedded workbook uses, so every chart kind
// — including bubble, whose per-series Y/size column pairs the old
// category/scatter-only writer laid out wrong (C248) — places each value in the
// exact cell its reference points at.
func writeChartData(sheet *Sheet, c *chart.Chart) error {
	for _, dc := range c.DataCells() {
		var value interface{}
		if dc.IsText {
			value = dc.Text
		} else {
			value = dc.Number
		}
		if err := sheet.SetCellValue(chartDataCell(dc.Col, dc.Row), value); err != nil {
			return err
		}
	}
	return nil
}

// chartDataCell builds a plain A1 reference from a 1-based column and row.
func chartDataCell(col, row int) string {
	return FormatCellRef(row, col)
}

// sheetsHaveCharts reports whether any sheet carries pending charts.
func (w *Workbook) sheetsHaveCharts() bool {
	for _, sheet := range w.sheets {
		if len(sheet.charts) > 0 {
			return true
		}
	}
	return false
}

// writeSheetCharts writes each of a sheet's chart parts (xl/charts/chartN.xml)
// with collision-safe names and returns the chart relationships for the sheet's
// drawing (along with the relationship ids the drawing XML references). Chart
// relationship ids continue after the drawing's image ids (startRelN).
func (w *Workbook) writeSheetCharts(writer *opc.Writer, sheet *Sheet, used map[string]struct{}, chartSeq *int, startRelN int) ([]*opc.Relationship, []string, error) {
	rels := make([]*opc.Relationship, 0, len(sheet.charts))
	rids := make([]string, len(sheet.charts))
	relN := startRelN
	for i := range sheet.charts {
		chartPart, chartFile := allocChartName(used, chartSeq)
		if err := writer.WritePart(chartPart, opc.ContentTypeChart, sheet.charts[i].chartXML); err != nil {
			return nil, nil, err
		}
		relN++
		rid := fmt.Sprintf("rId%d", relN)
		rels = append(rels, &opc.Relationship{
			ID:     rid,
			Type:   opc.RelTypeChart,
			Target: fmt.Sprintf("../charts/%s", chartFile),
		})
		rids[i] = rid
	}
	return rels, rids, nil
}

// allocChartName finds a free /xl/charts/chartN.xml part, marking it used.
func allocChartName(used map[string]struct{}, seq *int) (partName, fileName string) {
	for {
		fileName = fmt.Sprintf("chart%d.xml", *seq)
		partName = "/xl/charts/" + fileName
		*seq++
		if _, ok := used[partName]; !ok {
			used[partName] = struct{}{}
			return partName, fileName
		}
	}
}
