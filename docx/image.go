package docx

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// svgBlipExtURI is the Office drawing extension GUID that carries the SVG
// variant of an image alongside its raster fallback (matches the PPTX path).
const svgBlipExtURI = "{96DAC541-7B7A-43D3-8B79-37D633B846F1}"

// nsDrawingSVG2016 is the namespace of the asvg:svgBlip element.
const nsDrawingSVG2016 = "http://schemas.microsoft.com/office/drawing/2016/SVG/main"

// InlineImage represents an image in a document. Despite the historical name
// it covers both inline images (in the text flow) and floating/anchored ones
// (positioned relative to the page); floating is set when the image was added
// with AddFloatingImage*.
type InlineImage struct {
	relID     string
	svgRelID  string // set when the image also carries an SVG variant
	widthEMU  int64
	heightEMU int64
	altText   string
	drawingID uint32
	run       *Run
	// drawing is the w:drawing element this handle was created for. Updates
	// are applied to it directly: matching by position in the run would let a
	// handle for the second image overwrite the first one's drawing.
	drawing *oxml.CT_Drawing

	floating bool
	anchor   Anchor
}

// Anchor positions a floating image relative to the page or the surrounding
// text. The zero value anchors relative to the column/paragraph at offset
// (0,0), in front of the text.
type Anchor struct {
	// RelativeToPage anchors X/Y to the page origin instead of the column
	// (horizontal) and paragraph (vertical).
	RelativeToPage bool
	// X and Y are the offsets from the anchor origin, in points.
	X, Y float64
	// BehindText places the image behind the text (e.g. a watermark) instead
	// of in front of it.
	BehindText bool
}

// SetSize sets the image size in points.
func (img *InlineImage) SetSize(widthPt, heightPt float64) {
	img.touch()
	img.widthEMU = int64(math.Round(widthPt * 914400 / 72))
	img.heightEMU = int64(math.Round(heightPt * 914400 / 72))
	img.updateDrawingXML()
}

// SetAltText sets the alt text description for the image.
func (img *InlineImage) SetAltText(text string) {
	img.touch()
	img.altText = text
	img.updateDrawingXML()
}

// touch flags the header/footer part this image belongs to as modified, so an
// edit made through a live handle into a reopened header/footer is written back
// instead of being masked by the preserved original bytes. A no-op for images
// in the main document part.
func (img *InlineImage) touch() {
	if img != nil && img.run != nil {
		img.run.touch()
	}
}

func (img *InlineImage) updateDrawingXML() {
	if img.drawing == nil {
		return
	}
	// Regenerate the drawing this handle owns (inline or anchored); other
	// drawings in the same run are untouched.
	img.drawing.RawContent = img.buildDrawingXML()
}

// pointsToEMU converts points to EMU (914400 EMU per inch, 72 pt per inch).
func pointsToEMU(pt float64) int64 {
	return int64(math.Round(pt * 914400 / 72))
}

// buildDrawingXML builds the drawing content (wp:inline or wp:anchor) for the
// image, shared between the two placement modes.
func (img *InlineImage) buildDrawingXML() []byte {
	if img.floating {
		return img.buildAnchorXML()
	}
	return img.buildInlineXML()
}

// blipXML builds the a:blip element, including the svgBlip extension when the
// image carries an SVG variant. The raster relationship is always the primary
// r:embed (what Word without SVG support renders); the SVG is the extension.
func (img *InlineImage) blipXML() string {
	if img.svgRelID == "" {
		return fmt.Sprintf(`<a:blip r:embed="%s"/>`, img.relID)
	}
	return fmt.Sprintf(
		`<a:blip r:embed="%s">`+
			`<a:extLst>`+
			`<a:ext uri="%s">`+
			`<asvg:svgBlip xmlns:asvg="%s" r:embed="%s"/>`+
			`</a:ext>`+
			`</a:extLst>`+
			`</a:blip>`,
		img.relID, svgBlipExtURI, nsDrawingSVG2016, img.svgRelID)
}

