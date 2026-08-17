package xlsx

import (
	"bytes"
	"encoding/xml"
	"errors"
	xmlb "github.com/mgilbir/spine/common/xml"
	"path"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// ErrNilWorkbook is returned when a copy operation is given a nil source
// workbook.
var ErrNilWorkbook = errors.New("xlsx: source workbook is nil")

// AppendSheetsFrom copies every sheet of other into this workbook, in order,
// after the existing sheets, and returns the new sheets.
//
// It is the whole-file merge xlsx was missing: docx has Document.Append and
// pptx has Presentation.AppendSlidesFrom, but "merge workbook B into A" here
// meant looping CopySheetFrom over other.Sheets() by name and reconciling the
// rest by hand (C569). Each sheet is copied under a unique name — a " (2)"-style
// suffix is appended when the source name is already taken — so the returned
// slice, not the source names, is how to find the copies.
//
// The per-sheet contract is CopySheetFrom's, and so are its limits: cell values,
// styles, formulas, merged ranges, column widths, row heights and images are
// carried; charts, cross-sheet formula references, defined names and pivot
// tables are not. A sheet that fails to copy aborts the whole append, leaving
// the sheets copied before it in place — this is not transactional.
//
// Chartsheets, dialogsheets and macrosheets in other are skipped: they round-trip
// verbatim and have no worksheet model to copy from (CopySheetFrom would report
// ErrSheetNotFound for them).
func (w *Workbook) AppendSheetsFrom(other *Workbook) ([]*Sheet, error) {
	if other == nil {
		return nil, ErrNilWorkbook
	}
	if other == w {
		return nil, errors.New("xlsx: cannot append a workbook into itself")
	}
	// Snapshot the source list first: CopySheetFrom appends to w.sheets, and if
	// w and other were ever the same slice the loop would not terminate.
	src := other.Sheets()
	out := make([]*Sheet, 0, len(src))
	for _, s := range src {
		if s == nil || s.opaque {
			continue
		}
		copied, err := w.CopySheetFrom(other, s.Name())
		if err != nil {
			return out, err
		}
		out = append(out, copied)
	}
	return out, nil
}

// CopySheetFrom copies the sheet named sheetName from other into this workbook
// under a unique name (a suffix is appended if the name is already taken),
// returning the new sheet. Cell values, styles, formulas, merged ranges, and
// column widths / row heights are carried over. Shared-string cell values are
// resolved and written as inline strings so the two workbooks' string tables
// need not be merged, and cell style indices are remapped into this workbook's
// stylesheet (deduplicated).
//
// Images embedded in the source sheet — both those added this session and those
// loaded from an opened source's drawing part — are copied into a fresh drawing
// on the new sheet, with their media re-embedded under non-colliding part names.
// Charts embedded in the source sheet are not copied (deferred), and cross-sheet
// references in copied formulas are not rewritten.
func (w *Workbook) CopySheetFrom(other *Workbook, sheetName string) (*Sheet, error) {
	if other == nil {
		return nil, ErrNilWorkbook
	}
	if other == w {
		return nil, errors.New("xlsx: cannot copy a sheet from a workbook into itself")
	}

	src, err := other.SheetByName(sheetName)
	if err != nil {
		return nil, err
	}
	if src.ws() == nil {
		return nil, ErrSheetNotFound
	}

	// The godoc above promises a unique name with a suffix appended when taken,
	// so this path asks for the derived name explicitly (C440): AddSheet itself
	// now rejects a collision rather than renaming behind the caller's back.
	dst := w.addSheet(w.UniqueSheetName(sheetName))

	// styleCache maps a source cellXfs index to the index it was assigned in
	// this workbook's stylesheet, so identical styles are registered once.
	styleCache := make(map[uint32]uint32)

	copyCells := func() error {
		// Cursors over the destination rows: dst.Cell would re-scan the growing
		// destination for every source cell, making a whole-sheet copy
		// quadratic in both the row count and the per-row cell count.
		cursors := dst.newRowCursors()
		for i := range src.ws().SheetData.Row {
			row := &src.ws().SheetData.Row[i]
			for _, sc := range row.C {
				if sc == nil || sc.R == "" {
					continue
				}
				dc, err := cursors.cellByRef(sc.R)
				if err != nil {
					return err
				}
				if err := copyCellValue(w, other, sc, dc); err != nil {
					return err
				}
				if sc.S != nil {
					newIdx, err := remapStyleIndex(w, other, *sc.S, styleCache)
					if err != nil {
						return err
					}
					dc.SetStyleIndex(newIdx)
				}
			}
		}
		return nil
	}
	if err := copyCells(); err != nil {
		// Roll the half-populated sheet back out of the workbook: returning it
		// attached left the caller with a partial copy under a name that now
		// looks taken (C549d).
		if delErr := w.DeleteSheet(dst.index); delErr != nil {
			return nil, err
		}
		return nil, err
	}

	copyMerges(src, dst)
	copyColumnWidths(src, dst)
	copyRowHeights(src, dst)
	copySheetImages(src, dst, other)

	return dst, nil
}

// copySheetImages copies every image on src into dst by appending reconstructed
// sheetImage entries (with cloned media bytes) to dst.images. The shared
// save-time drawing pipeline then allocates fresh drawing/media part names and
// wires the worksheet->drawing and drawing->media relationships, so the copy
// never collides with the destination's existing parts. Both created-source
// images (added this session) and opened-source images (parsed from the source
// drawing part) are carried.
func copySheetImages(src, dst *Sheet, srcWB *Workbook) {
	// Images added to src this session.
	for i := range src.images {
		img := src.images[i] // value copy
		img.data = bytes.Clone(img.data)
		if img.svgData != nil {
			img.svgData = bytes.Clone(img.svgData)
		}
		dst.images = append(dst.images, img)
		dst.markDirty()
	}
	// Images loaded from an opened source's drawing part.
	for _, img := range openedSheetImages(src, srcWB) {
		dst.images = append(dst.images, img)
		dst.markDirty()
	}
}

// openedSheetImages reconstructs sheetImage values from an opened source sheet's
// drawing part, resolving each picture's blip to its media bytes. Anchors that
// are not pictures (e.g. charts) or whose media cannot be resolved are skipped.
// The SVG variant of an SVG-with-raster-fallback image is not reconstructed
// (deferred); the raster fallback is copied.
func openedSheetImages(src *Sheet, srcWB *Workbook) []sheetImage {
	if srcWB == nil || src.ws() == nil || src.ws().Drawing == nil {
		return nil
	}
	drawingPart := src.resolveRelTarget(src.partName, src.ws().Drawing.RID)
	if drawingPart == "" {
		return nil
	}
	part, ok := srcWB.preservedParts[drawingPart]
	if !ok {
		return nil
	}
	var wsDr copyWsDr
	if err := xmlb.Unmarshal(part.Data, &wsDr); err != nil {
		return nil
	}

	var out []sheetImage
	appendAnchor := func(a *copyAnchor, twoCell bool) {
		if a.Pic == nil || a.Pic.BlipFill.Blip.Embed == "" || a.From == nil {
			return
		}
		mediaPart := src.resolveRelTarget(drawingPart, a.Pic.BlipFill.Blip.Embed)
		if mediaPart == "" {
			return
		}
		media, ok := srcWB.preservedParts[mediaPart]
		if !ok {
			return
		}
		img := sheetImage{
			data:        bytes.Clone(media.Data),
			ext:         strings.TrimPrefix(path.Ext(mediaPart), "."),
			contentType: media.ContentType,
			fromCol:     a.From.Col,
			fromRow:     a.From.Row,
			altText:     a.Pic.NvPicPr.CNvPr.Descr,
		}
		if twoCell && a.To != nil {
			img.twoCell = true
			img.toCol = a.To.Col
			img.toRow = a.To.Row
		} else if a.Ext != nil {
			img.widthEMU = a.Ext.Cx
			img.heightEMU = a.Ext.Cy
		}
		out = append(out, img)
	}
	for i := range wsDr.OneCell {
		appendAnchor(&wsDr.OneCell[i], false)
	}
	for i := range wsDr.TwoCell {
		appendAnchor(&wsDr.TwoCell[i], true)
	}
	return out
}

// copyWsDr / copyAnchor are a minimal spreadsheet-drawing parse model for the
// copy path. Unlike the read-only xdr* model (image_reader.go) it also decodes
// the two-cell anchor's <to> marker so a spanning image copies faithfully.
type copyWsDr struct {
	XMLName xml.Name     `xml:"wsDr"`
	OneCell []copyAnchor `xml:"oneCellAnchor"`
	TwoCell []copyAnchor `xml:"twoCellAnchor"`
}

type copyAnchor struct {
	From *xdrMarker `xml:"from"`
	To   *xdrMarker `xml:"to"`
	Ext  *xdrExt    `xml:"ext"`
	Pic  *xdrPic    `xml:"pic"`
}

// copyCellValue transfers the value of source cell sc into destination cell dc,
// resolving shared strings and preserving formulas and self-contained values.
func copyCellValue(dstWB, srcWB *Workbook, sc *oxml.CT_Cell, dc *Cell) error {
	switch {
	case sc.F != nil:
		// Formula: copy the formula element and its cached value/type verbatim.
		f := *sc.F
		dc.cell.F = &f
		dc.cell.T = sc.T
		dc.cell.V = cloneStringPtr(sc.V)
		dc.cell.Is = cloneRst(sc.Is)
	case sc.T == "s":
		// Shared string: resolve against the source table and store inline so
		// the destination does not depend on the source's string table.
		if sc.V != nil {
			idx, err := strconv.Atoi(strings.TrimSpace(*sc.V))
			if err != nil {
				return nil
			}
			dc.SetString(srcWB.resolveSharedString(idx))
		}
	default:
		// Numbers, booleans, errors, inline and cached strings are
		// self-contained; copy the type, value, and any inline string verbatim.
		dc.cell.T = sc.T
		dc.cell.V = cloneStringPtr(sc.V)
		dc.cell.Is = cloneRst(sc.Is)
	}
	dc.markSheetDirty()
	return nil
}

// remapStyleIndex returns the index in dst's stylesheet equivalent to srcIdx in
// src's stylesheet, registering (and deduplicating) the style on first use. The
// records are cloned directly rather than round-tripped through the public
// CellStyle, which models colours only as RGB and so silently dropped every
// theme- and indexed-coloured font and fill (C549c).
func remapStyleIndex(dst, src *Workbook, srcIdx uint32, cache map[uint32]uint32) (uint32, error) {
	if mapped, ok := cache[srcIdx]; ok {
		return mapped, nil
	}
	newIdx, err := dst.Styles().importXf(src.Styles(), srcIdx)
	if err != nil {
		return 0, err
	}
	cache[srcIdx] = newIdx
	return newIdx, nil
}

// copyMerges copies the merged ranges of src into dst. A single-cell merge
// reference ("A1", no colon) is legal — parseCellRangeRef accepts it — and was
// dropped by a plain strings.Cut on ":" (C549b).
func copyMerges(src, dst *Sheet) {
	if src.ws().MergeCells == nil {
		return
	}
	for _, mc := range src.ws().MergeCells.MergeCell {
		rng, err := parseCellRangeRef(mc.Ref)
		if err != nil {
			continue
		}
		// Best-effort: a fresh sheet has no overlapping ranges, but ignore any
		// individual failure rather than abort the whole copy.
		_ = dst.MergeCells(
			FormatCellRef(rng.minRow, rng.minCol),
			FormatCellRef(rng.maxRow, rng.maxCol),
		)
	}
}

// copyColumnWidths copies custom column widths from src to dst, preserving the
// source's <col> ranges. Expanding a range into one SetColWidth per column
// turned a single <col min="1" max="16384"> into 16384 entries, each carved
// through the whole existing list — quadratic work and a bloated sheet (C549a).
func copyColumnWidths(src, dst *Sheet) {
	var entries []oxml.CT_Col
	for _, cols := range src.ws().Cols {
		for _, col := range cols.Col {
			if col.Width == nil {
				continue
			}
			minCol, maxCol := col.Min, col.Max
			if minCol < 1 {
				minCol = 1
			}
			if maxCol > uint32(MaxCol) {
				maxCol = uint32(MaxCol)
			}
			if minCol > maxCol {
				continue
			}
			w := *col.Width
			customWidth := true
			entries = append(entries, oxml.CT_Col{
				Min:         minCol,
				Max:         maxCol,
				Width:       &w,
				CustomWidth: &customWidth,
			})
		}
	}
	if len(entries) == 0 {
		return
	}
	dst.markDirty()
	ws := dst.ensureWS()
	if len(ws.Cols) == 0 {
		ws.Cols = append(ws.Cols, oxml.CT_Cols{})
	}
	ws.EnsureChildOrder("cols")
	ws.Cols[0].Col = append(ws.Cols[0].Col, entries...)
}

// copyRowHeights copies custom row heights from src to dst.
func copyRowHeights(src, dst *Sheet) {
	for i := range src.ws().SheetData.Row {
		row := &src.ws().SheetData.Row[i]
		if row.Ht == nil {
			continue
		}
		if num, ok := rowNumberOf(row); ok {
			_ = dst.SetRowHeight(int(num), *row.Ht)
		}
	}
}

// cloneStringPtr returns a copy of a *string, or nil.
func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

// cloneRst returns a shallow copy of a *CT_Rst, or nil. The copy is safe to
// attach to another workbook because callers never mutate the shared run/text
// slices after the copy.
func cloneRst(r *oxml.CT_Rst) *oxml.CT_Rst {
	if r == nil {
		return nil
	}
	c := *r
	return &c
}
