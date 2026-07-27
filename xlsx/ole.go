package xlsx

import (
	"fmt"
	"sort"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// OLEObject is an embedded OLE object extracted from a workbook: an opaque
// binary part (typically /xl/embeddings/oleObjectN.bin) plus the metadata
// needed to identify it. The Data bytes are the object exactly as stored;
// spine does not parse the embedded OLE/CFB stream.
type OLEObject struct {
	// Name is the OPC part name of the embedded object.
	Name string
	// ContentType is the part's content type (usually opc.ContentTypeOLEObject).
	ContentType string
	// Data is the raw embedded object, carried verbatim.
	Data []byte
	// ProgID is the OLE server programmatic identifier declared by the
	// referencing element (e.g. "Excel.Sheet.12"), or "" when none is declared
	// in a form spine recognizes.
	ProgID string
}

// OLEObjects returns the workbook's embedded OLE objects. Objects are located
// through the package's oleObject relationships; any remaining
// /xl/embeddings/*.bin parts typed as OLE objects are included as a fallback.
// The result is ordered by part name for determinism. Extraction is read-only
// and leaves every part byte-for-byte unchanged on a subsequent save.
func (w *Workbook) OLEObjects() []OLEObject {
	seen := make(map[string]bool)
	var objects []OLEObject

	owners := make([]string, 0, len(w.relationships))
	for owner := range w.relationships {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	for _, owner := range owners {
		for _, rel := range w.relationships[owner] {
			if rel == nil || rel.Type != opc.RelTypeOLEObject || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			part, ok := w.preservedParts[target]
			if !ok || seen[target] {
				continue
			}
			seen[target] = true
			progID := ""
			if src, ok := w.preservedParts[owner]; ok {
				progID = coxml.ExtractOLEProgID(src.Data, rel.ID)
			}
			objects = append(objects, OLEObject{
				Name:        target,
				ContentType: part.ContentType,
				Data:        part.Data,
				ProgID:      progID,
			})
		}
	}

	names := make([]string, 0, len(w.preservedParts))
	for name := range w.preservedParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if seen[name] {
			continue
		}
		part := w.preservedParts[name]
		if part.ContentType != opc.ContentTypeOLEObject || !strings.Contains(strings.ToLower(name), "/embeddings/") {
			continue
		}
		seen[name] = true
		objects = append(objects, OLEObject{
			Name:        name,
			ContentType: part.ContentType,
			Data:        part.Data,
		})
	}

	sort.Slice(objects, func(i, j int) bool { return objects[i].Name < objects[j].Name })
	return objects
}

// OLEObjectSpec describes an OLE object to embed on a sheet with
// Sheet.AddOLEObject. Data is the object's raw OLE/CFB bytes (spine stores them
// verbatim and never parses them). The remaining fields are optional.
type OLEObjectSpec struct {
	// Data is the embedded object, carried verbatim (required, non-empty).
	Data []byte
	// ProgID is the OLE server programmatic identifier (e.g. "Word.Document.12",
	// "Package"). Defaults to "Package" when empty.
	ProgID string
	// ContentType is the embedding part's content type. Defaults to
	// opc.ContentTypeOLEObject.
	ContentType string
	// Ext is the embedding part's file extension without the dot (e.g. "bin",
	// "docx"). Defaults to "bin".
	Ext string
	// Anchor is the top-left cell the object is anchored to (e.g. "B2").
	// Defaults to "A1".
	Anchor string
	// Preview is an optional raster/metafile preview of the object shown on the
	// sheet (PNG/EMF bytes). When empty the object embeds without an on-sheet
	// image (it still opens and round-trips).
	Preview []byte
	// PreviewContentType is the preview image's content type (e.g.
	// opc.ContentTypePNG, opc.ContentTypeEMF). Required when Preview is set.
	PreviewContentType string
	// PreviewExt is the preview image's extension without the dot (e.g. "png",
	// "emf"). Required when Preview is set.
	PreviewExt string
}

// pendingOLE is one OLE object embedded this session, awaiting the save pass.
type pendingOLE struct {
	data        []byte
	progID      string
	contentType string
	ext         string
	row, col    int // 0-based top-left anchor cell
	preview     []byte
	previewCT   string
	previewExt  string
}

// AddOLEObject embeds an OLE object on the sheet: it writes the object as an
// embedding part, wires the worksheet <oleObjects> reference and its
// relationship, and generates a legacy VML shape (with the optional preview
// image) so Excel renders the object at the anchor cell.
//
// It authors the classic legacy embedding form (an <oleObject> with a matching
// VML Pict shape), which Excel opens without a repair prompt. The object is
// re-extractable through Workbook.OLEObjects and round-trips on save.
//
// Comments and OLE objects coexist: adding comments to the sheet after this
// call folds their note shapes into the same legacy VML drawing under one
// <legacyDrawing> (C283).
//
// Limitation: the reverse order is still rejected — a sheet that already carries
// comments, or a pre-existing <oleObjects>/legacyDrawing element (e.g. a form
// control), owns the single legacy VML drawing, and merging an authored object
// into an existing one is out of scope. Add the OLE object first (then comments),
// or add it on a sheet without those.
func (s *Sheet) AddOLEObject(spec OLEObjectSpec) error {
	if s.opaque {
		return ErrNotWorksheet
	}
	if len(spec.Data) == 0 {
		return fmt.Errorf("xlsx: AddOLEObject: Data must not be empty")
	}
	if len(spec.Preview) > 0 && (spec.PreviewContentType == "" || spec.PreviewExt == "") {
		return fmt.Errorf("xlsx: AddOLEObject: Preview requires PreviewContentType and PreviewExt")
	}
	if s.hasComments() {
		return fmt.Errorf("xlsx: AddOLEObject: sheet already has comments, which own its legacy VML drawing")
	}
	if s.ws() != nil && (s.ws().OleObjects != nil || s.ws().LegacyDrawing != nil) {
		return fmt.Errorf("xlsx: AddOLEObject: sheet already has an oleObjects/legacyDrawing element")
	}

	anchor := spec.Anchor
	if anchor == "" {
		anchor = "A1"
	}
	row, col, err := ParseCellRef(anchor)
	if err != nil {
		return fmt.Errorf("xlsx: AddOLEObject: invalid anchor %q: %w", spec.Anchor, err)
	}

	progID := spec.ProgID
	if progID == "" {
		progID = "Package"
	}
	ct := spec.ContentType
	if ct == "" {
		ct = opc.ContentTypeOLEObject
	}
	ext := spec.Ext
	if ext == "" {
		ext = "bin"
	}

	s.ensureWorksheet()
	s.oleEmbeds = append(s.oleEmbeds, pendingOLE{
		data:        spec.Data,
		progID:      progID,
		contentType: ct,
		ext:         ext,
		row:         row - 1,
		col:         col - 1,
		preview:     spec.Preview,
		previewCT:   spec.PreviewContentType,
		previewExt:  spec.PreviewExt,
	})
	s.markDirty()
	return nil
}

// sheetsHaveOLE reports whether any sheet has pending OLE embeds.
func (w *Workbook) sheetsHaveOLE() bool {
	for _, sheet := range w.sheets {
		if len(sheet.oleEmbeds) > 0 {
			return true
		}
	}
	return false
}

// writeSheetOLE writes the embedding parts and the legacy VML drawing for a
// sheet's pending OLE objects, wiring the object and VML relationships into
// sheetRels and populating the worksheet's typed <oleObjects> and
// <legacyDrawing> elements. The (possibly extended) sheetRels is returned.
func (w *Workbook) writeSheetOLE(
	writer *opc.Writer,
	sheet *Sheet,
	sheetRels []*opc.Relationship,
	relUsed map[string]struct{},
	used map[string]struct{},
	embedSeq, vmlSeq, mediaSeq *int,
) ([]*opc.Relationship, error) {
	nextRID := func() string {
		id := fmt.Sprintf("rId%d", nextRelationshipID(relUsed))
		relUsed[id] = struct{}{}
		return id
	}

	oleEl := &oxml.CT_OleObjects{Dirty: true}
	shapes := make([]oleShape, 0, len(sheet.oleEmbeds))

	// VML relationships (preview images) live in the VML part's own .rels; number
	// them independently of the sheet relationships.
	var vmlRels []*opc.Relationship
	vmlRelN := 0
	nextVMLRID := func() string { vmlRelN++; return fmt.Sprintf("rId%d", vmlRelN) }

	for i := range sheet.oleEmbeds {
		emb := &sheet.oleEmbeds[i]
		shapeID := uint32(1025 + i)

		embedPart := allocName(used, embedSeq, "/xl/embeddings/oleObject%d."+emb.ext)
		if err := writer.WritePart(embedPart, emb.contentType, emb.data); err != nil {
			return nil, err
		}
		embedRID := nextRID()
		sheetRels = ensureRel(sheetRels, embedRID, opc.RelTypeOLEObject, relTargetFromSheet(embedPart))

		shape := oleShape{shapeID: shapeID, row: emb.row, col: emb.col}
		if len(emb.preview) > 0 {
			previewPart := allocName(used, mediaSeq, "/xl/media/image%d."+emb.previewExt)
			if err := writer.WritePart(previewPart, emb.previewCT, emb.preview); err != nil {
				return nil, err
			}
			previewRID := nextVMLRID()
			vmlRels = append(vmlRels, &opc.Relationship{ID: previewRID, Type: opc.RelTypeImage, Target: relTargetFromSheet(previewPart)})
			shape.imageRID = previewRID
		}
		shapes = append(shapes, shape)

		oleEl.OleObject = append(oleEl.OleObject, oxml.CT_OleObject{
			ProgID:  emb.progID,
			ShapeID: shapeID,
			RID:     embedRID,
		})
	}

	// A sheet's comments and OLE objects share one legacy VML part. When the
	// sheet also has comment notes (writeSheetComments ran first and deferred the
	// VML to this path), render their note shapes into the same drawing so a
	// single <legacyDrawing> covers both. Their shape ids were reserved above the
	// OLE shapes' ids (1025..) by writeSheetComments (C283).
	noteShapes := ""
	if sc := sheet.comments; sc != nil && sc.mutated && sc.legacy != nil && len(sc.legacy.Comments) > 0 {
		noteShapes = buildCommentVMLShapes(sc.legacy)
	}

	// One VML drawing renders all of the sheet's OLE shapes (and any comment note
	// shapes); the worksheet <legacyDrawing> points at it.
	vmlPart := allocName(used, vmlSeq, "/xl/drawings/vmlDrawing%d.vml")
	if err := writer.WritePart(vmlPart, opc.ContentTypeVMLDrawing, buildOLEVML(shapes, noteShapes)); err != nil {
		return nil, err
	}
	if len(vmlRels) > 0 {
		if err := writer.WritePartRelationships(vmlPart, vmlRels); err != nil {
			return nil, err
		}
	}
	vmlRID := nextRID()
	sheetRels = ensureRel(sheetRels, vmlRID, opc.RelTypeVMLDrawing, relTargetFromSheet(vmlPart))

	sheet.ws().LegacyDrawing = &oxml.CT_LegacyDrawing{RID: vmlRID}
	sheet.ws().EnsureChildOrder("legacyDrawing")
	sheet.ws().OleObjects = oleEl
	sheet.ws().EnsureChildOrder("oleObjects")

	return sheetRels, nil
}

// oleShape is the geometry and relationship of one OLE object's VML shape.
type oleShape struct {
	shapeID  uint32
	row, col int // 0-based anchor cell
	imageRID string
}

// buildOLEVML renders the legacy VML drawing that gives each embedded OLE object
// a Pict shape anchored to its cell. Geometry is approximate (Excel recomputes
// it from column widths on open); the shape exists so the object's shapeId
// resolves and Excel renders it without a repair prompt.
//
// noteShapes, when non-empty, is the pre-rendered set of comment note-box shapes
// (from buildCommentVMLShapes) to fold into the same drawing so comments and OLE
// objects can coexist under one <legacyDrawing>; the note-box shapetype is
// emitted alongside them.
func buildOLEVML(shapes []oleShape, noteShapes string) []byte {
	var b strings.Builder
	b.WriteString(`<xml xmlns:v="urn:schemas-microsoft-com:vml"` + "\n")
	b.WriteString(` xmlns:o="urn:schemas-microsoft-com:office:office"` + "\n")
	b.WriteString(` xmlns:x="urn:schemas-microsoft-com:office:excel">` + "\n")
	b.WriteString(` <o:shapelayout v:ext="edit">` + "\n")
	b.WriteString(`  <o:idmap v:ext="edit" data="1"/>` + "\n")
	b.WriteString(` </o:shapelayout>`)
	b.WriteString(`<v:shapetype id="_x0000_t75" coordsize="21600,21600" o:spt="75"` + "\n")
	b.WriteString(`  o:preferrelative="t" path="m@4@5l@4@11@9@11@9@5xe" filled="f" stroked="f">` + "\n")
	b.WriteString(`  <v:stroke joinstyle="miter"/>` + "\n")
	b.WriteString(`  <v:path o:extrusionok="f" gradientshapeok="t" o:connecttype="rect"/>` + "\n")
	b.WriteString(`  <o:lock v:ext="edit" aspectratio="t"/>` + "\n")
	b.WriteString(` </v:shapetype>`)

	for i, sh := range shapes {
		marginLeft := (sh.col) * 48
		marginTop := sh.row * 15
		anchor := fmt.Sprintf("%d, 0, %d, 0, %d, 0, %d, 0", sh.col, sh.row, sh.col+2, sh.row+4)
		fmt.Fprintf(&b, `<v:shape id="_x0000_s%d" type="#_x0000_t75" style='position:absolute;`+"\n", sh.shapeID)
		fmt.Fprintf(&b, `  margin-left:%dpt;margin-top:%dpt;width:96pt;height:72pt;z-index:%d'`+"\n", marginLeft, marginTop, i+1)
		b.WriteString(`  o:ole="">` + "\n")
		if sh.imageRID != "" {
			fmt.Fprintf(&b, `  <v:imagedata o:relid="%s" o:title=""/>`+"\n", sh.imageRID)
		}
		b.WriteString(`  <x:ClientData ObjectType="Pict">` + "\n")
		b.WriteString(`   <x:MoveWithCells/>` + "\n")
		b.WriteString(`   <x:SizeWithCells/>` + "\n")
		fmt.Fprintf(&b, `   <x:Anchor>%s</x:Anchor>`+"\n", anchor)
		b.WriteString(`   <x:CF>Pict</x:CF>` + "\n")
		b.WriteString(`   <x:AutoPict/>` + "\n")
		b.WriteString(`  </x:ClientData>` + "\n")
		b.WriteString(` </v:shape>`)
	}
	if noteShapes != "" {
		b.WriteString(vmlNoteShapetype)
		b.WriteString(noteShapes)
	}
	b.WriteString(`</xml>` + "\n")
	return []byte(b.String())
}
