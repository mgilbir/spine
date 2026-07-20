package xlsx

import (
	"fmt"
	"sort"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

const (
	nsSML = xmlb.NSSpreadsheetML
	nsR   = xmlb.NSOfficeDocumentRels
)

// marshalWorkbookXML marshals a workbook to XML.
func marshalWorkbookXML(wb *oxml.CT_Workbook) ([]byte, error) {
	b := xmlb.NewSpreadsheetMLBuilder()
	b.SetSelfClosingSpace(wb.SelfClosingSpace)
	if !wb.PerGapWS {
		// With per-gap whitespace captured, the verbatim gaps replay and the
		// uniform separator must not also fire.
		b.SetElementSeparator(wb.ElemSeparator)
	}

	b.WriteProlog(wb.Prolog)

	if len(wb.OriginalRootAttrs) > 0 {
		// Use preserved root attributes in their original order
		b.StartElementWithRootAttrs(nsSML, "workbook", wb.OriginalRootAttrs)
	} else {
		// New workbook: use standard namespace declarations
		nsDecls := xmlb.SpreadsheetMLNamespaces()
		if len(wb.OriginalNSDecls) > 0 {
			nsDecls = wb.OriginalNSDecls
		}
		var attrs []xmlb.Attr
		if wb.Conformance != "" {
			attrs = append(attrs, xmlb.StrAttr("conformance", wb.Conformance))
		}
		b.StartElementWithNS(nsSML, "workbook", nsDecls, attrs...)
	}

	if len(wb.ChildOrder) > 0 {
		// Use preserved child order for round-trip fidelity
		marshalWorkbookChildrenOrdered(b, wb)
	} else {
		// Fixed order for new workbooks
		marshalWorkbookChildrenDefault(b, wb)
	}

	b.EndElement(nsSML, "workbook")
	b.WriteTrailer(wb.Prolog)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("xlsx: marshal workbook.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalWorkbookChildrenOrdered marshals workbook children in their original order.
func marshalWorkbookChildrenOrdered(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	for _, childName := range wb.ChildOrder {
		switch {
		case childName == "fileVersion" && wb.FileVersion != nil:
			b.MarshalElement(nsSML, "fileVersion", wb.FileVersion)
		case childName == "workbookPr" && wb.WorkbookPr != nil:
			b.MarshalElement(nsSML, "workbookPr", wb.WorkbookPr)
		case childName == "workbookProtection" && wb.WorkbookProtection != nil:
			marshalWorkbookProtection(b, wb.WorkbookProtection)
		case childName == "AlternateContent" && wb.AlternateContent != nil:
			wb.AlternateContent.MarshalToBuilder(b, nsSML, "AlternateContent")
		case childName == "bookViews" && wb.BookViews != nil:
			b.MarshalElement(nsSML, "bookViews", wb.BookViews)
		case childName == "sheets":
			marshalWorkbookSheets(b, wb)
		case childName == "definedNames":
			marshalWorkbookDefinedNames(b, wb)
		case childName == "calcPr" && wb.CalcPr != nil:
			b.MarshalElement(nsSML, "calcPr", wb.CalcPr)
		case childName == "pivotCaches" && wb.PivotCaches != nil:
			wb.PivotCaches.MarshalToBuilder(b, nsSML, "pivotCaches")
		case childName == "extLst":
			marshalWorkbookExtLst(b, wb)
		case strings.HasPrefix(childName, "unknown:"):
			var idx int
			_, _ = fmt.Sscanf(childName, "unknown:%d", &idx)
			if idx < len(wb.UnknownChildren) {
				b.WriteRaw(wb.UnknownChildren[idx].Data)
			}
		case strings.HasPrefix(childName, "ws:"):
			var idx int
			_, _ = fmt.Sscanf(childName, "ws:%d", &idx)
			if idx < len(wb.WsRaw) {
				b.WriteRaw(wb.WsRaw[idx])
			}
		}
	}
}

// marshalWorkbookChildrenDefault marshals workbook children in standard order (new workbooks).
func marshalWorkbookChildrenDefault(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	if wb.FileVersion != nil {
		b.MarshalElement(nsSML, "fileVersion", wb.FileVersion)
	}
	if wb.WorkbookPr != nil {
		b.MarshalElement(nsSML, "workbookPr", wb.WorkbookPr)
	}
	if wb.WorkbookProtection != nil {
		marshalWorkbookProtection(b, wb.WorkbookProtection)
	}
	if wb.AlternateContent != nil {
		wb.AlternateContent.MarshalToBuilder(b, nsSML, "AlternateContent")
	}
	if wb.BookViews != nil {
		b.MarshalElement(nsSML, "bookViews", wb.BookViews)
	}
	marshalWorkbookSheets(b, wb)
	marshalWorkbookDefinedNames(b, wb)
	if wb.CalcPr != nil {
		b.MarshalElement(nsSML, "calcPr", wb.CalcPr)
	}
	if wb.PivotCaches != nil {
		wb.PivotCaches.MarshalToBuilder(b, nsSML, "pivotCaches")
	}
	marshalWorkbookExtLst(b, wb)
}

// marshalWorkbookProtection writes the workbookProtection element. A parsed,
// unmodified element is re-emitted from its verbatim source bytes for
// byte-identical round-trip; an authored one is marshaled from its typed fields.
func marshalWorkbookProtection(b *xmlb.Builder, wp *oxml.CT_WorkbookProtection) {
	if wp.Raw != nil {
		b.WriteRaw(wp.Raw)
		return
	}
	b.MarshalElement(nsSML, "workbookProtection", wp)
}

// marshalWorkbookSheets marshals the sheets element.
func marshalWorkbookSheets(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	if wb.Sheets.CapturedChildren != nil {
		// Ordered replay (with verbatim inter-child whitespace).
		b.MarshalElement(nsSML, "sheets", &wb.Sheets)
		return
	}
	b.StartElement(nsSML, "sheets")
	for i := range wb.Sheets.Sheet {
		wb.Sheets.Sheet[i].MarshalToBuilder(b, nsSML, "sheet")
	}
	b.EndElement(nsSML, "sheets")
}

// marshalWorkbookDefinedNames marshals the definedNames element.
func marshalWorkbookDefinedNames(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	if wb.DefinedNames == nil {
		return
	}
	// An empty <definedNames/> present in the source is kept: dropping the
	// element on a no-op round trip would drift from the producer's bytes.
	if len(wb.DefinedNames.DefinedName) == 0 && !hasRawKids(wb.DefinedNames.CapturedChildren) {
		b.EmptyElementStyled(wb.DefinedNames.CapturedEmptyTag, nsSML, "definedNames")
		return
	}
	if wb.DefinedNames.CapturedChildren != nil {
		// Ordered replay (with verbatim inter-child whitespace).
		b.MarshalElement(nsSML, "definedNames", wb.DefinedNames)
		return
	}
	b.StartElement(nsSML, "definedNames")
	for i := range wb.DefinedNames.DefinedName {
		wb.DefinedNames.DefinedName[i].MarshalToBuilder(b, nsSML, "definedName")
	}
	b.EndElement(nsSML, "definedNames")
}

// hasRawKids reports whether a child capture will emit raw children.
func hasRawKids(cc *xmlb.ChildCapture) bool {
	return cc != nil && len(cc.Raw) > 0
}

// marshalWorkbookExtLst marshals the extLst element.
func marshalWorkbookExtLst(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	if wb.ExtLst == nil || len(wb.ExtLst.Ext) == 0 {
		return
	}
	// Replay declarations the source carried on the extLst element itself
	// (e.g. <extLst xmlns:x15="...">).
	b.StartElement(nsSML, "extLst", xmlb.RawAttrList(wb.ExtLst.CapturedAttrs)...)
	for i := range wb.ExtLst.Ext {
		wb.ExtLst.Ext[i].MarshalToBuilder(b, nsSML, "ext")
	}
	b.EndElement(nsSML, "extLst")
}

// marshalStylesheetXML marshals a stylesheet to XML.
func marshalStylesheetXML(ss *oxml.CT_Stylesheet) ([]byte, error) {
	b := xmlb.NewSpreadsheetMLBuilder()
	b.WriteHeader()

	if len(ss.OriginalRootAttrs) > 0 {
		// Preserve the original root attributes (xmlns decls + mc:Ignorable etc.).
		b.StartElementWithRootAttrs(nsSML, "styleSheet", ss.OriginalRootAttrs)
	} else {
		nsDecls := xmlb.SpreadsheetMLNamespaces()
		if len(ss.OriginalNSDecls) > 0 {
			nsDecls = ss.OriginalNSDecls
		}
		b.StartElementWithNS(nsSML, "styleSheet", nsDecls)
	}

	if ss.NumFmts != nil && len(ss.NumFmts.NumFmt) > 0 {
		b.MarshalElement(nsSML, "numFmts", ss.NumFmts)
	}
	if ss.Fonts != nil {
		b.MarshalElement(nsSML, "fonts", ss.Fonts)
	}
	if ss.Fills != nil {
		b.MarshalElement(nsSML, "fills", ss.Fills)
	}
	if ss.Borders != nil {
		b.MarshalElement(nsSML, "borders", ss.Borders)
	}
	if ss.CellStyleXfs != nil {
		b.MarshalElement(nsSML, "cellStyleXfs", ss.CellStyleXfs)
	}
	if ss.CellXfs != nil {
		b.MarshalElement(nsSML, "cellXfs", ss.CellXfs)
	}
	if ss.CellStyles != nil {
		b.MarshalElement(nsSML, "cellStyles", ss.CellStyles)
	}
	if ss.Dxfs != nil {
		b.MarshalElement(nsSML, "dxfs", ss.Dxfs)
	}
	if ss.TableStyles != nil {
		b.MarshalElement(nsSML, "tableStyles", ss.TableStyles)
	}
	if ss.Colors != nil {
		b.MarshalElement(nsSML, "colors", ss.Colors)
	}
	if ss.ExtLst != nil && len(ss.ExtLst.Ext) > 0 {
		b.StartElement(nsSML, "extLst")
		for i := range ss.ExtLst.Ext {
			ss.ExtLst.Ext[i].MarshalToBuilder(b, nsSML, "ext")
		}
		b.EndElement(nsSML, "extLst")
	}

	b.EndElement(nsSML, "styleSheet")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("xlsx: marshal styles.xml: %w", err)
	}
	return b.Bytes(), nil
}

// updateSheetDimension recomputes an existing dimension element from the
// sheet's cell references. Cell writes past the recorded used range would
// otherwise leave a stale dimension (e.g. "A1:B2" after writing Z99) (C117).
// It only rewrites a dimension the sheet already has — the element is
// optional and absent dimensions stay absent — and leaves it untouched when
// no cell reference is parseable.
func updateSheetDimension(ws *oxml.CT_Worksheet) {
	if ws.Dimension == nil {
		return
	}
	minRow, minCol := 0, 0
	maxRow, maxCol := 0, 0
	for i := range ws.SheetData.Row {
		for _, c := range ws.SheetData.Row[i].C {
			row, col, err := ParseCellRef(c.R)
			if err != nil {
				continue
			}
			if minRow == 0 || row < minRow {
				minRow = row
			}
			if minCol == 0 || col < minCol {
				minCol = col
			}
			if row > maxRow {
				maxRow = row
			}
			if col > maxCol {
				maxCol = col
			}
		}
	}
	if minRow == 0 || minCol == 0 {
		return
	}
	if minRow == maxRow && minCol == maxCol {
		ws.Dimension.Ref = FormatCellRef(minRow, minCol)
		return
	}
	ws.Dimension.Ref = FormatCellRef(minRow, minCol) + ":" + FormatCellRef(maxRow, maxCol)
}

// marshalWorksheetXML marshals a worksheet to XML.
func marshalWorksheetXML(ws *oxml.CT_Worksheet) ([]byte, error) {
	b := xmlb.NewSpreadsheetMLBuilder()
	b.WriteHeader()

	b.SetElementSeparator(ws.ElemSeparator)

	if len(ws.OriginalRootAttrs) > 0 {
		// Preserve the original root attributes (xmlns decls + mc:Ignorable etc.).
		b.StartElementWithRootAttrs(nsSML, "worksheet", ws.OriginalRootAttrs)
	} else {
		nsDecls := xmlb.SpreadsheetMLNamespaces()
		if len(ws.OriginalNSDecls) > 0 {
			nsDecls = ws.OriginalNSDecls
		}
		b.StartElementWithNS(nsSML, "worksheet", nsDecls)
	}

	if len(ws.ChildOrder) > 0 {
		marshalWorksheetChildrenOrdered(b, ws)
	} else {
		marshalWorksheetChildrenDefault(b, ws)
	}

	b.EndElement(nsSML, "worksheet")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("xlsx: marshal worksheet part: %w", err)
	}
	return b.Bytes(), nil
}

// marshalWorksheetChild emits a single known worksheet child by local name,
// consuming the next entry of the repeatable cols/conditionalFormatting slices
// via the supplied indices. Returns false if the name is not a known child.
func marshalWorksheetChild(b *xmlb.Builder, ws *oxml.CT_Worksheet, name string, colsIdx, cfIdx *int) bool {
	switch name {
	case "sheetPr":
		if ws.SheetPr != nil {
			b.MarshalElement(nsSML, "sheetPr", ws.SheetPr)
		}
	case "dimension":
		if ws.Dimension != nil {
			b.MarshalElement(nsSML, "dimension", ws.Dimension)
		}
	case "sheetViews":
		if ws.SheetViews != nil {
			b.MarshalElement(nsSML, "sheetViews", ws.SheetViews)
		}
	case "sheetFormatPr":
		if ws.SheetFormatPr != nil {
			ws.SheetFormatPr.MarshalToBuilder(b, nsSML, "sheetFormatPr")
		}
	case "cols":
		if *colsIdx < len(ws.Cols) {
			b.MarshalElement(nsSML, "cols", &ws.Cols[*colsIdx])
			*colsIdx++
		}
	case "sheetData":
		marshalSheetData(b, &ws.SheetData)
	case "sheetCalcPr":
		if ws.SheetCalcPr != nil {
			b.MarshalElement(nsSML, "sheetCalcPr", ws.SheetCalcPr)
		}
	case "sheetProtection":
		if ws.SheetProtection != nil {
			b.MarshalElement(nsSML, "sheetProtection", ws.SheetProtection)
		}
	case "autoFilter":
		if ws.AutoFilter != nil {
			b.MarshalElement(nsSML, "autoFilter", ws.AutoFilter)
		}
	case "sortState":
		if ws.SortState != nil {
			b.MarshalElement(nsSML, "sortState", ws.SortState)
		}
	case "mergeCells":
		if ws.MergeCells != nil {
			b.MarshalElement(nsSML, "mergeCells", ws.MergeCells)
		}
	case "phoneticPr":
		if ws.PhoneticPr != nil {
			b.MarshalElement(nsSML, "phoneticPr", ws.PhoneticPr)
		}
	case "conditionalFormatting":
		if *cfIdx < len(ws.ConditionalFormatting) {
			b.MarshalElement(nsSML, "conditionalFormatting", &ws.ConditionalFormatting[*cfIdx])
			*cfIdx++
		}
	case "dataValidations":
		if ws.DataValidations != nil {
			b.MarshalElement(nsSML, "dataValidations", ws.DataValidations)
		}
	case "hyperlinks":
		if ws.Hyperlinks != nil {
			marshalHyperlinks(b, ws.Hyperlinks)
		}
	case "printOptions":
		if ws.PrintOptions != nil {
			b.MarshalElement(nsSML, "printOptions", ws.PrintOptions)
		}
	case "pageMargins":
		if ws.PageMargins != nil {
			b.MarshalElement(nsSML, "pageMargins", ws.PageMargins)
		}
	case "pageSetup":
		if ws.PageSetup != nil {
			ws.PageSetup.MarshalToBuilder(b, nsSML, "pageSetup")
		}
	case "headerFooter":
		if ws.HeaderFooter != nil {
			b.MarshalElement(nsSML, "headerFooter", ws.HeaderFooter)
		}
	case "rowBreaks":
		if ws.RowBreaks != nil {
			b.MarshalElement(nsSML, "rowBreaks", ws.RowBreaks)
		}
	case "colBreaks":
		if ws.ColBreaks != nil {
			b.MarshalElement(nsSML, "colBreaks", ws.ColBreaks)
		}
	case "drawing":
		if ws.Drawing != nil {
			ws.Drawing.MarshalToBuilder(b, nsSML, "drawing")
		}
	case "legacyDrawing":
		if ws.LegacyDrawing != nil {
			ws.LegacyDrawing.MarshalToBuilder(b, nsSML, "legacyDrawing")
		}
	case "tableParts":
		if ws.TableParts != nil {
			marshalTableParts(b, ws.TableParts)
		}
	case "extLst":
		marshalWorksheetExtLst(b, ws)
	default:
		return false
	}
	return true
}

// marshalWorksheetExtLst emits the worksheet extension list if non-empty.
func marshalWorksheetExtLst(b *xmlb.Builder, ws *oxml.CT_Worksheet) {
	if ws.ExtLst == nil || len(ws.ExtLst.Ext) == 0 {
		return
	}
	b.StartElement(nsSML, "extLst")
	for i := range ws.ExtLst.Ext {
		ws.ExtLst.Ext[i].MarshalToBuilder(b, nsSML, "ext")
	}
	b.EndElement(nsSML, "extLst")
}

// marshalWorksheetChildrenOrdered emits children in their original order,
// including any unknown children captured as raw bytes.
func marshalWorksheetChildrenOrdered(b *xmlb.Builder, ws *oxml.CT_Worksheet) {
	colsIdx, cfIdx := 0, 0
	for _, name := range ws.ChildOrder {
		if strings.HasPrefix(name, "unknown:") {
			var idx int
			_, _ = fmt.Sscanf(name, "unknown:%d", &idx)
			if idx >= 0 && idx < len(ws.UnknownChildren) {
				b.WriteRaw(ws.UnknownChildren[idx].Data)
			}
			continue
		}
		marshalWorksheetChild(b, ws, name, &colsIdx, &cfIdx)
	}
}

// marshalWorksheetChildrenDefault emits children in schema order for sheets
// created from scratch (no captured child order).
func marshalWorksheetChildrenDefault(b *xmlb.Builder, ws *oxml.CT_Worksheet) {
	order := []string{
		"sheetPr", "dimension", "sheetViews", "sheetFormatPr", "cols", "sheetData",
		"sheetCalcPr", "sheetProtection", "autoFilter", "sortState", "mergeCells",
		"phoneticPr", "conditionalFormatting", "dataValidations", "hyperlinks",
		"printOptions", "pageMargins", "pageSetup", "headerFooter", "rowBreaks",
		"colBreaks", "drawing", "legacyDrawing", "tableParts", "extLst",
	}
	colsIdx, cfIdx := 0, 0
	for _, name := range order {
		// Emit every cols / conditionalFormatting entry, then the rest once.
		switch name {
		case "cols":
			for colsIdx < len(ws.Cols) {
				marshalWorksheetChild(b, ws, "cols", &colsIdx, &cfIdx)
			}
		case "conditionalFormatting":
			for cfIdx < len(ws.ConditionalFormatting) {
				marshalWorksheetChild(b, ws, "conditionalFormatting", &colsIdx, &cfIdx)
			}
		default:
			marshalWorksheetChild(b, ws, name, &colsIdx, &cfIdx)
		}
	}
}

// marshalSheetData marshals the sheetData element with its rows and cells.
func marshalSheetData(b *xmlb.Builder, sd *oxml.CT_SheetData) {
	if len(sd.Row) == 0 {
		b.EmptyElement(nsSML, "sheetData")
		return
	}
	// OOXML requires rows in ascending row-number order. Cells within a row are
	// already sorted at marshal time; sort the rows too (stable, so equal or
	// underivable numbers keep their relative order).
	sort.SliceStable(sd.Row, func(i, j int) bool {
		ri, _ := rowNumberOf(&sd.Row[i])
		rj, _ := rowNumberOf(&sd.Row[j])
		return ri < rj
	})
	b.StartElement(nsSML, "sheetData")
	for i := range sd.Row {
		sd.Row[i].MarshalToBuilder(b, nsSML, "row")
	}
	b.EndElement(nsSML, "sheetData")
}

// marshalHyperlinks marshals the hyperlinks element.
func marshalHyperlinks(b *xmlb.Builder, hl *oxml.CT_Hyperlinks) {
	if len(hl.Hyperlink) == 0 {
		b.EmptyElement(nsSML, "hyperlinks")
		return
	}
	b.StartElement(nsSML, "hyperlinks")
	for i := range hl.Hyperlink {
		hl.Hyperlink[i].MarshalToBuilder(b, nsSML, "hyperlink")
	}
	b.EndElement(nsSML, "hyperlinks")
}

// marshalTableParts marshals the tableParts element.
func marshalTableParts(b *xmlb.Builder, tp *oxml.CT_TableParts) {
	var attrs []xmlb.Attr
	if tp.Count != nil {
		attrs = append(attrs, xmlb.UintAttr("count", *tp.Count))
	}
	if len(tp.TablePart) == 0 {
		b.EmptyElement(nsSML, "tableParts", attrs...)
		return
	}
	b.StartElement(nsSML, "tableParts", attrs...)
	for i := range tp.TablePart {
		tp.TablePart[i].MarshalToBuilder(b, nsSML, "tablePart")
	}
	b.EndElement(nsSML, "tableParts")
}
