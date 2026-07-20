package pptx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// oleGraphicDataURI is the a:graphicData/@uri that marks a graphic frame as an
// embedded OLE object (CT_OleObject in the p: namespace).
const oleGraphicDataURI = "http://schemas.openxmlformats.org/presentationml/2006/ole"

// OLEObject is an embedded OLE object extracted from a presentation: an opaque
// binary part (typically /ppt/embeddings/oleObjectN.bin) plus the metadata
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
	// referencing slide (e.g. "Excel.Sheet.12"), or "" when none is declared in
	// a form spine recognizes.
	ProgID string
}

// OLEObjects returns the presentation's embedded OLE objects. Objects are
// located through the package's oleObject relationships; any remaining
// /ppt/embeddings/*.bin parts typed as OLE objects are included as a fallback.
// The result is ordered by part name for determinism. Extraction is read-only
// and leaves every part byte-for-byte unchanged on a subsequent save.
func (p *Presentation) OLEObjects() []OLEObject {
	seen := make(map[string]bool)
	var objects []OLEObject

	// progID declared on a p:oleObj in a parsed slide's shape tree, keyed by the
	// embedded object part it references. This recovers the progID for objects
	// created via AddOLEObject (and any parsed OLE frame) whose owner slide no
	// longer retains its source bytes.
	frameProgIDs := p.oleProgIDsByPart()

	owners := make([]string, 0, len(p.relationships))
	for owner := range p.relationships {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	for _, owner := range owners {
		for _, rel := range p.relationships[owner] {
			if rel == nil || rel.Type != opc.RelTypeOLEObject || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			part, ok := p.otherParts[target]
			if !ok || seen[target] {
				continue
			}
			seen[target] = true
			progID := ""
			if src, ok := p.otherParts[owner]; ok {
				// ProgID is available only when the referencing part is carried
				// raw (e.g. a preserved unreferenced slide). Slides parsed into
				// the model do not retain source bytes, so ProgID is best-effort
				// and may be "" for a typical embedded object.
				progID = coxml.ExtractOLEProgID(src.Data, rel.ID)
			}
			if progID == "" {
				progID = frameProgIDs[target]
			}
			objects = append(objects, OLEObject{
				Name:        target,
				ContentType: part.ContentType,
				Data:        part.Data,
				ProgID:      progID,
			})
		}
	}

	names := make([]string, 0, len(p.otherParts))
	for name := range p.otherParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if seen[name] {
			continue
		}
		part := p.otherParts[name]
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

// oleProgIDsByPart scans every parsed slide's shape tree for OLE-object graphic
// frames (a:graphicData uri=".../ole") and maps each embedded object part to the
// progID declared on its p:oleObj. It lets OLEObjects report a progID for
// objects whose owner slide no longer carries source bytes.
func (p *Presentation) oleProgIDsByPart() map[string]string {
	out := make(map[string]string)
	for _, s := range p.slides {
		if s == nil || s.slideXML == nil || s.slideXML.CSld == nil || s.slideXML.CSld.SpTree == nil {
			continue
		}
		for _, gf := range s.slideXML.CSld.SpTree.GraphicFrame {
			if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil {
				continue
			}
			gd := gf.Graphic.GraphicData
			if gd.URI != oleGraphicDataURI || len(gd.RawContent) == 0 {
				continue
			}
			relID := attrValueFromRaw(gd.RawContent, "r:id")
			progID := attrValueFromRaw(gd.RawContent, "progId")
			if relID == "" || progID == "" {
				continue
			}
			if target := s.relTargetPart(relID); target != "" {
				out[target] = progID
			}
		}
	}
	return out
}

// OLEObjectFrame is the shape an embedded OLE object created via
// Slide.AddOLEObject occupies on a slide: a p:graphicFrame whose graphicData
// (uri=".../ole") carries a p:oleObj that references the embedded object part by
// relationship id and a fallback preview picture. AddOLEObject creates the
// object part, the preview image part, and the wiring relationships eagerly;
// this frame only records geometry and the relationship ids.
type OLEObjectFrame struct {
	BaseShape

	// oleRelID is the slide relationship (RelTypeOLEObject) to the embedded
	// object part; imageRelID is the image relationship to the preview picture.
	oleRelID, imageRelID string
	progID               string
	// partName is the absolute OLE object part name (e.g.
	// /ppt/embeddings/oleObject1.bin).
	partName string
	// showAsIcon renders the object as its application icon rather than a preview.
	showAsIcon bool
}

// ShapeType reports that an OLEObjectFrame is an embedded OLE object.
func (of *OLEObjectFrame) ShapeType() ShapeType { return ShapeTypeOLEObject }

// oleObjectConfig holds the resolved settings for a created OLE object.
type oleObjectConfig struct {
	x, y, width, height int64
	name                string
	contentType         string
	previewData         []byte
	previewCT           string
	showAsIcon          bool
}

// OLEObjectOption configures Slide.AddOLEObject.
type OLEObjectOption func(*oleObjectConfig)

// WithOLEBounds positions and sizes the object's frame, in EMUs. The default
// fills a 3"×2" region near the top-left of the slide.
func WithOLEBounds(x, y, width, height int64) OLEObjectOption {
	return func(c *oleObjectConfig) { c.x, c.y, c.width, c.height = x, y, width, height }
}

// WithOLEName sets the frame's display name (p:cNvPr/@name and p:oleObj/@name).
func WithOLEName(name string) OLEObjectOption {
	return func(c *oleObjectConfig) { c.name = name }
}

// WithOLEContentType overrides the content type stored for the embedded object
// part. It defaults to the generic embedded-OLE content type; some producers use
// a progID-specific type.
func WithOLEContentType(ct string) OLEObjectOption {
	return func(c *oleObjectConfig) { c.contentType = ct }
}

// WithOLEPreviewImage sets the fallback preview picture shown in place of the
// object until it is activated. contentType is the image MIME type (e.g.
// "image/png"). PowerPoint requires a preview picture on an OLE object; when
// none is supplied AddOLEObject embeds a minimal transparent placeholder.
func WithOLEPreviewImage(data []byte, contentType string) OLEObjectOption {
	return func(c *oleObjectConfig) { c.previewData, c.previewCT = data, contentType }
}

// WithOLEShowAsIcon renders the object as its application icon rather than a
// content preview.
func WithOLEShowAsIcon(showAsIcon bool) OLEObjectOption {
	return func(c *oleObjectConfig) { c.showAsIcon = showAsIcon }
}

// Default OLE frame placement and size (EMUs): a 3"×2" box near the top-left.
const (
	oleDefaultX      = 838200
	oleDefaultY      = 838200
	oleDefaultWidth  = 2743200
	oleDefaultHeight = 1828800
)

// AddOLEObject embeds an OLE object on the slide: it stores the object's binary
// payload (typically an OLE/CFB compound file) as an /ppt/embeddings/oleObjectN.bin
// part, embeds a fallback preview picture, wires the slide relationships, and
// appends a p:graphicFrame (a:graphicData uri=".../ole") whose p:oleObj ties
// them together. progID is the OLE server programmatic identifier (e.g.
// "Excel.Sheet.12", "Word.Document.12", or "Package" for a generic package).
//
// The object fills a default 3"×2" region; pass WithOLEBounds to place it.
// PowerPoint requires a preview picture, so a minimal transparent placeholder is
// embedded unless WithOLEPreviewImage supplies one. AddOLEObject returns the
// created frame, or an error when the slide is not attached to a presentation or
// the payload/progID is empty.
func (s *Slide) AddOLEObject(data []byte, progID string, opts ...OLEObjectOption) (*OLEObjectFrame, error) {
	p := s.presentation
	if p == nil {
		return nil, fmt.Errorf("pptx: AddOLEObject: slide is not attached to a presentation")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("pptx: AddOLEObject: object data is empty")
	}
	if progID == "" {
		return nil, fmt.Errorf("pptx: AddOLEObject: progID is empty")
	}
	if s.partName == "" {
		s.partName = p.nextAvailableSlidePartName()
	}

	cfg := oleObjectConfig{
		x: oleDefaultX, y: oleDefaultY, width: oleDefaultWidth, height: oleDefaultHeight,
		contentType: opc.ContentTypeOLEObject,
		previewData: minimalTransparentPNG, previewCT: "image/png",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if len(cfg.previewData) == 0 {
		cfg.previewData, cfg.previewCT = minimalTransparentPNG, "image/png"
	}
	if cfg.contentType == "" {
		cfg.contentType = opc.ContentTypeOLEObject
	}

	// Store the embedded object part and its slide relationship.
	olePart := p.nextEmbeddedOLEName()
	p.otherParts[olePart] = &coxml.RawPart{ContentType: cfg.contentType, Data: data}
	oleRelID := s.nextRelID()
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         oleRelID,
		Type:       opc.RelTypeOLEObject,
		Target:     relativeTarget(s.partName, olePart),
		TargetMode: opc.TargetModeInternal,
	})

	// Embed the fallback preview picture (reusing an identical media part).
	imageRelID := s.embedImageData(cfg.previewData, cfg.previewCT)

	of := &OLEObjectFrame{
		oleRelID:   oleRelID,
		imageRelID: imageRelID,
		progID:     progID,
		partName:   olePart,
		showAsIcon: cfg.showAsIcon,
	}
	if cfg.name != "" {
		of.SetName(cfg.name)
	}
	of.SetPosition(dml.EMU(cfg.x), dml.EMU(cfg.y))
	of.SetSize(dml.EMU(cfg.width), dml.EMU(cfg.height))
	s.addShape(of)
	return of, nil
}

// nextEmbeddedOLEName returns an unused /ppt/embeddings/oleObjectN.bin part name.
func (p *Presentation) nextEmbeddedOLEName() string {
	return p.nextIndexedPartName("/ppt/embeddings/oleObject", ".bin")
}

// oleObjectFrameToOxml converts an OLEObjectFrame to the p:graphicFrame element
// that hosts the embedded object. The a:graphicData for an OLE object carries a
// p:oleObj, which the shared graphicData model preserves as raw content, so the
// frame is serialized by generating those bytes.
func oleObjectFrameToOxml(of *OLEObjectFrame, id uint32) *oxml.GraphicFrame {
	name := of.Name()
	if name == "" {
		name = "Object " + strconv.FormatUint(uint64(id), 10)
	}
	x, y := of.Position()
	w, h := of.Size()

	var b strings.Builder
	b.WriteString(`<p:oleObj name="`)
	b.WriteString(xmlb.EscapeAttrValue(name))
	b.WriteString(`"`)
	if of.showAsIcon {
		b.WriteString(` showAsIcon="1"`)
	}
	b.WriteString(` r:id="`)
	b.WriteString(of.oleRelID)
	b.WriteString(`" imgW="`)
	b.WriteString(strconv.FormatInt(int64(w), 10))
	b.WriteString(`" imgH="`)
	b.WriteString(strconv.FormatInt(int64(h), 10))
	b.WriteString(`" progId="`)
	b.WriteString(xmlb.EscapeAttrValue(of.progID))
	b.WriteString(`"><p:embed/>`)
	// Fallback preview picture (required by CT_OleObject).
	b.WriteString(`<p:pic><p:nvPicPr><p:cNvPr id="0" name=""/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>`)
	b.WriteString(`<p:blipFill><a:blip r:embed="`)
	b.WriteString(of.imageRelID)
	b.WriteString(`"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>`)
	b.WriteString(`<p:spPr><a:xfrm><a:off x="`)
	b.WriteString(strconv.FormatInt(int64(x), 10))
	b.WriteString(`" y="`)
	b.WriteString(strconv.FormatInt(int64(y), 10))
	b.WriteString(`"/><a:ext cx="`)
	b.WriteString(strconv.FormatInt(int64(w), 10))
	b.WriteString(`" cy="`)
	b.WriteString(strconv.FormatInt(int64(h), 10))
	b.WriteString(`"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic></p:oleObj>`)

	return &oxml.GraphicFrame{
		NvGraphicFramePr: &oxml.NvGraphicFramePr{
			CNvPr: &dml.CNvPr{Id: id, Name: name},
			CNvGraphicFramePr: &oxml.CNvGraphicFramePr{
				GraphicFrameLocks: &oxml.GraphicFrameLocks{NoGrp: true},
			},
			NvPr: &oxml.NvPr{},
		},
		Xfrm: &dml.Xfrm{
			Off: &dml.OffXML{X: int64(x), Y: int64(y)},
			Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
		},
		Graphic: &oxml.AGraphic{
			GraphicData: &oxml.AGraphicData{
				URI:        oleGraphicDataURI,
				RawContent: []byte(b.String()),
			},
		},
	}
}
