package xlsxrender

import (
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/render"
	"github.com/mgilbir/spine/xlsx"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
	"github.com/mgilbir/spine/xlsx/internal/view"
)

const nsSML = xmlb.NSSpreadsheetML

// PrepareRange prepares an explicit cell range as a native PNG/SVG snapshot.
// The initial profile uses the default stylesheet, explicit column widths, plain
// ASCII strings, booleans and General integers of at most nine decimal digits.
// Formula cells require a supported cached value; no formula is evaluated.
// Rich styling, merged cells, drawings and unsupported sheet features fail.
// Missing cells remain absent. This is a range preview, not print pagination.
func PrepareRange(ctx context.Context, sheet *xlsx.Sheet, ref string, opts render.Options) (*render.Page, error) {
	if ctx == nil || sheet == nil {
		return nil, fmt.Errorf("%w: worksheet", render.ErrInvalid)
	}
	s := view.SheetOf(sheet)
	if s == nil || s.Opaque {
		return nil, fmt.Errorf("%w: worksheet", render.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(ref) > 32 {
		return nil, fmt.Errorf("%w: range reference", render.ErrInvalid)
	}
	minRow, minCol, maxRow, maxCol, err := view.ParseRange(ref)
	if err != nil {
		return nil, fmt.Errorf("%w: range", render.ErrInvalid)
	}
	limits, err := core.ResolveLimits(opts.Limits)
	if err != nil {
		return nil, err
	}
	b, err := core.NewSourceBudget(opts.MaxSourceBytes, opts.MaxLayoutNodes)
	if err != nil {
		return nil, err
	}
	nr, nc := maxRow-minRow+1, maxCol-minCol+1
	if nr > b.Nodes || nc > b.Nodes/nr || nr+nc+3 > limits.MaxOperations {
		return nil, fmt.Errorf("%w: range cells", render.ErrLimit)
	}
	rels := s.Relationships
	if len(rels) > b.Nodes {
		return nil, render.ErrLimit
	}
	b.Nodes -= len(rels)
	seen := map[string]bool{}
	for _, rel := range rels {
		if rel == nil {
			return nil, render.ErrInvalid
		}
		var expected string
		if rel.Type == opc.RelTypeStyles {
			expected = "/xl/styles.xml"
		}
		if rel.Type == opc.RelTypeSharedStrings {
			expected = "/xl/sharedStrings.xml"
		}
		if expected != "" {
			if seen[rel.Type] || rel.TargetMode == opc.TargetModeExternal || opc.ResolvePartName(s.WorkbookPart, rel.Target) != expected {
				return nil, fmt.Errorf("%w: style/string relationship", render.ErrUnsupported)
			}
			seen[rel.Type] = true
		}
	}
	for _, part := range []struct{ name, root string }{{s.PartName, "worksheet"}, {"/xl/styles.xml", "styleSheet"}, {"/xl/sharedStrings.xml", "sst"}} {
		if p := s.PreservedParts[part.name]; p != nil {
			if err = b.CheckXML(ctx, p.Data, func(n core.XMLNode) error { return sheetRenderXML(n, part.root) }); err != nil {
				return nil, fmt.Errorf("xlsx: %s: %w", part.name, err)
			}
		}
	}
	ws := s.Load()
	if ws == nil {
		return nil, fmt.Errorf("%w: missing worksheet", render.ErrInvalid)
	}
	if err = renderWorksheetProfile(ws); err != nil {
		return nil, err
	}
	ss := s.Stylesheet
	if ss == nil {
		ss = view.DefaultStylesheet()
	}
	copyStyles := *ss
	copyStyles.XMLName = xml.Name{}
	copyStyles.OriginalNSDecls = nil
	copyStyles.OriginalRootAttrs = nil
	if !reflect.DeepEqual(&copyStyles, view.DefaultStylesheet()) {
		return nil, fmt.Errorf("%w: default stylesheet required", render.ErrUnsupported)
	}
	if opts.Fonts == nil {
		return nil, fmt.Errorf("%w: explicit font resolver required", render.ErrUnsupported)
	}
	face, err := opts.Fonts(ctx, render.FontRequest{Family: "Calibri"})
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	breaker, err := core.NewTextLayout(limits)
	if err != nil {
		return nil, err
	}
	fontSize, _ := style.FromPx(11 * 4.0 / 3)
	digitWidth := 0.0
	for digit := '0'; digit <= '9'; digit++ {
		lines, e := breaker.PlainLines(ctx, face, string(digit), fontSize, sheetRenderUnit(8192))
		if e != nil {
			return nil, e
		}
		digitWidth = max(digitWidth, lines[0].Width.Px())
	}
	digitWidth = math.Round(digitWidth)
	if digitWidth < 1 || digitWidth > 8192 {
		return nil, fmt.Errorf("%w: digit width", render.ErrUnsupported)
	}
	// All source rows/cells/column entries are charged before indexing them.
	if len(ws.SheetData.Row) > b.Nodes {
		return nil, render.ErrLimit
	}
	b.Nodes -= len(ws.SheetData.Row)
	rows := make(map[int]*oxml.CT_Row)
	for i := range ws.SheetData.Row {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		r := &ws.SheetData.Row[i]
		if r.R == nil || *r.R == 0 || *r.R > xlsx.MaxRow || rows[int(*r.R)] != nil {
			return nil, fmt.Errorf("%w: row reference", render.ErrInvalid)
		}
		if len(r.C) > b.Nodes {
			return nil, render.ErrLimit
		}
		b.Nodes -= len(r.C)
		if len(r.ExtRaw) > 0 || r.S != nil || r.CustomFormat != nil || r.ThickTop != nil || r.ThickBot != nil || r.Ph != nil {
			return nil, fmt.Errorf("%w: row formatting", render.ErrUnsupported)
		}
		rows[int(*r.R)] = r
	}
	widths := make([]float64, nc)
	configured := make([]bool, nc)
	for _, group := range ws.Cols {
		if len(group.Col) > b.Nodes {
			return nil, render.ErrLimit
		}
		b.Nodes -= len(group.Col)
		for _, col := range group.Col {
			if col.Min == 0 || col.Max < col.Min || col.Max > xlsx.MaxCol {
				return nil, fmt.Errorf("%w: column range", render.ErrInvalid)
			}
			if col.Style != nil || col.BestFit != nil || col.Phonetic != nil || col.OutlineLevel != nil || col.Collapsed != nil {
				return nil, fmt.Errorf("%w: column style", render.ErrUnsupported)
			}
			for c := max(int(col.Min), minCol); c <= min(int(col.Max), maxCol); c++ {
				ci := c - minCol
				if configured[ci] {
					return nil, fmt.Errorf("%w: overlapping columns", render.ErrInvalid)
				}
				configured[ci] = true
				if col.Hidden != nil && *col.Hidden {
					continue
				}
				if col.Width == nil || math.IsNaN(*col.Width) || math.IsInf(*col.Width, 0) || *col.Width <= 0 || *col.Width > 255 {
					return nil, fmt.Errorf("%w: explicit column width required", render.ErrUnsupported)
				}
				widths[ci] = math.Trunc((256**col.Width + math.Trunc(128/digitWidth)) / 256 * digitWidth)
			}
		}
	}
	xs := make([]float64, nc+1)
	ys := make([]float64, nr+1)
	for i, w := range widths {
		if !configured[i] {
			return nil, fmt.Errorf("%w: explicit column width required", render.ErrUnsupported)
		}
		xs[i+1] = xs[i] + w
		if _, ok := style.FromPx(xs[i+1]); !ok {
			return nil, render.ErrLimit
		}
		if xs[i+1] > float64(limits.MaxDimension) {
			return nil, render.ErrLimit
		}
	}
	defaultHeight := 15.0
	if ws.SheetFormatPr != nil {
		f := ws.SheetFormatPr
		if f.ZeroHeight != nil && *f.ZeroHeight {
			return nil, fmt.Errorf("%w: default hidden rows", render.ErrUnsupported)
		}
		defaultHeight = f.DefaultRowHeight
	}
	for i := 0; i < nr; i++ {
		height := defaultHeight
		if row := rows[minRow+i]; row != nil {
			if row.Ht != nil {
				height = *row.Ht
			}
			if row.Hidden != nil && *row.Hidden {
				height = 0
			}
		}
		if math.IsNaN(height) || math.IsInf(height, 0) || height < 0 || height > 4096 {
			return nil, fmt.Errorf("%w: row height", render.ErrInvalid)
		}
		ys[i+1] = ys[i] + height*4/3
		if _, ok := style.FromPx(ys[i+1]); !ok {
			return nil, render.ErrLimit
		}
		if ys[i+1] > float64(limits.MaxDimension) {
			return nil, render.ErrLimit
		}
	}
	if xs[nc] <= 0 || ys[nr] <= 0 {
		return nil, fmt.Errorf("%w: empty visible range", render.ErrInvalid)
	}
	if xs[nc]*ys[nr] > float64(limits.MaxPixels) {
		return nil, render.ErrLimit
	}
	ops := []layout.Op{layout.FillRect{Rect: layout.Rect{W: sheetRenderUnit(xs[nc]), H: sheetRenderUnit(ys[nr])}, Color: style.RGBA{R: 255, G: 255, B: 255, A: 1}}}
	grid := true
	if ws.SheetViews != nil && len(ws.SheetViews.SheetView) > 0 {
		view := ws.SheetViews.SheetView[0]
		if (view.ShowFormulas != nil && *view.ShowFormulas) || (view.RightToLeft != nil && *view.RightToLeft) || view.ColorId != nil {
			return nil, fmt.Errorf("%w: worksheet view", render.ErrUnsupported)
		}
		if view.ShowGridLines != nil {
			grid = *view.ShowGridLines
		}
	}
	if grid {
		gray := style.RGBA{R: 210, G: 210, B: 210, A: 1}
		for _, x := range xs {
			ops = append(ops, layout.FillRect{Rect: layout.Rect{X: sheetRenderUnit(min(x, xs[nc]-1)), W: sheetRenderUnit(1), H: sheetRenderUnit(ys[nr])}, Color: gray})
		}
		for _, y := range ys {
			ops = append(ops, layout.FillRect{Rect: layout.Rect{Y: sheetRenderUnit(min(y, ys[nr]-1)), H: sheetRenderUnit(1), W: sheetRenderUnit(xs[nc])}, Color: gray})
		}
	}
	for ri := 0; ri < nr; ri++ {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		row := rows[minRow+ri]
		if row == nil || ys[ri+1] == ys[ri] {
			continue
		}
		cells := make([]*oxml.CT_Cell, nc)
		for _, cell := range row.C {
			if cell == nil {
				return nil, render.ErrInvalid
			}
			r, c, ok := cell.RowCol()
			if !ok || r != minRow+ri {
				return nil, fmt.Errorf("%w: cell reference", render.ErrInvalid)
			}
			if c < minCol || c > maxCol {
				continue
			}
			if cells[c-minCol] != nil {
				return nil, fmt.Errorf("%w: duplicate cell", render.ErrInvalid)
			}
			cells[c-minCol] = cell
		}
		end := xs[nc]
		for ci := nc - 1; ci >= 0; ci-- {
			if widths[ci] == 0 {
				continue
			}
			cell := cells[ci]
			if cell == nil {
				continue
			}
			if styleID, _ := cell.StyleIndex(); styleID != 0 {
				return nil, fmt.Errorf("%w: cell style", render.ErrUnsupported)
			}
			if cell.Cm() != nil || cell.Vm() != nil || cell.Ph() != nil || len(cell.ExtRaw()) > 0 {
				return nil, fmt.Errorf("%w: cell metadata", render.ErrUnsupported)
			}
			text, align, e := renderCellText(s, cell)
			if e != nil {
				return nil, e
			}
			if text == "" {
				continue
			}
			room := end - xs[ci] - 4
			if align != "l" {
				room = widths[ci] - 4
			}
			if room <= 0 {
				return nil, fmt.Errorf("%w: cell text room", render.ErrUnsupported)
			}
			lines, e := breaker.PlainLines(ctx, face, text, fontSize, sheetRenderUnit(room))
			if e != nil {
				return nil, e
			}
			if len(lines) != 1 {
				return nil, fmt.Errorf("%w: cell text overflow", render.ErrUnsupported)
			}
			line := lines[0]
			metrics := line.Face.Descriptor()
			em := float64(line.Face.UnitsPerEm())
			ascent := float64(metrics.Ascent) * fontSize.Px() / em
			descent := -float64(metrics.Descent) * fontSize.Px() / em
			if ascent <= 0 || descent < 0 || ascent+descent+4 > ys[ri+1]-ys[ri] {
				return nil, fmt.Errorf("%w: cell vertical overflow", render.ErrUnsupported)
			}
			x := xs[ci] + 2
			if align == "r" {
				x = xs[ci+1] - 2 - line.Width.Px()
			}
			if align == "c" {
				x = xs[ci] + (widths[ci]-line.Width.Px())/2
			}
			if len(ops) >= limits.MaxOperations {
				return nil, render.ErrLimit
			}
			clipEnd := end
			if align != "l" {
				clipEnd = xs[ci+1]
			}
			ops = append(ops, layout.DrawGlyphs{At: layout.Point{X: sheetRenderUnit(x), Y: sheetRenderUnit(ys[ri+1] - 2 - descent)}, Text: text, Glyphs: line.Glyphs, Face: line.Face, Size: fontSize, Color: style.RGBA{A: 1}, Clip: layout.Clip{Active: true, Rect: layout.Rect{X: sheetRenderUnit(xs[ci]), Y: sheetRenderUnit(ys[ri]), W: sheetRenderUnit(clipEnd - xs[ci]), H: sheetRenderUnit(ys[ri+1] - ys[ri])}}})
			end = xs[ci]
		}
	}
	return render.Prepare(ctx, dml.EMU(math.Round(xs[nc]*float64(dml.EMUsPerPixel))), dml.EMU(math.Round(ys[nr]*float64(dml.EMUsPerPixel))), ops, limits)
}

func sheetRenderUnit(px float64) style.Unit { u, _ := style.FromPx(px); return u }

func renderCellText(s *view.Sheet, c *oxml.CT_Cell) (string, string, error) {
	if c.F != nil && c.V == nil {
		return "", "", fmt.Errorf("%w: uncached formula", render.ErrUnsupported)
	}
	plain := func(r *oxml.CT_Rst) (string, error) {
		if r == nil {
			return "", nil
		}
		if len(r.R) != 0 || len(r.RPh) != 0 || r.PhoneticPr != nil {
			return "", fmt.Errorf("%w: rich string", render.ErrUnsupported)
		}
		if r.T == nil {
			return "", nil
		}
		return *r.T, nil
	}
	switch c.Type() {
	case "inlineStr":
		text, e := plain(c.Is)
		return text, "l", e
	case "s":
		if c.V == nil {
			return "", "", render.ErrInvalid
		}
		idx, e := strconv.ParseUint(*c.V, 10, 32)
		if e != nil || s.SharedStrings == nil || idx >= uint64(len(s.SharedStrings.Si)) {
			return "", "", fmt.Errorf("%w: shared string index", render.ErrInvalid)
		}
		text, e := plain(&s.SharedStrings.Si[idx])
		return text, "l", e
	case "str":
		if c.V == nil {
			return "", "l", nil
		}
		return *c.V, "l", nil
	case "b":
		if c.V == nil {
			return "", "", render.ErrInvalid
		}
		if *c.V == "1" {
			return "TRUE", "c", nil
		}
		if *c.V == "0" {
			return "FALSE", "c", nil
		}
		return "", "", render.ErrInvalid
	case "", "n":
		if c.V == nil {
			return "", "l", nil
		}
		n, e := strconv.ParseInt(*c.V, 10, 32)
		if e != nil || n <= -1000000000 || n >= 1000000000 {
			return "", "", fmt.Errorf("%w: General numeric value", render.ErrUnsupported)
		}
		return strconv.FormatInt(n, 10), "r", nil
	default:
		return "", "", fmt.Errorf("%w: cell type", render.ErrUnsupported)
	}
}

func renderWorksheetProfile(ws *oxml.CT_Worksheet) error {
	if ws.SheetPr != nil || ws.SheetCalcPr != nil || ws.SheetProtection != nil || ws.Scenarios != nil || ws.AutoFilter != nil || ws.SortState != nil || ws.MergeCells != nil || ws.PhoneticPr != nil || len(ws.ConditionalFormatting) > 0 || ws.DataValidations != nil || ws.Hyperlinks != nil || ws.PrintOptions != nil || ws.PageMargins != nil || ws.PageSetup != nil || ws.HeaderFooter != nil || ws.RowBreaks != nil || ws.ColBreaks != nil || ws.Drawing != nil || ws.LegacyDrawing != nil || ws.OleObjects != nil || ws.TableParts != nil || ws.ExtLst != nil || len(ws.UnknownChildren) > 0 {
		return fmt.Errorf("%w: worksheet feature", render.ErrUnsupported)
	}
	return nil
}

func sheetRenderXML(n core.XMLNode, root string) error {
	if n.Text {
		if len(n.Path) > 0 {
			last := n.Path[len(n.Path)-1]
			if last.Space == nsSML && (last.Local == "t" || last.Local == "v" || last.Local == "f") {
				return nil
			}
		}
		return fmt.Errorf("%w: XML text", render.ErrUnsupported)
	}
	if n.Name.Space != nsSML || (len(n.Path) == 1 && n.Name.Local != root) {
		return fmt.Errorf("%w: XML namespace/root", render.ErrUnsupported)
	}
	spec, ok := sheetRenderElements[n.Name.Local]
	if !ok {
		return fmt.Errorf("%w: XML %s", render.ErrUnsupported, n.Name.Local)
	}
	if len(n.Path) > 1 && !strings.Contains(" "+spec.parents+" ", " "+n.Path[len(n.Path)-2].Local+" ") {
		return fmt.Errorf("%w: XML placement", render.ErrUnsupported)
	}
	if n.Occurrence > 1 && !spec.repeat {
		return fmt.Errorf("%w: repeated XML", render.ErrInvalid)
	}
	for _, a := range n.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		if n.Name.Local == "t" && a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "space" && (a.Value == "preserve" || a.Value == "default") {
			continue
		}
		if a.Name.Space != "" || !strings.Contains(" "+spec.attrs+" ", " "+a.Name.Local+" ") {
			return fmt.Errorf("%w: XML attribute", render.ErrUnsupported)
		}
	}
	return nil
}

var sheetRenderElements = map[string]struct {
	parents, attrs string
	repeat         bool
}{
	"worksheet": {}, "dimension": {"worksheet", "ref", false}, "sheetViews": {"worksheet", "", false}, "sheetView": {"sheetViews", "workbookViewId showGridLines showRowColHeaders tabSelected zoomScale zoomScaleNormal", false}, "selection": {"sheetView", "activeCell sqref", true},
	"sheetFormatPr": {"worksheet", "baseColWidth defaultColWidth defaultRowHeight customHeight zeroHeight", false}, "cols": {"worksheet", "", true}, "col": {"cols", "min max width hidden customWidth", true}, "sheetData": {"worksheet", "", false}, "row": {"sheetData", "r spans ht hidden customHeight", true}, "c": {"row", "r s t", true}, "v": {"c", "", false}, "f": {"c", "", false}, "is": {"c", "", false}, "t": {"is si", "", false},
	"sst": {"", "count uniqueCount", false}, "si": {"sst", "", true},
	"styleSheet": {}, "fonts": {"styleSheet", "count", false}, "font": {"fonts", "", true}, "name": {"font", "val", false}, "sz": {"font", "val", false}, "fills": {"styleSheet", "count", false}, "fill": {"fills", "", true}, "patternFill": {"fill", "patternType", false}, "borders": {"styleSheet", "count", false}, "border": {"borders", "", true}, "cellStyleXfs": {"styleSheet", "count", false}, "cellXfs": {"styleSheet", "count", false}, "xf": {"cellStyleXfs cellXfs", "numFmtId fontId fillId borderId xfId", true}, "cellStyles": {"styleSheet", "count", false}, "cellStyle": {"cellStyles", "name xfId builtinId", true},
}
