package docx

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// nsWordprocessingML is the WordprocessingML main namespace URI, used to
// namespace the w:object attributes written for an embedded OLE object.
const nsWordprocessingML = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// emuPerTwip is the number of EMU in one twentieth of a point (twip): 914400
// EMU per inch / 1440 twips per inch.
const emuPerTwip = 635

// oleShapeTypeVML is the standard VML shapetype (id _x0000_t75, an OLE/picture
// frame) that a v:shape references by type. Word emits it once alongside the
// object; including it keeps down-level renderers from failing to resolve the
// referenced type.
const oleShapeTypeVML = `<v:shapetype id="_x0000_t75" coordsize="21600,21600" o:spt="75" ` +
	`o:preferrelative="t" path="m@4@5l@4@11@9@11@9@5xe" filled="f" stroked="f">` +
	`<v:stroke joinstyle="miter"/>` +
	`<v:formulas>` +
	`<v:f eqn="if lineDrawn pixelLineWidth 0"/>` +
	`<v:f eqn="sum @0 1 0"/>` +
	`<v:f eqn="sum 0 0 @1"/>` +
	`<v:f eqn="prod @2 1 2"/>` +
	`<v:f eqn="prod @3 21600 pixelWidth"/>` +
	`<v:f eqn="prod @3 21600 pixelHeight"/>` +
	`<v:f eqn="sum @0 0 1"/>` +
	`<v:f eqn="prod @6 1 2"/>` +
	`<v:f eqn="prod @7 21600 pixelWidth"/>` +
	`<v:f eqn="sum @8 21600 0"/>` +
	`<v:f eqn="prod @7 21600 pixelHeight"/>` +
	`<v:f eqn="sum @10 21600 0"/>` +
	`</v:formulas>` +
	`<v:path o:extrusionok="f" gradientshapeok="t" o:connecttype="rect"/>` +
	`<o:lock v:ext="edit" aspectratio="t"/>` +
	`</v:shapetype>`

// OLEObject is an embedded OLE object extracted from a document: an opaque
// binary part (typically /word/embeddings/oleObjectN.bin) plus the metadata
// needed to identify it. The Data bytes are the object exactly as stored;
// spine does not parse the embedded OLE/CFB stream.
type OLEObject struct {
	// Name is the OPC part name of the embedded object (e.g.
	// "/word/embeddings/oleObject1.bin").
	Name string
	// ContentType is the part's content type. Embedded objects are usually
	// typed opc.ContentTypeOLEObject, but a specific server may use its own
	// (e.g. a binary Excel worksheet).
	ContentType string
	// Data is the raw embedded object, carried verbatim.
	Data []byte
	// ProgID is the OLE server programmatic identifier declared by the
	// referencing element (e.g. "Excel.Sheet.12"), or "" when the document
	// does not declare one in a form spine recognizes.
	ProgID string
}

