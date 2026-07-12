package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// saveOpenedSheetImages writes the drawing, media, and relationship parts for
// images added to an opened (round-tripped) workbook. Unlike the created-book
// path (writeSheetDrawing), part names are chosen to avoid the parts the
// package already carries, and each image-bearing sheet's existing
// relationships are rebuilt to include the new drawing reference (the raw
// sheet .rels is streamed verbatim otherwise, so the drawing link must be added
// here).
//
// It returns the set of sheet .rels part names it rebuilt, so the verbatim
// preserved-parts loop skips streaming their stale originals. It also sets each
// sheet's <drawing> element and marks it dirty so the worksheet is re-marshaled
// with the reference.
//
// Called before the preserved parts are streamed, so worksheetParts and
// needRelsRebuild see the freshly-dirtied sheets.
func (w *Workbook) saveOpenedSheetImages(writer *opc.Writer) (rebuiltRels map[string]bool, err error) {
	rebuiltRels = make(map[string]bool)

	// Occupied part names, accumulated so multiple image-bearing sheets don't
	// collide with each other or with existing parts.
	used := make(map[string]struct{}, len(w.preservedParts)+len(w.sheets))
	for name := range w.preservedParts {
		used[name] = struct{}{}
	}
	for _, sheet := range w.sheets {
		if sheet.partName != "" {
			used[sheet.partName] = struct{}{}
		}
	}
	drawingSeq, mediaSeq := 1, 1

	for i, sheet := range w.sheets {
		if len(sheet.images) == 0 {
			continue
		}
		// Resolve the sheet's part name (a new sheet added to an opened book
		// may not have one yet).
		if sheet.partName == "" {
			partName, _ := nextWorksheetPartName(w.preservedParts, w.sheets, i+1)
			sheet.partName = partName
			used[partName] = struct{}{}
		}
		if sheet.worksheet == nil {
			// Nothing parsed to attach the drawing to; skip rather than
			// produce a dangling reference.
			continue
		}

		drawingPart, drawingFile := allocDrawingName(used, &drawingSeq)

		// Allocate a drawing relationship id on the sheet that doesn't collide
		// with the sheet's existing relationships.
		sheetRels := cloneRelationships(w.relationships[sheet.partName])
		sheetRelUsed := relIDSet(sheetRels)
		drawingRID := fmt.Sprintf("rId%d", nextRelationshipID(sheetRelUsed))

		// Point the worksheet at the drawing and force a re-marshal so the
		// <drawing r:id> element is emitted. A sheet parsed from a file
		// marshals by its captured ChildOrder, which won't contain "drawing"
		// if the original had none, so the element must be inserted into the
		// order at its schema position (audit C157 class).
		sheet.worksheet.Drawing = &oxml.CT_Drawing{RID: drawingRID}
		ensureDrawingInChildOrder(sheet.worksheet)
		sheet.dirty = true

		// Sheet -> drawing relationship, appended to the sheet's existing rels.
		sheetRels = append(sheetRels, &opc.Relationship{
			ID:     drawingRID,
			Type:   opc.RelTypeDrawing,
			Target: fmt.Sprintf("../drawings/%s", drawingFile),
		})
		if err := writer.WritePartRelationships(sheet.partName, sheetRels); err != nil {
			return nil, err
		}
		rebuiltRels[relsPartFor(sheet.partName)] = true

		// Media parts + drawing -> image relationships.
		drawingRels, imgRels, werr := w.writeSheetMedia(writer, sheet, used, &mediaSeq)
		if werr != nil {
			return nil, werr
		}

		// Drawing part + its relationships.
		if err := writer.WritePart(drawingPart, opc.ContentTypeDrawing, marshalDrawingXML(sheet.images, imgRels)); err != nil {
			return nil, err
		}
		if err := writer.WritePartRelationships(drawingPart, drawingRels); err != nil {
			return nil, err
		}
	}
	return rebuiltRels, nil
}

