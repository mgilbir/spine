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

// InlineImage represents an inline image in a document.
type InlineImage struct {
	relID         string
	widthEMU      int64
	heightEMU     int64
	altText       string
	drawingID     uint32
	run           *Run
}

// SetSize sets the image size in points.
func (img *InlineImage) SetSize(widthPt, heightPt float64) {
	img.widthEMU = int64(math.Round(widthPt * 914400 / 72))
	img.heightEMU = int64(math.Round(heightPt * 914400 / 72))
	img.updateDrawingXML()
}

// SetAltText sets the alt text description for the image.
func (img *InlineImage) SetAltText(text string) {
	img.altText = text
	img.updateDrawingXML()
}

func (img *InlineImage) updateDrawingXML() {
	if img.run == nil {
		return
	}
	// Find and update the drawing in the run
	for _, d := range img.run.r.Drawing {
		if d.RawContent != nil {
			d.RawContent = img.buildInlineXML()
			return
		}
	}
}

func (img *InlineImage) buildInlineXML() []byte {
	altText := xmlEscapeAttr(img.altText)
	xml := fmt.Sprintf(
		`<wp:inline distT="0" distB="0" distL="0" distR="0" `+
			`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" `+
			`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" `+
			`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" `+
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:docPr id="%d" name="Picture %d" descr="%s"/>`+
			`<a:graphic>`+
			`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
			`<pic:pic>`+
			`<pic:nvPicPr>`+
			`<pic:cNvPr id="%d" name="Picture %d"/>`+
			`<pic:cNvPicPr/>`+
			`</pic:nvPicPr>`+
			`<pic:blipFill>`+
			`<a:blip r:embed="%s"/>`+
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
			`</a:graphic>`+
			`</wp:inline>`,
		img.widthEMU, img.heightEMU,
		img.drawingID, img.drawingID, altText,
		img.drawingID, img.drawingID,
		img.relID,
		img.widthEMU, img.heightEMU,
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

// AddImage adds an inline image from a file path to the run.
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

	return r.addImageData(data, ct, ext)
}

// AddImageFromBytes adds an inline image from raw bytes to the run.
func (r *Run) AddImageFromBytes(data []byte, contentType string) (*InlineImage, error) {
	ext := extForContentType(contentType)
	if ext == "" {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
	return r.addImageData(data, contentType, ext)
}

func (r *Run) addImageData(data []byte, contentType, ext string) (*InlineImage, error) {
	doc := r.paragraph.document
	if doc == nil {
		return nil, fmt.Errorf("run is not attached to a document")
	}

	// The relationship is registered against the part that contains the
	// drawing: document.xml for body paragraphs, the header/footer part for
	// paragraphs in a header or footer — an r:embed only resolves through the
	// rels of the part it appears in.
	owner := mainDocumentPart
	if r.paragraph.hfPart != "" {
		owner = r.paragraph.hfPart
	}

	relID := fmt.Sprintf("rId%d", doc.nextRelID())

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
		// Assign image number derived from the parts already in the package
		// (preserved parts plus images added in this session), so adding an
		// image to an opened document that already contains
		// /word/media/image1.png does not produce a duplicate part name.
		imgNum := doc.nextImageNumber()
		partName = fmt.Sprintf("/word/media/image%d%s", imgNum, ext)
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

	// Default size: 100x100 points (will be overridden by SetSize)
	img := &InlineImage{
		relID:     relID,
		widthEMU:  int64(100 * 914400 / 72),
		heightEMU: int64(100 * 914400 / 72),
		drawingID: uint32(imageNumberFromPartName(partName)),
		run:       r,
	}

	// Add drawing element to run
	drawing := &oxml.CT_Drawing{
		RawContent: img.buildInlineXML(),
	}
	r.r.AppendDrawing(drawing)

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
	default:
		return ""
	}
}
