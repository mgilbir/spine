package xlsx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// existingDrawingPart returns the absolute part name of the drawing the sheet
// already references (from the opened package), or "" if the sheet has no
// drawing, its reference does not resolve, or the target is not a preserved
// part (e.g. a drawing created earlier this session, which the fresh-drawing
// path already handles without loss).
func (w *Workbook) existingDrawingPart(sheet *Sheet) string {
	if sheet.ws() == nil || sheet.ws().Drawing == nil || sheet.ws().Drawing.RID == "" {
		return ""
	}
	target := sheet.resolveRelTarget(sheet.partName, sheet.ws().Drawing.RID)
	if target == "" {
		return ""
	}
	if _, ok := w.preservedParts[target]; !ok {
		return ""
	}
	return target
}

// appendToExistingDrawing merges the sheet's session-added image and chart
// anchors into the drawing it already references, writing the merged drawing
// part and its merged relationships. The preserved drawing (and its .rels) are
// left untouched in the model — the caller records them as rebuilt so the
// verbatim stream skips their stale originals, and re-reading the untouched
// originals makes a second save reproduce the same merge rather than double it.
// New relationship ids and shape ids are allocated past the drawing's existing
// ones so nothing collides.
func (w *Workbook) appendToExistingDrawing(writer *opc.Writer, sheet *Sheet, drawingPart string, used map[string]struct{}, mediaSeq, chartSeq *int) error {
	part, ok := w.preservedParts[drawingPart]
	if !ok {
		return fmt.Errorf("xlsx: drawing part %q not found", drawingPart)
	}
	existingRels := cloneRelationships(w.relationships[drawingPart])
	relBase := maxRelNum(existingRels)
	shapeBase := maxDrawingShapeID(part.Data)

	// Image relationship ids come first, then charts — continuing past the
	// drawing's existing relationship ids.
	mediaRels, imgRels, err := w.writeSheetMedia(writer, sheet, used, mediaSeq, relBase)
	if err != nil {
		return err
	}
	chartRels, chartRIDs, err := w.writeSheetCharts(writer, sheet, used, chartSeq, relBase+len(mediaRels))
	if err != nil {
		return err
	}

	newAnchors := drawingAnchorsXML(sheet.images, imgRels, sheet.charts, chartRIDs, shapeBase)
	merged := spliceDrawingAnchors(part.Data, newAnchors)
	if err := writer.WritePart(drawingPart, opc.ContentTypeDrawing, merged); err != nil {
		return err
	}

	mergedRels := existingRels
	mergedRels = append(mergedRels, mediaRels...)
	mergedRels = append(mergedRels, chartRels...)
	return writer.WritePartRelationships(drawingPart, mergedRels)
}

// maxRelNum returns the largest N among the relationships' "rIdN" ids, or 0 when
// none parse, so new ids allocated as rId(maxRelNum+1)... cannot collide.
func maxRelNum(rels []*opc.Relationship) int {
	max := 0
	for _, r := range rels {
		if r == nil || !strings.HasPrefix(r.ID, "rId") {
			continue
		}
		if n, err := strconv.Atoi(r.ID[len("rId"):]); err == nil && n > max {
			max = n
		}
	}
	return max
}

