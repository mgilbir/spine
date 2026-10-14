package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// runSet implements `spine set <file> <selector> <value>`.
func runSet(fs *flagSetT, stdout io.Writer) error {
	args := fs.Args()
	if len(args) != 3 {
		return &flagError{message: fmt.Sprintf("set expects a file, a selector and a value (%d arguments given)", len(args))}
	}
	common := flagsCommon(fs)
	path, selector, value := args[0], args[1], args[2]
	d, err := openDocument(path, openOptions{password: common.password})
	if err != nil {
		return err
	}
	defer d.close()
	switch {
	case strings.HasPrefix(selector, "slide/"):
		if common.setType != "" {
			return typeNotAllowed()
		}
		if d.kind != "pptx" {
			return errf("invalid_selector", "selector %q addresses a slide but %s is not a pptx file", selector, path)
		}
		return setPptxText(d, selector, value, common, stdout)
	default:
		if d.kind != "xlsx" {
			return errf("invalid_selector", "selector %q addresses a cell but %s is not an xlsx file", selector, path)
		}
		return setXlsxCell(d, selector, value, common, stdout)
	}
}

// setXlsxCell applies one cell write and saves atomically.
func setXlsxCell(d *document, selector, value string, common *commonFlags, stdout io.Writer) error {
	sheetName, ref, selectsNothing, err := parseCellSelector(selector)
	if err != nil {
		return err
	}
	if selectsNothing {
		return errf("not_found", "sheet not found: %q", sheetName)
	}
	row, col, perr := xlsx.ParseCellRef(ref)
	if perr != nil {
		return errf("invalid_range", "invalid cell reference %q: %v", ref, perr)
	}
	sheet, serr := resolveSheet(d.wb, sheetName)
	if serr != nil {
		return serr
	}
	if merr := checkMerged(sheet, ref, row, col); merr != nil {
		return merr
	}
	typ, typed, verr := coerceValue(value, common.setType)
	if verr != nil {
		return verr
	}
	cell, cerr := sheet.Cell(ref)
	if cerr != nil {
		return errf("io", "cannot address %s: %v", ref, cerr)
	}
	switch v := typed.(type) {
	case string:
		cell.SetString(v)
	case float64:
		cell.SetFloat(v)
	case bool:
		cell.SetBool(v)
	case time.Time:
		// A date value is stored as an Excel serial with a date number
		// format, so it reads back as a date rather than a bare number.
		cell.SetTime(v)
		if serr := cell.SetStyle(xlsx.CellStyle{NumberFormatID: xlsx.NumberFormatDate}); serr != nil {
			return errf("io", "cannot apply date format to %s: %v", ref, serr)
		}
	case formulaValue:
		cell.SetFormula(string(v))
	}
	out := setXLSXOutput{Selector: selector, Type: typ, Value: cellValue(cell)}
	destination := d.path
	if common.out != "" {
		destination = common.out
	}
	if serr := d.save(destination); serr != nil {
		return serr
	}
	return writeJSON(stdout, out)
}

// formulaValue marks an inferred or forced formula.
type formulaValue string

// coerceValue interprets value per the spec: --type forces, otherwise infer
// (=... formula, true/false boolean, parseable float number, else string).
func coerceValue(value, typ string) (string, any, error) {
	switch typ {
	case "":
		// Default inference.
		switch {
		case strings.HasPrefix(value, "="):
			return "formula", formulaValue(value), nil
		case value == "true" || value == "false":
			return "boolean", value == "true", nil
		default:
			if f, err := strconv.ParseFloat(value, 64); err == nil {
				return "number", f, nil
			}
			return "string", value, nil
		}
	case "number":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", nil, errf("invalid_value", "value %q is not a number", value)
		}
		return "number", f, nil
	case "string":
		return "string", value, nil
	case "boolean":
		switch value {
		case "true":
			return "boolean", true, nil
		case "false":
			return "boolean", false, nil
		default:
			return "", nil, errf("invalid_value", "value %q is not a boolean; use true or false", value)
		}
	case "formula":
		if !strings.HasPrefix(value, "=") {
			return "", nil, errf("invalid_value", "value %q is not a formula; it must start with =", value)
		}
		return "formula", formulaValue(value), nil
	case "date":
		t, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return "", nil, errf("invalid_value", "value %q is not an RFC 3339 date", value)
		}
		return "date", t, nil
	default:
		return "", nil, &flagError{message: fmt.Sprintf("--type must be number, string, boolean, formula or date (got %q)", typ)}
	}
}

// checkMerged rejects writes into a merged range other than its top-left cell.
func checkMerged(sheet *xlsx.Sheet, ref string, row, col int) error {
	for _, merged := range sheet.MergedCells() {
		parts := strings.SplitN(merged, ":", 2)
		if len(parts) != 2 {
			continue
		}
		r1, c1, err1 := xlsx.ParseCellRef(parts[0])
		r2, c2, err2 := xlsx.ParseCellRef(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}
		if row >= r1 && row <= r2 && col >= c1 && col <= c2 {
			if row == r1 && col == c1 {
				continue // the top-left cell is the merge's own write target
			}
			return errf("invalid_value", "%s is part of merged range %s; write to %s", ref, merged, xlsx.FormatCellRef(r1, c1))
		}
	}
	return nil
}

// setPptxText applies one text write and saves atomically.
func setPptxText(d *document, selector, value string, common *commonFlags, stdout io.Writer) error {
	sel, err := parsePptxSelector(selector)
	if err != nil {
		return err
	}
	slide, serr := resolveSlide(d.pres, sel.slide)
	if serr != nil {
		return serr
	}
	var shape pptx.Shape
	switch sel.kind {
	case "slide":
		return errf("invalid_selector", "selector %q does not address text; use slide/%d/title, slide/%d/body or slide/%d/shape/<ID>", selector, sel.slide, sel.slide, sel.slide)
	case "title":
		ph := slide.TitlePlaceholder()
		if ph == nil {
			return errf("not_found", "slide %d has no title placeholder", sel.slide)
		}
		shape = ph
	case "body":
		ph := slide.BodyPlaceholder()
		if ph == nil {
			return errf("not_found", "slide %d has no body placeholder", sel.slide)
		}
		shape = ph
	case "shape":
		sh, sherr := resolveShape(slide, sel.slide, sel.shape)
		if sherr != nil {
			return sherr
		}
		shape = sh
	}
	tf := textFrameOf(shape)
	if tf == nil {
		return errf("unsupported", "shape %d on slide %d does not hold text", shape.ID(), sel.slide)
	}
	tf.SetText(value)
	out := setPPTXOutput{Selector: selector, ShapeID: shape.ID(), Text: textOf(shape)}
	destination := d.path
	if common.out != "" {
		destination = common.out
	}
	if serr := d.save(destination); serr != nil {
		return serr
	}
	return writeJSON(stdout, out)
}

// textFrameOf returns the text frame to write for a text-bearing shape, or
// nil for shapes that hold no text.
func textFrameOf(sh pptx.Shape) *pptx.TextFrame {
	switch v := sh.(type) {
	case *pptx.TextBox:
		return v.TextFrame()
	case *pptx.AutoShape:
		return v.TextFrame()
	case *pptx.PlaceholderShape:
		return v.TextFrame()
	default:
		return nil
	}
}

// textOf reads a shape's text back after an edit.
func textOf(sh pptx.Shape) string {
	if tf := textFrameOf(sh); tf != nil {
		return tf.Text()
	}
	return ""
}