// OLEObjects returns the document's embedded OLE objects. Objects are located
// through the package's oleObject relationships; any remaining
// /word/embeddings/*.bin parts typed as OLE objects are included as a fallback.
// The result is ordered by part name for determinism. Extraction is read-only
// and leaves every part byte-for-byte unchanged on a subsequent save.
func (d *Document) OLEObjects() []OLEObject {
	seen := make(map[string]bool)
	var objects []OLEObject

	// Deterministic iteration over the owning parts so ProgID resolution and
	// ordering do not depend on map order.
	owners := make([]string, 0, len(d.relationships))
	for owner := range d.relationships {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	for _, owner := range owners {
		for _, rel := range d.relationships[owner] {
			if rel == nil || rel.Type != opc.RelTypeOLEObject || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			part, ok := d.preservedParts[target]
			if !ok || seen[target] {
				continue
			}
			seen[target] = true
			progID := ""
			if src := d.rawPartData(owner); src != nil {
				progID = coxml.ExtractOLEProgID(src, rel.ID)
			}
			objects = append(objects, OLEObject{
				Name:        target,
				ContentType: part.ContentType,
				Data:        part.Data,
				ProgID:      progID,
			})
		}
	}

	// Fallback: embedded object parts that no oleObject relationship named but
	// that are typed as OLE objects still count as embedded objects.
	names := make([]string, 0, len(d.preservedParts))
	for name := range d.preservedParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if seen[name] {
			continue
		}
		part := d.preservedParts[name]
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

// --- embed ---

// Default OLE display size: 1 inch square (EMU, 914400 per inch).
const (
	defaultOLEWidthEMU  = 914400
	defaultOLEHeightEMU = 914400
)

// OLEEmbedOptions configures an embedded OLE object created with AddOLEObject.
// The zero value embeds the object displayed at 1in x 1in with a transparent
// placeholder icon, shown as content (not iconized).
type OLEEmbedOptions struct {
	// WidthEMU and HeightEMU are the display size in EMU (914400 per inch). When
	// zero a default of 1in x 1in is used.
	WidthEMU  int64
	HeightEMU int64
	// Icon is the presentation image shown in place of the object (PNG, EMF, or
	// WMF bytes). When empty a 1x1 transparent PNG placeholder is embedded.
	Icon []byte
	// IconContentType is the content type of Icon (e.g. opc.ContentTypePNG,
	// opc.ContentTypeEMF). Required when Icon is set; ignored otherwise.
	IconContentType string
	// DisplayAsIcon marks the object with DrawAspect="Icon" (shown as a small
	// program icon) instead of DrawAspect="Content".
	DisplayAsIcon bool
}

// AddOLEObject embeds an OLE object in a new run at the end of the document body
// and returns a handle to the stored part. It is a convenience wrapper over
// Paragraph.AddOLEObject.
func (d *Document) AddOLEObject(data []byte, progID string, opts OLEEmbedOptions) (*OLEObject, error) {
	return d.AddParagraph().AddOLEObject(data, progID, opts)
}

// AddOLEObject embeds an OLE object as a package part and inserts a w:object
// reference (a VML v:shape presentation image plus an o:OLEObject descriptor)
// into a new run at the end of the paragraph. data is the object stream (an
// OLE/CFB compound file) stored verbatim as /word/embeddings/oleObjectN.bin;
// progID names the server (e.g. "Excel.Sheet.12"). A presentation icon is
// embedded as an image part and referenced by the shape. The embedded object is
// reported by Document.OLEObjects() after a save/open round trip.
func (p *Paragraph) AddOLEObject(data []byte, progID string, opts OLEEmbedOptions) (*OLEObject, error) {
	doc := p.document
	if doc == nil {
		return nil, fmt.Errorf("paragraph is not attached to a document")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("ole object data is empty")
	}

	width := opts.WidthEMU
	if width <= 0 {
		width = defaultOLEWidthEMU
	}
	height := opts.HeightEMU
	if height <= 0 {
		height = defaultOLEHeightEMU
	}

	iconData := opts.Icon
	iconCT := opts.IconContentType
	if len(iconData) == 0 {
		iconData, iconCT = minimalTransparentPNG, opc.ContentTypePNG
	}
	iconExt := extForContentType(iconCT)
	if iconExt == "" {
		return nil, fmt.Errorf("unsupported OLE icon content type: %s", iconCT)
	}

	run := p.AddRun()
	owner := run.ownerPart()

	// Icon image part + relationship, then the embedded object part +
	// relationship, both scoped to the part that carries the run.
	iconRelID, _ := doc.registerImagePart(owner, iconData, iconCT, iconExt)
	objRelID, partName := doc.registerOLEPart(owner, data)

	obj := &OLEObject{
		Name:        partName,
		ContentType: opc.ContentTypeOLEObject,
		Data:        data,
		ProgID:      progID,
	}

	id := doc.nextShapeID()
	run.r.AppendObject(buildOLEObjectXML(id, width, height, progID, objRelID, iconRelID, opts.DisplayAsIcon))
	return obj, nil
}

// registerOLEPart stores an embedded OLE object part and registers an oleObject
// relationship in the owning part's scope, returning the relationship id and the
// part name. The part is written by writeAddedParts alongside image parts.
func (d *Document) registerOLEPart(owner string, data []byte) (relID, partName string) {
	relID = fmt.Sprintf("rId%d", d.nextRelIDForPart(owner))
	partName = fmt.Sprintf("/word/embeddings/oleObject%d.bin", d.nextEmbeddingNumber())
	d.imageParts = append(d.imageParts, &imagePart{
		data:        data,
		contentType: opc.ContentTypeOLEObject,
		partName:    partName,
		relID:       relID,
		owner:       owner,
	})
	d.addPartRelationship(owner, &opc.Relationship{
		ID:     relID,
		Type:   opc.RelTypeOLEObject,
		Target: partName[len("/word/"):],
	})
	return relID, partName
}

// nextEmbeddingNumber returns the smallest positive N for which no
// /word/embeddings/oleObjectN.bin part exists, scanning both the parts
// preserved from an opened package and objects added earlier in this session.
func (d *Document) nextEmbeddingNumber() int {
	used := make(map[int]bool)
	mark := func(name string) {
		const prefix = "/word/embeddings/oleobject"
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, prefix) || !strings.HasSuffix(lower, ".bin") {
			return
		}
		numStr := lower[len(prefix) : len(lower)-len(".bin")]
		if n, err := strconv.Atoi(numStr); err == nil && n > 0 {
			used[n] = true
		}
	}
	for name := range d.preservedParts {
		mark(name)
	}
	for name := range d.otherParts {
		mark(name)
	}
	for _, ip := range d.imageParts {
		mark(ip.partName)
	}
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

// buildOLEObjectXML builds a w:object element (as a raw run child) for an
// embedded OLE object: the VML shapetype and a v:shape presentation image
// referencing the icon relationship, plus an o:OLEObject descriptor referencing
// the embedded object relationship and declaring the ProgID. The VML (v/o)
// namespaces are declared on the w:object element so every descendant resolves;
// r: is bound at the document root.
func buildOLEObjectXML(id int, widthEMU, heightEMU int64, progID, objRelID, iconRelID string, asIcon bool) *oxml.CT_RawElement {
	dxaOrig := widthEMU / emuPerTwip
	dyaOrig := heightEMU / emuPerTwip
	widthPt := emuToPoints(widthEMU)
	heightPt := emuToPoints(heightEMU)
	shapeID := fmt.Sprintf("_x0000_i%d", 1024+id)
	objectID := fmt.Sprintf("_%d", 1000000000+id)

	drawAspect := "Content"
	if asIcon {
		drawAspect = "Icon"
	}

	raw := oleShapeTypeVML +
		fmt.Sprintf(
			`<v:shape id="%s" type="#_x0000_t75" style="width:%.2fpt;height:%.2fpt" o:ole="">`+
				`<v:imagedata r:id="%s" o:title=""/>`+
				`</v:shape>`,
			shapeID, widthPt, heightPt, iconRelID) +
		fmt.Sprintf(
			`<o:OLEObject Type="Embed" ProgID="%s" ShapeID="%s" DrawAspect="%s" ObjectID="%s" r:id="%s"/>`,
			xmlEscapeAttr(progID), shapeID, drawAspect, objectID, objRelID)

	return &oxml.CT_RawElement{
		Attrs: []xml.Attr{
			{Name: xml.Name{Space: nsWordprocessingML, Local: "dxaOrig"}, Value: strconv.FormatInt(dxaOrig, 10)},
			{Name: xml.Name{Space: nsWordprocessingML, Local: "dyaOrig"}, Value: strconv.FormatInt(dyaOrig, 10)},
			{Name: xml.Name{Space: "xmlns", Local: "v"}, Value: "urn:schemas-microsoft-com:vml"},
			{Name: xml.Name{Space: "xmlns", Local: "o"}, Value: "urn:schemas-microsoft-com:office:office"},
		},
		RawContent: []byte(raw),
	}
}