// saveOpenedSheetAttachments writes the drawing/media parts for images and the
// comment/VML/threaded-comment/person parts for comments added to an opened
// (round-tripped) workbook. Both attach to a sheet through its .rels, and the
// OPC writer rejects a duplicate .rels write, so a single per-sheet pass builds
// the combined relationship set and writes each sheet's .rels exactly once —
// letting images and comments coexist on the same sheet.
//
// Part names avoid the parts the package already carries. It returns the set of
// sheet .rels part names it rebuilt (so the verbatim loop skips their stale
// originals), the set of non-.rels preserved parts it rewrote (a drawing merged
// with session anchors — likewise skipped by the verbatim loop), and the
// workbook-relative target of a regenerated person list (so the caller wires the
// workbook relationship), or "" if none. Each touched sheet is marked dirty so
// its worksheet is re-marshaled with the new <drawing>/<legacyDrawing>
// references.
//
// Called before the preserved parts are streamed, so worksheetParts and
// needRelsRebuild see the freshly-dirtied sheets.
func (w *Workbook) saveOpenedSheetAttachments(writer *opc.Writer) (rebuiltRels, rebuiltParts map[string]bool, personTarget string, err error) {
	rebuiltRels = make(map[string]bool)
	rebuiltParts = make(map[string]bool)

	// Occupied part names, accumulated so multiple attachment-bearing sheets
	// don't collide with each other or with existing parts.
	used := make(map[string]struct{}, len(w.preservedParts)+len(w.sheets))
	for name := range w.preservedParts {
		used[name] = struct{}{}
	}
	for _, sheet := range w.sheets {
		if sheet.partName != "" {
			used[sheet.partName] = struct{}{}
		}
	}
	drawingSeq, mediaSeq, chartSeq, tableSeq := 1, 1, 1, 1
	pivotSeq, cacheSeq := 1, 1
	embedSeq, oleVMLSeq := 1, 1
	cseq := newCommentSeq()

	// Rebuilt fresh each save; consumed when the workbook relationships and
	// <pivotCaches> element are finalized.
	w.pendingPivotCaches = nil

	for i, sheet := range w.sheets {
		// Opaque sheets (chartsheet/dialogsheet/macrosheet) are preserved verbatim
		// and carry no worksheet model to attach drawings/tables/etc. to (C241).
		if sheet.opaque {
			continue
		}
		hasImages := len(sheet.images) > 0
		hasCharts := len(sheet.charts) > 0
		hasComments := sheet.comments != nil && sheet.comments.mutated
		hasHyperlinks := len(sheet.pendingHyperlinkRels) > 0
		hasTables := len(sheet.newTables) > 0
		hasPivots := len(sheet.newPivots) > 0
		hasOLE := len(sheet.oleEmbeds) > 0
		if !hasImages && !hasCharts && !hasComments && !hasHyperlinks && !hasTables && !hasPivots && !hasOLE {
			continue
		}
		// Resolve the sheet's part name (a new sheet added to an opened book
		// may not have one yet).
		if sheet.partName == "" {
			partName, _ := nextWorksheetPartName(w.preservedParts, w.sheets, i+1)
			sheet.partName = partName
			used[partName] = struct{}{}
		}
		if sheet.ws() == nil {
			// Nothing parsed to attach to; skip rather than produce a dangling
			// reference.
			continue
		}

		sheetRels := cloneRelationships(w.relationships[sheet.partName])
		// Drop relationships for hyperlinks removed/replaced this session. A
		// hyperlink loaded from the opened file keeps its rel here; re-emitting
		// it would bloat the .rels and leak a stale external URL.
		if len(sheet.removedHyperlinkRIDs) > 0 {
			filtered := sheetRels[:0]
			for _, rel := range sheetRels {
				if rel != nil && sheet.removedHyperlinkRIDs[rel.ID] {
					continue
				}
				filtered = append(filtered, rel)
			}
			sheetRels = filtered
		}
		// Merge external-hyperlink relationships added via SetHyperlink. Their
		// ids are already baked into the worksheet's <hyperlink r:id>; merge them
		// before computing relUsed so drawing/comment ids do not collide.
		sheetRels = append(sheetRels, sheet.pendingHyperlinkRels...)
		relUsed := relIDSet(sheetRels)

		if hasImages || hasCharts {
			// When the sheet already carries a drawing (opened from a file with
			// existing images/charts), append the session's anchors to it and
			// reuse its relationship, rather than repointing the sheet at a fresh
			// drawing that holds only the new anchors — which would orphan the
			// original drawing and lose its charts/images on reopen (C249).
			if existing := w.existingDrawingPart(sheet); existing != "" {
				if werr := w.appendToExistingDrawing(writer, sheet, existing, used, &mediaSeq, &chartSeq); werr != nil {
					return nil, nil, "", werr
				}
				// The merged drawing part and its rels are written fresh here, so
				// the verbatim stream must skip their stale preserved originals.
				rebuiltParts[existing] = true
				rebuiltRels[relsPartFor(existing)] = true
			} else {
				drawingPart, drawingFile := allocDrawingName(used, &drawingSeq)
				drawingRID := fmt.Sprintf("rId%d", nextRelationshipID(relUsed))
				relUsed[drawingRID] = struct{}{}

				// Point the worksheet at the drawing and force a re-marshal so the
				// <drawing r:id> element is emitted. A sheet parsed from a file
				// marshals by its captured ChildOrder, which won't contain
				// "drawing" if the original had none, so the element must be
				// inserted into the order at its schema position (audit C157).
				sheet.ws().Drawing = &oxml.CT_Drawing{RID: drawingRID}
				ensureDrawingInChildOrder(sheet.ws())

				sheetRels = append(sheetRels, &opc.Relationship{
					ID:     drawingRID,
					Type:   opc.RelTypeDrawing,
					Target: fmt.Sprintf("../drawings/%s", drawingFile),
				})

				// Images and charts share the drawing part; their relationships
				// live together in the drawing's .rels (image ids first, then
				// charts).
				drawingRels, imgRels, werr := w.writeSheetMedia(writer, sheet, used, &mediaSeq, 0)
				if werr != nil {
					return nil, nil, "", werr
				}
				chartRels, chartRIDs, cerr := w.writeSheetCharts(writer, sheet, used, &chartSeq, len(drawingRels))
				if cerr != nil {
					return nil, nil, "", cerr
				}
				drawingRels = append(drawingRels, chartRels...)
				if err := writer.WritePart(drawingPart, opc.ContentTypeDrawing, marshalDrawingXML(sheet.images, imgRels, sheet.charts, chartRIDs)); err != nil {
					return nil, nil, "", err
				}
				if err := writer.WritePartRelationships(drawingPart, drawingRels); err != nil {
					return nil, nil, "", err
				}
			}
		}

		if hasComments {
			sheetRels, err = w.writeSheetComments(writer, sheet, sheetRels, relUsed, used, cseq)
			if err != nil {
				return nil, nil, "", err
			}
		}

		if hasTables {
			sheetRels, err = w.writeSheetTables(writer, sheet, sheetRels, relUsed, used, &tableSeq)
			if err != nil {
				return nil, nil, "", err
			}
		}

		if hasPivots {
			sheetRels, err = w.writeSheetPivotTables(writer, sheet, sheetRels, relUsed, used, &pivotSeq, &cacheSeq)
			if err != nil {
				return nil, nil, "", err
			}
		}

		if hasOLE {
			sheetRels, err = w.writeSheetOLE(writer, sheet, sheetRels, relUsed, used, &embedSeq, &oleVMLSeq, &mediaSeq)
			if err != nil {
				return nil, nil, "", err
			}
		}

		if err := writer.WritePartRelationships(sheet.partName, sheetRels); err != nil {
			return nil, nil, "", err
		}
		rebuiltRels[relsPartFor(sheet.partName)] = true
		sheet.dirty = true
	}

	// Workbook-shared person list (threaded comment authors).
	personTarget, err = w.writeWorkbookPersons(writer, used)
	if err != nil {
		return nil, nil, "", err
	}
	return rebuiltRels, rebuiltParts, personTarget, nil
}

// writeSheetMedia writes the raster (and, for SVG, the svg) media parts for a
// sheet's images with collision-safe names, returning the drawing's image
// relationships and the per-image relationship ids the drawing XML references.
func (w *Workbook) writeSheetMedia(writer *opc.Writer, sheet *Sheet, used map[string]struct{}, mediaSeq *int, startRelN int) ([]*opc.Relationship, []imageRels, error) {
	drawingRels := make([]*opc.Relationship, 0, len(sheet.images))
	imgRels := make([]imageRels, len(sheet.images))
	relN := startRelN
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
