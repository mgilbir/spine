package main

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// ---- info ----

func runInfo(fs *flagSetT, stdout io.Writer) error {
	args := fs.Args()
	if len(args) != 1 {
		return &flagError{message: fmt.Sprintf("info expects exactly one file argument (%d given)", len(args))}
	}
	common := flagsCommon(fs)
	d, err := openDocument(args[0], openOptions{password: common.password})
	if err != nil {
		return err
	}
	defer d.close()
	return writeJSON(stdout, docInfo(d))
}

// maxCellsPerRange caps how many cells one cells read may cover, on the CLI
// and on GET /cells alike.
const maxCellsPerRange int64 = 10_000_000

// docInfo builds the info output for an open document.
func docInfo(d *document) any {
	if d.kind == "xlsx" {
		active := d.wb.ActiveSheet()
		out := infoXLSX{Kind: "xlsx", SheetCount: d.wb.SheetCount()}
		for i, s := range d.wb.Sheets() {
			out.Sheets = append(out.Sheets, sheetInfo{Index: i, Name: s.Name(), Active: s == active})
		}
		out.Title = d.wb.Properties.Title
		out.Creator = d.wb.Properties.Creator
		return out
	}
	out := infoPPTX{Kind: "pptx", SlideCount: d.pres.SlideCount()}
	out.Slides = make([]slideInfo, 0, d.pres.SlideCount())
	for i, s := range d.pres.Slides() {
		out.Slides = append(out.Slides, slideInfo{Index: i + 1, Name: s.Name(), ShapeCount: len(s.Shapes())})
	}
	out.Title = d.pres.Properties.Title
	out.Creator = d.pres.Properties.Creator
	return out
}

// ---- ls ----

func runLs(fs *flagSetT, stdout io.Writer) error {
	args := fs.Args()
	if len(args) != 2 {
		return &flagError{message: fmt.Sprintf("ls expects a file and a scope (%d arguments given)", len(args))}
	}
	path, scope := args[0], args[1]
	common := flagsCommon(fs)
	d, err := openDocument(path, openOptions{password: common.password})
	if err != nil {
		return err
	}
	defer d.close()
	out, err := docLS(d, scope)
	if err != nil {
		return err
	}
	return writeJSON(stdout, out)
}

// docLS builds the ls output for scope: sheets, slides or slide/<N>.
func docLS(d *document, scope string) (any, error) {
	switch {
	case scope == "sheets":
		return lsSheetsDoc(d)
	case scope == "slides":
		return lsSlidesDoc(d)
	case strings.HasPrefix(scope, "slide/"):
		return lsSlideShapesDoc(d, scope)
	default:
		return nil, errf("invalid_selector", "unknown scope %q; use sheets, slides or slide/<N>", scope)
	}
}

func lsSheetsDoc(d *document) (any, error) {
	if d.kind != "xlsx" {
		return nil, errf("unsupported", "scope sheets applies only to xlsx files")
	}
	active := d.wb.ActiveSheet()
	out := make([]sheetInfo, 0, d.wb.SheetCount())
	for i, s := range d.wb.Sheets() {
		out = append(out, sheetInfo{Index: i, Name: s.Name(), Active: s == active})
	}
	return out, nil
}

func lsSlidesDoc(d *document) (any, error) {
	if d.kind != "pptx" {
		return nil, errf("unsupported", "scope slides applies only to pptx files")
	}
	out := make([]slideEntry, 0, d.pres.SlideCount())
	for i, s := range d.pres.Slides() {
		out = append(out, slideEntry{Index: i + 1, Name: s.Name(), ShapeCount: len(s.Shapes()), TextPreview: textPreview(s.Text())})
	}
	return out, nil
}