// picGraphicXML builds the shared <a:graphic> ... </a:graphic> pic fragment.
func (img *InlineImage) picGraphicXML() string {
	return fmt.Sprintf(
		`<a:graphic>`+
			`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:pic>`+
			`<pic:nvPicPr>`+
			`<pic:cNvPr id="%d" name="Picture %d"/>`+
			`<pic:cNvPicPr/>`+
			`</pic:nvPicPr>`+
			`<pic:blipFill>`+
			`%s`+
			`<a:stretch><a:fillRect/></a:stretch>`+
			`</pic:blipFill>`+
			`<pic:spPr>`+
			`<a:xfrm>`+
			`<a:off x="0" y="0"/>`+
			`<a:ext cx="%d" cy="%d"/>`+
			`</a:xfrm>`+
			`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`+
			`</pic:spPr>`+
			`</pic:pic>`+
			`</a:graphicData>`+
			`</a:graphic>`,
		img.drawingID, img.drawingID,
		img.blipXML(),
		img.widthEMU, img.heightEMU,
	)
}

const drawingNamespaces = `xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

func (img *InlineImage) buildInlineXML() []byte {
	altText := xmlEscapeAttr(img.altText)
	xml := fmt.Sprintf(
		`<wp:inline distT="0" distB="0" distL="0" distR="0" `+drawingNamespaces+`>`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:docPr id="%d" name="Picture %d" descr="%s"/>`+
			`%s`+
			`</wp:inline>`,
		img.widthEMU, img.heightEMU,
		img.drawingID, img.drawingID, altText,
		img.picGraphicXML(),
	)
	return []byte(xml)
}