// writeSheetMedia writes the raster (and, for SVG, the svg) media parts for a
// sheet's images with collision-safe names, returning the drawing's image
// relationships and the per-image relationship ids the drawing XML references.
func (w *Workbook) writeSheetMedia(writer *opc.Writer, sheet *Sheet, used map[string]struct{}, mediaSeq *int) ([]*opc.Relationship, []imageRels, error) {
	drawingRels := make([]*opc.Relationship, 0, len(sheet.images))
	imgRels := make([]imageRels, len(sheet.images))
	relN := 0
	nextRelID := func() string { relN++; return fmt.Sprintf("rId%d", relN) }

	for i := range sheet.images {
		img := sheet.images[i]

		rasterPart, rasterFile := allocMediaName(used, mediaSeq, img.ext)
		if err := writer.WritePart(rasterPart, img.contentType, img.data); err != nil {
			return nil, nil, err
		}
		rasterRID := nextRelID()
		drawingRels = append(drawingRels, &opc.Relationship{
			ID:     rasterRID,
			Type:   opc.RelTypeImage,
			Target: fmt.Sprintf("../media/%s", rasterFile),
		})
		imgRels[i].rasterRID = rasterRID

		if len(img.svgData) > 0 {
			svgPart, svgFile := allocMediaName(used, mediaSeq, "svg")
			if err := writer.WritePart(svgPart, opc.ContentTypeSVG, img.svgData); err != nil {
				return nil, nil, err
			}
			svgRID := nextRelID()
			drawingRels = append(drawingRels, &opc.Relationship{
				ID:     svgRID,
				Type:   opc.RelTypeImage,
				Target: fmt.Sprintf("../media/%s", svgFile),
			})
			imgRels[i].svgRID = svgRID
		}
	}
	return drawingRels, imgRels, nil
}

// allocDrawingName finds a free /xl/drawings/drawingN.xml part, marking it used.
func allocDrawingName(used map[string]struct{}, seq *int) (partName, fileName string) {
	for {
		fileName = fmt.Sprintf("drawing%d.xml", *seq)
		partName = "/xl/drawings/" + fileName
		*seq++
		if _, ok := used[partName]; !ok {
			used[partName] = struct{}{}
			return partName, fileName
		}
	}
}

// allocMediaName finds a free /xl/media/imageN.<ext> part, marking it used.
func allocMediaName(used map[string]struct{}, seq *int, ext string) (partName, fileName string) {
	for {
		fileName = fmt.Sprintf("image%d.%s", *seq, ext)
		partName = "/xl/media/" + fileName
		*seq++
		if _, ok := used[partName]; !ok {
			used[partName] = struct{}{}
			return partName, fileName
		}
	}
}

// relIDSet returns the set of relationship ids in use.
func relIDSet(rels []*opc.Relationship) map[string]struct{} {
	set := make(map[string]struct{}, len(rels))
	for _, r := range rels {
		if r != nil {
			set[r.ID] = struct{}{}
		}
	}
	return set
}

// relsPartFor returns the .rels part path for a part, e.g.
// "/xl/worksheets/sheet1.xml" -> "/xl/worksheets/_rels/sheet1.xml.rels".
func relsPartFor(partName string) string {
	i := strings.LastIndex(partName, "/")
	if i < 0 {
		return "/_rels/" + partName + ".rels"
	}
	return partName[:i] + "/_rels/" + partName[i+1:] + ".rels"
}

// ensureDrawingInChildOrder inserts "drawing" into a parsed worksheet's
// captured child order at its schema position (after colBreaks, before
// legacyDrawing/tableParts/extLst) so the ChildOrder-gated marshal emits it.
// It is a no-op when the order is empty (a from-scratch sheet marshals in
// schema order) or already contains "drawing".
func ensureDrawingInChildOrder(ws *oxml.CT_Worksheet) {
	if len(ws.ChildOrder) == 0 {
		return
	}
	for _, name := range ws.ChildOrder {
		if name == "drawing" {
			return
		}
	}
	// Elements that follow drawing in the schema; insert before the first
	// present one.
	after := map[string]bool{"legacyDrawing": true, "tableParts": true, "extLst": true}
	insertAt := len(ws.ChildOrder)
	for i, name := range ws.ChildOrder {
		if after[name] {
			insertAt = i
			break
		}
	}
	ws.ChildOrder = append(ws.ChildOrder, "")
	copy(ws.ChildOrder[insertAt+1:], ws.ChildOrder[insertAt:])
	ws.ChildOrder[insertAt] = "drawing"
}

// sheetsHaveImages reports whether any sheet carries pending images.
func (w *Workbook) sheetsHaveImages() bool {
	for _, sheet := range w.sheets {
		if len(sheet.images) > 0 {
			return true
		}
	}
	return false
}
