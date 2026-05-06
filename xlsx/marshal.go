package xlsx

import (
	"fmt"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

const (
	nsSML = xmlb.NSSpreadsheetML
	nsR   = xmlb.NSPresentationRels
)

// marshalWorkbookXML marshals a workbook to XML.
func marshalWorkbookXML(wb *oxml.CT_Workbook) []byte {
	b := xmlb.NewSpreadsheetMLBuilder()
	b.SetSelfClosingSpace(wb.SelfClosingSpace)
	b.SetElementSeparator(wb.ElemSeparator)

	if wb.OriginalXMLSep != "" {
		// Use original separator between XML declaration and root element
		b.WriteRaw([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>"))
		b.WriteRaw([]byte(wb.OriginalXMLSep))
	} else {
		b.WriteHeader()
	}

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
	return b.Bytes()
}

// marshalWorkbookChildrenOrdered marshals workbook children in their original order.
func marshalWorkbookChildrenOrdered(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	for _, childName := range wb.ChildOrder {
		switch {
		case childName == "fileVersion" && wb.FileVersion != nil:
			b.MarshalElement(nsSML, "fileVersion", wb.FileVersion)
		case childName == "workbookPr" && wb.WorkbookPr != nil:
			b.MarshalElement(nsSML, "workbookPr", wb.WorkbookPr)
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
		case childName == "extLst":
			marshalWorkbookExtLst(b, wb)
		case strings.HasPrefix(childName, "unknown:"):
			var idx int
			_, _ = fmt.Sscanf(childName, "unknown:%d", &idx)
			if idx < len(wb.UnknownChildren) {
				b.WriteRaw(wb.UnknownChildren[idx].Data)
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
	marshalWorkbookExtLst(b, wb)
}

// marshalWorkbookSheets marshals the sheets element.
func marshalWorkbookSheets(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	b.StartElement(nsSML, "sheets")
	for i := range wb.Sheets.Sheet {
		wb.Sheets.Sheet[i].MarshalToBuilder(b, nsSML, "sheet")
	}
	b.EndElement(nsSML, "sheets")
}

// marshalWorkbookDefinedNames marshals the definedNames element.
func marshalWorkbookDefinedNames(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	if wb.DefinedNames == nil || len(wb.DefinedNames.DefinedName) == 0 {
		return
	}
	b.StartElement(nsSML, "definedNames")
	for i := range wb.DefinedNames.DefinedName {
		wb.DefinedNames.DefinedName[i].MarshalToBuilder(b, nsSML, "definedName")
	}
	b.EndElement(nsSML, "definedNames")
}

// marshalWorkbookExtLst marshals the extLst element.
func marshalWorkbookExtLst(b *xmlb.Builder, wb *oxml.CT_Workbook) {
	if wb.ExtLst == nil || len(wb.ExtLst.Ext) == 0 {
		return
	}
	b.StartElement(nsSML, "extLst")
	for i := range wb.ExtLst.Ext {
		wb.ExtLst.Ext[i].MarshalToBuilder(b, nsSML, "ext")
	}
	b.EndElement(nsSML, "extLst")
}

// marshalStylesheetXML marshals a stylesheet to XML.
func marshalStylesheetXML(ss *oxml.CT_Stylesheet) []byte {
	b := xmlb.NewSpreadsheetMLBuilder()
	b.WriteHeader()

	nsDecls := xmlb.SpreadsheetMLNamespaces()
	if len(ss.OriginalNSDecls) > 0 {
		nsDecls = ss.OriginalNSDecls
	}

	b.StartElementWithNS(nsSML, "styleSheet", nsDecls)

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
	return b.Bytes()
}

// marshalWorksheetXML marshals a worksheet to XML.
func marshalWorksheetXML(ws *oxml.CT_Worksheet) []byte {
	b := xmlb.NewSpreadsheetMLBuilder()
	b.WriteHeader()

	// Use original namespace declarations if available (for round-trip fidelity),
	// otherwise use the standard set.
	nsDecls := xmlb.SpreadsheetMLNamespaces()
	if len(ws.OriginalNSDecls) > 0 {
		nsDecls = ws.OriginalNSDecls
	}

	b.StartElementWithNS(nsSML, "worksheet", nsDecls)

	if ws.SheetPr != nil {
		b.MarshalElement(nsSML, "sheetPr", ws.SheetPr)
	}
	if ws.Dimension != nil {
		b.MarshalElement(nsSML, "dimension", ws.Dimension)
	}
	if ws.SheetViews != nil {
		b.MarshalElement(nsSML, "sheetViews", ws.SheetViews)
	}
	if ws.SheetFormatPr != nil {
		ws.SheetFormatPr.MarshalToBuilder(b, nsSML, "sheetFormatPr")
	}
	for i := range ws.Cols {
		b.MarshalElement(nsSML, "cols", &ws.Cols[i])
	}

	// SheetData - always present (even if empty)
	marshalSheetData(b, &ws.SheetData)

	if ws.SheetCalcPr != nil {
		b.MarshalElement(nsSML, "sheetCalcPr", ws.SheetCalcPr)
	}
	if ws.SheetProtection != nil {
		b.MarshalElement(nsSML, "sheetProtection", ws.SheetProtection)
	}
	if ws.AutoFilter != nil {
		b.MarshalElement(nsSML, "autoFilter", ws.AutoFilter)
	}
	if ws.SortState != nil {
		b.MarshalElement(nsSML, "sortState", ws.SortState)
	}
	if ws.MergeCells != nil {
		b.MarshalElement(nsSML, "mergeCells", ws.MergeCells)
	}
	if ws.PhoneticPr != nil {
		b.MarshalElement(nsSML, "phoneticPr", ws.PhoneticPr)
	}
	for i := range ws.ConditionalFormatting {
		b.MarshalElement(nsSML, "conditionalFormatting", &ws.ConditionalFormatting[i])
	}
	if ws.DataValidations != nil {
		b.MarshalElement(nsSML, "dataValidations", ws.DataValidations)
	}
	if ws.Hyperlinks != nil {
		marshalHyperlinks(b, ws.Hyperlinks)
	}
	if ws.PrintOptions != nil {
		b.MarshalElement(nsSML, "printOptions", ws.PrintOptions)
	}
	if ws.PageMargins != nil {
		b.MarshalElement(nsSML, "pageMargins", ws.PageMargins)
	}
	if ws.PageSetup != nil {
		ws.PageSetup.MarshalToBuilder(b, nsSML, "pageSetup")
	}
	if ws.HeaderFooter != nil {
		b.MarshalElement(nsSML, "headerFooter", ws.HeaderFooter)
	}
	if ws.RowBreaks != nil {
		b.MarshalElement(nsSML, "rowBreaks", ws.RowBreaks)
	}
	if ws.ColBreaks != nil {
		b.MarshalElement(nsSML, "colBreaks", ws.ColBreaks)
	}
	if ws.Drawing != nil {
		ws.Drawing.MarshalToBuilder(b, nsSML, "drawing")
	}
	if ws.LegacyDrawing != nil {
		ws.LegacyDrawing.MarshalToBuilder(b, nsSML, "legacyDrawing")
	}
	if ws.TableParts != nil {
		marshalTableParts(b, ws.TableParts)
	}
	if ws.ExtLst != nil && len(ws.ExtLst.Ext) > 0 {
		b.StartElement(nsSML, "extLst")
		for i := range ws.ExtLst.Ext {
			ws.ExtLst.Ext[i].MarshalToBuilder(b, nsSML, "ext")
		}
		b.EndElement(nsSML, "extLst")
	}

	b.EndElement(nsSML, "worksheet")
	return b.Bytes()
}

// marshalSheetData marshals the sheetData element with its rows and cells.
func marshalSheetData(b *xmlb.Builder, sd *oxml.CT_SheetData) {
	if len(sd.Row) == 0 {
		b.EmptyElement(nsSML, "sheetData")
		return
	}
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