func (img *InlineImage) buildAnchorXML() []byte {
	altText := xmlEscapeAttr(img.altText)

	behindDoc := "0"
	if img.anchor.BehindText {
		behindDoc = "1"
	}
	// A floating image over/under text uses wrapNone; behindDoc decides
	// whether it sits behind or in front of the text.
	hRel, vRel := "column", "paragraph"
	if img.anchor.RelativeToPage {
		hRel, vRel = "page", "page"
	}
	// relativeHeight is the stacking order; deriving it from the drawing id
	// keeps multiple anchored images from sharing a z-index while staying
	// deterministic.
	relHeight := 251658240 + int64(img.drawingID)

	xml := fmt.Sprintf(
		`<wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" `+
			`relativeHeight="%d" behindDoc="%s" locked="0" layoutInCell="1" allowOverlap="1" `+
			drawingNamespaces+`>`+
			`<wp:simplePos x="0" y="0"/>`+
			`<wp:positionH relativeFrom="%s"><wp:posOffset>%d</wp:posOffset></wp:positionH>`+
			`<wp:positionV relativeFrom="%s"><wp:posOffset>%d</wp:posOffset></wp:positionV>`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:wrapNone/>`+
			`<wp:docPr id="%d" name="Picture %d" descr="%s"/>`+
			`%s`+
			`</wp:anchor>`,
		relHeight, behindDoc,
		hRel, pointsToEMU(img.anchor.X),
		vRel, pointsToEMU(img.anchor.Y),
		img.widthEMU, img.heightEMU,
		img.drawingID, img.drawingID, altText,
		img.picGraphicXML(),
	)
	return []byte(xml)
}

// xmlEscapeAttr escapes a string for use in an XML attribute value.
func xmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// imagePart stores an image to be written to the package.
type imagePart struct {
	data        []byte
	contentType string
	partName    string
	relID       string
	owner       string // source part of the relationship (document or header/footer part)
}

// AddImage adds an inline image from a file path to the run. SVG files are
// supported and embedded with a transparent raster fallback (use AddSVGImage
// to supply your own fallback).
func (r *Run) AddImage(path string) (*InlineImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	ct := contentTypeForExt(ext)
	if ct == "" {
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}
	if ct == opc.ContentTypeSVG {
		return r.addSVGImageData(data, minimalTransparentPNG, opc.ContentTypePNG, Anchor{}, false)
	}
	return r.addImageData(data, ct, ext, Anchor{}, false)
}

// AddImageFromBytes adds an inline image from raw bytes to the run. Pass an
// "image/svg+xml" content type to embed an SVG (with a transparent raster
// fallback).
func (r *Run) AddImageFromBytes(data []byte, contentType string) (*InlineImage, error) {
	if contentType == opc.ContentTypeSVG {
		return r.addSVGImageData(data, minimalTransparentPNG, opc.ContentTypePNG, Anchor{}, false)
	}
	ext := extForContentType(contentType)
	if ext == "" {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
	return r.addImageData(data, contentType, ext, Anchor{}, false)
}

// AddFloatingImage adds a floating (page/paragraph-anchored) image from a file
// path, positioned by anchor. SVG is supported with a transparent fallback.
func (r *Run) AddFloatingImage(path string, anchor Anchor) (*InlineImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	ct := contentTypeForExt(ext)
	if ct == "" {
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}
	if ct == opc.ContentTypeSVG {
		return r.addSVGImageData(data, minimalTransparentPNG, opc.ContentTypePNG, anchor, true)
	}
	return r.addImageData(data, ct, ext, anchor, true)
}

// AddFloatingImageFromBytes adds a floating image from raw bytes, positioned
// by anchor (e.g. a cover-page logo or a behind-text watermark).
func (r *Run) AddFloatingImageFromBytes(data []byte, contentType string, anchor Anchor) (*InlineImage, error) {
	if contentType == opc.ContentTypeSVG {
		return r.addSVGImageData(data, minimalTransparentPNG, opc.ContentTypePNG, anchor, true)
	}
	ext := extForContentType(contentType)
	if ext == "" {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
	return r.addImageData(data, contentType, ext, anchor, true)
}

// AddSVGImage adds an inline SVG image from bytes with a caller-supplied raster
// fallback (shown by viewers that cannot render SVG). Use AddImageFromBytes
// with an "image/svg+xml" content type for the transparent-fallback shorthand.
func (r *Run) AddSVGImage(svgData, fallbackData []byte, fallbackContentType string) (*InlineImage, error) {
	return r.addSVGImageData(svgData, fallbackData, fallbackContentType, Anchor{}, false)
}

// AddFloatingSVGImage adds a floating SVG image with a caller-supplied raster
// fallback, positioned by anchor.
func (r *Run) AddFloatingSVGImage(svgData, fallbackData []byte, fallbackContentType string, anchor Anchor) (*InlineImage, error) {
	return r.addSVGImageData(svgData, fallbackData, fallbackContentType, anchor, true)
}

// ownerPart names the part that contains the run's drawing: document.xml for
// body paragraphs, the header/footer part for paragraphs in a header or
// footer — an r:embed only resolves through the rels of the part it appears
// in.
func (r *Run) ownerPart() string {
	if r.paragraph.hfPart != "" {
		return r.paragraph.hfPart
	}
	return r.paragraph.document.mainPart()
}

// registerImagePart stores an image part and registers its relationship in
// the owning part's scope (document.xml or a header/footer part), returning
// the relationship id and the image number. The part number is derived from
// parts already in the package, so adding to an opened document that already
// carries media/imageN.* does not collide (audit C155).
func (doc *Document) registerImagePart(owner string, data []byte, contentType, ext string) (relID string, num int) {
	relID = fmt.Sprintf("rId%d", doc.nextRelID())

	// Reuse an identical image already added in this session (e.g. the same
	// picture in the body and a header) instead of writing a duplicate media
	// part; each placement still gets its own relationship in its own part.
	partName := ""
	for _, existing := range doc.imageParts {
		if existing.contentType == contentType && bytes.Equal(existing.data, data) {
			partName = existing.partName
			break
		}
	}
	if partName == "" {
		partName = fmt.Sprintf("/word/media/image%d%s", doc.nextImageNumber(), ext)
		doc.imageParts = append(doc.imageParts, &imagePart{
			data:        data,
			contentType: contentType,
			partName:    partName,
			relID:       relID,
			owner:       owner,
		})
	}

	// Add relationship in the owning part's scope. Header/footer parts live in
	// /word/ like document.xml, so the media target is relative to /word/ in
	// both cases.
	doc.addPartRelationship(owner, &opc.Relationship{
		ID:     relID,
		Type:   opc.RelTypeImage,
		Target: partName[len("/word/"):],
	})
	return relID, imageNumberFromPartName(partName)
}

func (r *Run) addImageData(data []byte, contentType, ext string, anchor Anchor, floating bool) (*InlineImage, error) {
	doc := r.paragraph.document
	if doc == nil {
		return nil, fmt.Errorf("run is not attached to a document")
	}

	relID, num := doc.registerImagePart(r.ownerPart(), data, contentType, ext)

	img := &InlineImage{
		relID:     relID,
		widthEMU:  int64(100 * 914400 / 72), // 100pt default; override via SetSize
		heightEMU: int64(100 * 914400 / 72),
		drawingID: uint32(num),
		run:       r,
		floating:  floating,
		anchor:    anchor,
	}
	// Add drawing element to run and bind the handle to it, so later
	// SetSize/SetAltText calls update this drawing and not another one in the
	// same run (C25).
	drawing := &oxml.CT_Drawing{RawContent: img.buildDrawingXML()}
	img.drawing = drawing
	r.r.AppendDrawing(drawing)
	// A drawing added to a reopened header/footer run must regenerate that part
	// on save; otherwise the media part and its relationship are written but the
	// preserved header bytes (lacking the drawing) win, orphaning both.
	r.touch()
	return img, nil
}

// addSVGImageData embeds an SVG plus its raster fallback (two parts, two
// relationships) and builds a picture whose a:blip references the raster with
// an svgBlip extension referencing the SVG.
func (r *Run) addSVGImageData(svgData, fallbackData []byte, fallbackCT string, anchor Anchor, floating bool) (*InlineImage, error) {
	doc := r.paragraph.document
	if doc == nil {
		return nil, fmt.Errorf("run is not attached to a document")
	}
	if len(svgData) == 0 {
		return nil, fmt.Errorf("svg image data is empty")
	}
	fallbackExt := extForContentType(fallbackCT)
	if fallbackExt == "" || fallbackCT == opc.ContentTypeSVG {
		return nil, fmt.Errorf("unsupported raster fallback content type: %s", fallbackCT)
	}

	// Raster fallback part first (the primary r:embed), then the SVG part.
	// Both relationships are registered on the owning part, so an SVG image
	// placed in a header resolves both the raster and SVG rels from the
	// header part's own rels.
	owner := r.ownerPart()
	rasterRelID, num := doc.registerImagePart(owner, fallbackData, fallbackCT, fallbackExt)
	svgRelID, _ := doc.registerImagePart(owner, svgData, opc.ContentTypeSVG, ".svg")

	img := &InlineImage{
		relID:     rasterRelID,
		svgRelID:  svgRelID,
		widthEMU:  int64(100 * 914400 / 72),
		heightEMU: int64(100 * 914400 / 72),
		drawingID: uint32(num),
		run:       r,
		floating:  floating,
		anchor:    anchor,
	}
	// Bind the handle to its own drawing (C25), covering anchored drawings
	// too via buildDrawingXML.
	drawing := &oxml.CT_Drawing{RawContent: img.buildDrawingXML()}
	img.drawing = drawing
	r.r.AppendDrawing(drawing)
	// See addImageData: flag a reopened header/footer part so its regenerated
	// bytes carry the new drawing (and its image relationships resolve).
	r.touch()
	return img, nil
}

// imageNumberFromPartName extracts N from /word/media/imageN.ext, returning 0
// if the name does not have that form.
func imageNumberFromPartName(partName string) int {
	const prefix = "/word/media/image"
	if !strings.HasPrefix(partName, prefix) {
		return 0
	}
	rest := partName[len(prefix):]
	dot := strings.IndexByte(rest, '.')
	if dot <= 0 {
		return 0
	}
	n, err := strconv.Atoi(rest[:dot])
	if err != nil {
		return 0
	}
	return n
}

func contentTypeForExt(ext string) string {
	switch ext {
	case ".png":
		return opc.ContentTypePNG
	case ".jpg", ".jpeg":
		return opc.ContentTypeJPEG
	case ".gif":
		return opc.ContentTypeGIF
	case ".svg":
		return opc.ContentTypeSVG
	default:
		return ""
	}
}

func extForContentType(ct string) string {
	switch ct {
	case opc.ContentTypePNG:
		return ".png"
	case opc.ContentTypeJPEG:
		return ".jpg"
	case opc.ContentTypeGIF:
		return ".gif"
	case opc.ContentTypeSVG:
		return ".svg"
	default:
		return ""
	}
}

// minimalTransparentPNG is a 1x1 transparent PNG used as the raster fallback
// for SVG images added without an explicit fallback.
var minimalTransparentPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x60, 0x00, 0x02, 0x00,
	0x00, 0x05, 0x00, 0x01, 0xe2, 0x26, 0x05, 0x9b,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
	0xae, 0x42, 0x60, 0x82,
}