func lsSlideShapesDoc(d *document, scope string) (any, error) {
	if d.kind != "pptx" {
		return nil, errf("unsupported", "scope slide/<N> applies only to pptx files")
	}
	n, err := parseSlideArg(strings.TrimPrefix(scope, "slide/"))
	if err != nil {
		return nil, err
	}
	slide, err := resolveSlide(d.pres, n)
	if err != nil {
		return nil, err
	}
	out := make([]shapeEntry, 0, len(slide.Shapes()))
	for _, sh := range slide.Shapes() {
		entry := shapeEntry{ID: sh.ID(), Name: sh.Name(), Type: shapeTypeName(sh.ShapeType()), TextPreview: textPreview(shapeText(sh))}
		if ph, ok := sh.(*pptx.PlaceholderShape); ok {
			entry.Placeholder = &placeholderInfo{Type: placeholderTypeName(ph.PlaceholderType()), Idx: ph.Index()}
		}
		out = append(out, entry)
	}
	return out, nil
}

// shapeTypeName maps a shape kind to the ls type vocabulary.
func shapeTypeName(t pptx.ShapeType) string {
	switch t {
	case pptx.ShapeTypeTextBox:
		return "text_box"
	case pptx.ShapeTypePicture:
		return "picture"
	case pptx.ShapeTypeTable:
		return "table"
	case pptx.ShapeTypeChart:
		return "chart"
	case pptx.ShapeTypeGroup:
		return "group"
	case pptx.ShapeTypePlaceholder:
		return "placeholder"
	case pptx.ShapeTypeConnector:
		return "connector"
	case pptx.ShapeTypeAutoShape:
		return "auto_shape"
	case pptx.ShapeTypeVideo:
		return "video"
	case pptx.ShapeTypeAudio:
		return "audio"
	case pptx.ShapeTypeDiagram:
		return "diagram"
	case pptx.ShapeTypeOLEObject:
		return "ole_object"
	default:
		return "unknown"
	}
}

// placeholderTypeName maps a placeholder type to the ls vocabulary.
func placeholderTypeName(t pptx.PlaceholderType) string {
	switch t {
	case pptx.PlaceholderTitle:
		return "title"
	case pptx.PlaceholderBody:
		return "body"
	case pptx.PlaceholderCenteredTitle:
		return "center_title"
	default:
		return "other"
	}
}

// shapeText returns a shape's own text (group children concatenated), or ""
// for text-free shapes.
func shapeText(sh pptx.Shape) string {
	switch v := sh.(type) {
	case *pptx.TextBox:
		return v.TextFrame().Text()
	case *pptx.PlaceholderShape:
		return v.Text()
	case *pptx.AutoShape:
		if tf := v.TextFrame(); tf != nil {
			return tf.Text()
		}
	case *pptx.GroupShape:
		var sb strings.Builder
		for _, child := range v.Children() {
			if t := shapeText(child); t != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(t)
			}
		}
		return sb.String()
	case *pptx.Table:
		var sb strings.Builder
		for _, row := range v.Rows() {
			cells := row.Cells()
			texts := make([]string, len(cells))
			for i, c := range cells {
				texts[i] = c.Text()
			}
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(strings.Join(texts, "\t"))
		}
		return sb.String()
	}
	return ""
}

// ---- text ----

func runText(fs *flagSetT, stdout io.Writer) error {
	args := fs.Args()
	if len(args) != 1 {
		return &flagError{message: fmt.Sprintf("text expects exactly one file argument (%d given)", len(args))}
	}
	common := flagsCommon(fs)
	slideGiven, sheetGiven := false, false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "slide":
			slideGiven = true
		case "sheet":
			sheetGiven = true
		}
	})
	if slideGiven && common.slide < 1 {
		return &flagError{message: fmt.Sprintf("--slide must be 1 or greater (got %d)", common.slide)}
	}
	if sheetGiven && slideGiven {
		return &flagError{message: "--sheet and --slide cannot be combined"}
	}
	d, err := openDocument(args[0], openOptions{password: common.password})
	if err != nil {
		return err
	}
	defer d.close()
	text, err := docText(d, common.sheet, sheetGiven, slideGiven, common.slide)
	if err != nil {
		return err
	}
	if common.json {
		return writeJSON(stdout, textOutput{Text: text})
	}
	// The human format is exactly the Text() bytes, with a trailing newline
	// added when absent.
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	_, err = io.WriteString(stdout, text)
	return err
}

