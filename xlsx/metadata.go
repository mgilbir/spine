package xlsx

import (
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Sheet-metadata (cell/value metadata) identifiers. Excel links a dynamic-array
// (spill) master cell to an XLDAPR metadata record in xl/metadata.xml through
// the cell's cm attribute so it can render the spill without a full recalc.
const (
	// relTypeSheetMetadata links the workbook to its metadata part.
	relTypeSheetMetadata = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/sheetMetadata"
	// contentTypeSheetMetadata is the metadata part's content type.
	contentTypeSheetMetadata = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheetMetadata+xml"
	// metadataPartName is the conventional metadata part name.
	metadataPartName = "/xl/metadata.xml"
	// metadataRelTarget is the workbook-relative target of the metadata part.
	metadataRelTarget = "metadata.xml"
)

// prepareDynamicArrayMetadata tags this session's dynamic-array (spill) master
// cells with the cell-metadata index (cm="1") and writes the xl/metadata.xml
// part that the index points at, declaring the single XLDAPR (dynamic-array
// properties) record. With the metadata in place Excel shows the spill range
// immediately instead of demanding a recalc. It returns the workbook-relative
// target of the part to wire into the workbook relationships, or "" when nothing
// was synthesized.
//
// onlyDirty limits tagging to dirty (regenerated) sheets, as required on the
// round-trip path where clean sheets are streamed from their preserved bytes;
// the new-workbook path passes false to tag every sheet. It must run before the
// affected worksheet parts are marshaled so the cm attribute is emitted.
//
// Deferred: when the workbook already carries an xl/metadata.xml, spine leaves
// it untouched (merging a dynamic-array record into an existing, possibly
// unrelated, metadata part is out of scope) and Excel recomputes the spill on
// open instead.
func (w *Workbook) prepareDynamicArrayMetadata(writer *opc.Writer, onlyDirty bool) (string, error) {
	if _, exists := w.preservedParts[metadataPartName]; exists {
		return "", nil
	}

	tagged := false
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.ws() == nil {
			continue
		}
		if onlyDirty && !sheet.dirty {
			continue
		}
		for i := range sheet.ws().SheetData.Row {
			for _, c := range sheet.ws().SheetData.Row[i].C {
				if !isDynamicArrayMaster(c) || c.Cm != nil {
					continue
				}
				one := uint32(1)
				c.Cm = &one
				tagged = true
			}
		}
	}
	if !tagged {
		return "", nil
	}

	if err := writer.WritePart(metadataPartName, contentTypeSheetMetadata, dynamicArrayMetadataXML()); err != nil {
		return "", err
	}
	return metadataRelTarget, nil
}

// isDynamicArrayMaster reports whether cell c is the master of a dynamic-array
// (spill) formula: a t="array" formula carrying the alwaysCalcArray marking Excel
// writes for a dynamic array (as SetDynamicArrayFormula produces).
func isDynamicArrayMaster(c *oxml.CT_Cell) bool {
	return c != nil && c.F != nil && c.F.T == "array" && c.F.Aca != nil && *c.F.Aca
}

// dynamicArrayMetadataXML returns the bytes of an xl/metadata.xml part carrying
// the single XLDAPR dynamic-array metadata record that a spill master cell's
// cm="1" attribute references. The layout matches what Excel writes for a
// workbook whose only cell metadata is dynamic-array properties.
func dynamicArrayMetadataXML() []byte {
	const body = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<metadata xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` xmlns:xda="http://schemas.microsoft.com/office/spreadsheetml/2017/dynamicarray">` +
		`<metadataTypes count="1">` +
		`<metadataType name="XLDAPR" minSupportedVersion="120000" copy="1" pasteAll="1"` +
		` pasteValues="1" merge="1" splitFirst="1" rowColShift="1" clearFormats="1"` +
		` clearComments="1" assign="1" coerce="1" cellMeta="1"/>` +
		`</metadataTypes>` +
		`<futureMetadata name="XLDAPR" count="1"><bk><extLst>` +
		`<ext uri="{bdbb8cdc-fa1e-496e-a857-3c3f30c029c3}">` +
		`<xda:dynamicArrayProperties fDynamic="1" fCollapsed="0"/>` +
		`</ext></extLst></bk></futureMetadata>` +
		`<cellMetadata count="1"><bk><rc t="1" v="0"/></bk></cellMetadata>` +
		`</metadata>`
	return []byte(body)
}