// docText extracts the document's plain text, restricted to one sheet or
// slide when asked. The restrictions reuse the text command's semantics; an
// explicitly empty sheet name is distinguished from an absent restriction
// and resolves to not_found.
func docText(d *document, sheetName string, sheetGiven, slideGiven bool, slide int) (string, error) {
	var text string
	switch d.kind {
	case "xlsx":
		if slideGiven {
			return "", errf("invalid_value", "--slide applies only to pptx files")
		}
		if sheetGiven {
			s, serr := resolveSheet(d.wb, sheetName)
			if serr != nil {
				return "", serr
			}
			text = s.Text()
		} else {
			text = d.wb.Text()
		}
	default:
		if sheetGiven {
			return "", errf("invalid_value", "--sheet applies only to xlsx files")
		}
		if slideGiven {
			s, err := resolveSlide(d.pres, slide)
			if err != nil {
				return "", err
			}
			text = s.Text()
		} else {
			text = d.pres.Text()
		}
	}
	return text, nil
}

// ---- cells ----

func runCells(fs *flagSetT, stdout io.Writer) error {
	args := fs.Args()
	if len(args) != 3 {
		return &flagError{message: fmt.Sprintf("cells expects a file, a sheet and a range (%d arguments given)", len(args))}
	}
	common := flagsCommon(fs)
	path, sheetName, rng := args[0], args[1], args[2]
	d, err := openDocument(path, openOptions{password: common.password})
	if err != nil {
		return err
	}
	defer d.close()
	if d.kind != "xlsx" {
		return errf("unsupported", "cells applies only to xlsx files")
	}
	out, err := docCells(d, sheetName, rng)
	if err != nil {
		return err
	}
	if common.json {
		return writeJSON(stdout, out)
	}
	for _, c := range out.Cells {
		if _, werr := fmt.Fprintf(stdout, "%s\t%s\t%s\n", c.Ref, c.Type, valueString(c.Value)); werr != nil {
			return errf("io", "cannot write output: %v", werr)
		}
	}
	return nil
}

// docCells reads the present cells of an xlsx range, row-major.
func docCells(d *document, sheetName, rng string) (cellsOutput, error) {
	sheet, serr := resolveSheet(d.wb, sheetName)
	if serr != nil {
		return cellsOutput{}, serr
	}
	r1, c1, r2, c2, err := parseRange(rng)
	if err != nil {
		return cellsOutput{}, err
	}
	// 64-bit on purpose: the full grid is 17,179,869,184 cells, which
	// overflows a 32-bit int and would wrap the cap check into bypassing
	// itself (and the loop into running for hours).
	area := (int64(r2) - int64(r1) + 1) * (int64(c2) - int64(c1) + 1)
	if area > maxCellsPerRange {
		return cellsOutput{}, errf("invalid_range", "range %s covers %s cells; the limit is %s",
			formatRange(rng, r1, c1, r2, c2), commaInt(area), commaInt(maxCellsPerRange))
	}
	out := cellsOutput{Sheet: sheet.Name(), Range: formatRange(rng, r1, c1, r2, c2)}
	for r := r1; r <= r2; r++ {
		for c := c1; c <= c2; c++ {
			cell := sheet.FindCell(xlsx.FormatCellRef(r, c))
			if cell == nil || cell.Type() == xlsx.CellTypeEmpty {
				continue
			}
			entry := cellEntry{Ref: cell.Ref(), Type: cellTypeName(cell.Type()), Value: cellValue(cell)}
			if cell.Type() == xlsx.CellTypeFormula {
				entry.Formula = cell.Formula()
			}
			out.Cells = append(out.Cells, entry)
		}
	}
	if out.Cells == nil {
		out.Cells = []cellEntry{}
	}
	return out, nil
}

// parseRange parses "A1" or "A1:D20" into ordered 1-based bounds. The
// endpoints are swapped when given bottom-right first.
func parseRange(rng string) (r1, c1, r2, c2 int, err error) {
	endpoints := strings.SplitN(rng, ":", 2)
	row, col, perr := xlsx.ParseCellRef(endpoints[0])
	if perr != nil {
		return 0, 0, 0, 0, errf("invalid_range", "invalid range %q: %v", rng, perr)
	}
	r1, c1 = row, col
	r2, c2 = r1, c1
	if len(endpoints) == 2 {
		row, col, perr := xlsx.ParseCellRef(endpoints[1])
		if perr != nil {
			return 0, 0, 0, 0, errf("invalid_range", "invalid range %q: %v", rng, perr)
		}
		r2, c2 = row, col
	}
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	if c1 > c2 {
		c1, c2 = c2, c1
	}
	return r1, c1, r2, c2, nil
}

func formatRange(given string, r1, c1, r2, c2 int) string {
	if r1 == r2 && c1 == c2 {
		return xlsx.FormatCellRef(r1, c1)
	}
	return xlsx.FormatCellRef(r1, c1) + ":" + xlsx.FormatCellRef(r2, c2)
}

// commaInt renders a non-negative int64 with comma thousands separators, as
// in "17,179,869,184".
func commaInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	start := 0
	if n < 0 {
		start = 1
	}
	parts := make([]string, 0, len(s)/3+1)
	for i := len(s); i > start; {
		lo := i - 3
		if lo < start {
			lo = start
		}
		parts = append(parts, s[lo:i])
		i = lo
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return s[:start] + strings.Join(parts, ",")
}

// cellTypeName maps a CellType to the cells type vocabulary.
func cellTypeName(t xlsx.CellType) string {
	switch t {
	case xlsx.CellTypeString:
		return "string"
	case xlsx.CellTypeNumber:
		return "number"
	case xlsx.CellTypeBoolean:
		return "boolean"
	case xlsx.CellTypeFormula:
		return "formula"
	case xlsx.CellTypeError:
		return "error"
	case xlsx.CellTypeDate:
		return "date"
	default:
		return "empty"
	}
}

// cellValue types a cell's value for JSON per the spec: string→string,
// number→number, boolean→bool, error→string, date→RFC 3339, formula→the
// cached result typed as above, or null when no cached value is present.
func cellValue(c *xlsx.Cell) any {
	switch c.Type() {
	case xlsx.CellTypeString:
		return c.String()
	case xlsx.CellTypeNumber:
		return c.Float()
	case xlsx.CellTypeBoolean:
		return c.Bool()
	case xlsx.CellTypeError:
		if v, ok := c.Value().(xlsx.CellError); ok {
			return string(v)
		}
		return c.String()
	case xlsx.CellTypeDate:
		return c.Time().Format(time.RFC3339)
	case xlsx.CellTypeFormula:
		v := c.Value()
		// The library reports the formula text as the value of a formula
		// cell with no cached result; the spec wants null there.
		if s, ok := v.(string); ok && s == c.Formula() {
			return nil
		}
		return typedJSONValue(v)
	default:
		return nil
	}
}

// typedJSONValue types a library value the way cells reports literals.
func typedJSONValue(v any) any {
	switch t := v.(type) {
	case xlsx.CellError:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339)
	case string, float64, bool, nil:
		return v
	default:
		return fmt.Sprint(v)
	}
}

// valueString renders a JSON value for the human cells format.
func valueString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// flagsCommon recovers the shared flag values parsed into fs.
func flagsCommon(fs *flagSetT) *commonFlags {
	return fs.common
}
