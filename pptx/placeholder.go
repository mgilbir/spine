package pptx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgilbir/spine/common/dml"
)

// Common placeholder errors.
var (
	ErrNotPicturePlaceholder = errors.New("pptx: not a picture placeholder")
)

// PlaceholderType represents the type of a placeholder.
type PlaceholderType string

const (
	PlaceholderTitle          PlaceholderType = "title"
	PlaceholderBody           PlaceholderType = "body"
	PlaceholderCenteredTitle  PlaceholderType = "ctrTitle"
	PlaceholderSubtitle       PlaceholderType = "subTitle"
	PlaceholderDateTime       PlaceholderType = "dt"
	PlaceholderSlideNumber    PlaceholderType = "sldNum"
	PlaceholderFooter         PlaceholderType = "ftr"
	PlaceholderHeader         PlaceholderType = "hdr"
	PlaceholderObject         PlaceholderType = "obj"
	PlaceholderChart          PlaceholderType = "chart"
	PlaceholderTable          PlaceholderType = "tbl"
	PlaceholderClipArt        PlaceholderType = "clipArt"
	PlaceholderDiagram        PlaceholderType = "dgm"
	PlaceholderMedia          PlaceholderType = "media"
	PlaceholderSlideImage     PlaceholderType = "sldImg"
	PlaceholderPicture        PlaceholderType = "pic"
)

// PlaceholderOrientation specifies the orientation of a placeholder.
type PlaceholderOrientation string

const (
	PlaceholderOrientationHorizontal PlaceholderOrientation = "horz"
	PlaceholderOrientationVertical   PlaceholderOrientation = "vert"
)

// PlaceholderSize specifies the size hint for a placeholder.
type PlaceholderSize string

const (
	PlaceholderSizeFull    PlaceholderSize = "full"
	PlaceholderSizeHalf    PlaceholderSize = "half"
	PlaceholderSizeQuarter PlaceholderSize = "quarter"
)

// PlaceholderShape represents a placeholder shape on a slide.
type PlaceholderShape struct {
	BaseShape
	phType      PlaceholderType
	orientation PlaceholderOrientation
	size        PlaceholderSize
	idx         uint32
	textFrame   *TextFrame

	// Image replacement fields (only for picture placeholders)
	pendingImagePath string // file path to image (set via SetImage)
	pendingImageData []byte // raw image data (set via SetImageData)
	pendingImageCT   string // content type of pending image
	pendingSVGData   []byte // raw svg data (set via SetSVGData)
	pendingSVGCT     string // content type of pending svg
	slide            *Slide // back-reference to the owning slide (set during materialization)

	// Furniture placeholder fields (footer / slide number / date). When
	// inheritGeometry is set the shape is written with an empty <p:spPr/> so
	// PowerPoint inherits position and size from the layout placeholder of the
	// same type — the way real slides carry footer/number/date. fieldType, when
	// set, makes the text body an auto-updating <a:fld> (e.g. "slidenum",
	// "datetime") with fieldText as the fallback shown until PowerPoint
	// recalculates it.
	inheritGeometry bool
	fieldType       string
	fieldText       string
}

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

// NewPlaceholderShape creates a new placeholder shape.
func NewPlaceholderShape(phType PlaceholderType) *PlaceholderShape {
	return &PlaceholderShape{
		phType:    phType,
		textFrame: NewTextFrame(),
	}
}

// ShapeType returns ShapeTypePlaceholder.
func (p *PlaceholderShape) ShapeType() ShapeType {
	return ShapeTypePlaceholder
}

// PlaceholderType returns the type of the placeholder.
func (p *PlaceholderShape) PlaceholderType() PlaceholderType {
	return p.phType
}

// Orientation returns the orientation of the placeholder.
func (p *PlaceholderShape) Orientation() PlaceholderOrientation {
	return p.orientation
}

// SetOrientation sets the orientation of the placeholder.
func (p *PlaceholderShape) SetOrientation(orient PlaceholderOrientation) {
	p.orientation = orient
	p.dirty = true
}

// Size returns the size hint of the placeholder.
func (p *PlaceholderShape) PlaceholderSize() PlaceholderSize {
	return p.size
}

// SetPlaceholderSize sets the size hint of the placeholder.
func (p *PlaceholderShape) SetPlaceholderSize(size PlaceholderSize) {
	p.size = size
	p.dirty = true
}

// Index returns the placeholder index.
func (p *PlaceholderShape) Index() uint32 {
	return p.idx
}

// SetIndex sets the placeholder index.
func (p *PlaceholderShape) SetIndex(idx uint32) {
	p.idx = idx
	p.dirty = true
}

// TextFrame returns the text frame for the placeholder.
func (p *PlaceholderShape) TextFrame() *TextFrame {
	return p.textFrame
}

// SetText sets the text content of the placeholder.
func (p *PlaceholderShape) SetText(text string) {
	if p.textFrame == nil {
		p.textFrame = NewTextFrame()
	}
	p.textFrame.SetText(text)
}

// Text returns the text content of the placeholder.
func (p *PlaceholderShape) Text() string {
	if p.textFrame == nil {
		return ""
	}
	return p.textFrame.Text()
}

// SetHyperlink attaches an external-URL hyperlink to the placeholder. The
// External relationship is allocated on save.
func (p *PlaceholderShape) SetHyperlink(url string) *Hyperlink { return p.setHyperlinkURL(url) }

// SetActionHyperlink attaches a slide-show action hyperlink (e.g. ActionNextSlide)
// to the placeholder.
func (p *PlaceholderShape) SetActionHyperlink(action string) *Hyperlink {
	return p.setActionHyperlink(action)
}

// SetHyperlinkToSlide attaches an internal jump to the slide at the given 0-based
// index; the RelTypeSlide relationship is allocated on save.
func (p *PlaceholderShape) SetHyperlinkToSlide(index int) *Hyperlink { return p.setSlideLink(index) }

// IsTitle returns true if this is a title placeholder.
func (p *PlaceholderShape) IsTitle() bool {
	return p.phType == PlaceholderTitle || p.phType == PlaceholderCenteredTitle
}

// IsBody returns true if this is a body/content placeholder.
func (p *PlaceholderShape) IsBody() bool {
	return p.phType == PlaceholderBody || p.phType == PlaceholderObject
}

// IsPicture returns true if this is a picture placeholder.
func (p *PlaceholderShape) IsPicture() bool {
	return p.phType == PlaceholderPicture
}

// SetImage sets an image on a picture placeholder from a file path.
// Only works on placeholders of type PlaceholderPicture.
// The image is embedded when the presentation is saved.
func (p *PlaceholderShape) SetImage(imagePath string) error {
	if p.phType != PlaceholderPicture {
		return ErrNotPicturePlaceholder
	}

	data, ct, err := readImageFile(imagePath)
	if err != nil {
		return err
	}

	p.pendingImagePath = imagePath
	p.pendingImageData = data
	p.pendingImageCT = ct
	p.pendingSVGData = nil
	p.pendingSVGCT = ""

	return nil
}

// SetImageData sets an image on a picture placeholder from raw bytes.
// Only works on placeholders of type PlaceholderPicture.
// The contentType should be a MIME type like "image/png" or "image/jpeg".
func (p *PlaceholderShape) SetImageData(data []byte, contentType string) error {
	if p.phType != PlaceholderPicture {
		return ErrNotPicturePlaceholder
	}

	p.pendingImageData = data
	p.pendingImageCT = contentType
	p.pendingImagePath = ""
	p.pendingSVGData = nil
	p.pendingSVGCT = ""

	return nil
}

// SetSVGImageData sets SVG data plus a raster fallback image on a picture placeholder.
func (p *PlaceholderShape) SetSVGImageData(svgData, fallbackData []byte, fallbackCT string) error {
	if p.phType != PlaceholderPicture {
		return ErrNotPicturePlaceholder
	}

	p.pendingSVGData = svgData
	p.pendingSVGCT = "image/svg+xml"
	p.pendingImageData = fallbackData
	p.pendingImageCT = fallbackCT
	p.pendingImagePath = ""

	return nil
}

// SetSVGData sets SVG data on a picture placeholder using a transparent PNG fallback.
func (p *PlaceholderShape) SetSVGData(svgData []byte) error {
	return p.SetSVGImageData(svgData, minimalTransparentPNG, "image/png")
}

// hasPendingImage returns true if this placeholder has a pending image replacement.
func (p *PlaceholderShape) hasPendingImage() bool {
	return len(p.pendingImageData) > 0 || len(p.pendingSVGData) > 0
}

// contentTypeFromExt returns the MIME type for a file extension.
func contentTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".svg":
		return "image/svg+xml"
	case ".emf":
		return "image/x-emf"
	case ".wmf":
		return "image/x-wmf"
	default:
		return "image/png"
	}
}

// extFromContentType returns a file extension for a MIME type.
func extFromContentType(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpeg"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	case "image/svg+xml":
		return ".svg"
	case "image/x-emf":
		return ".emf"
	case "image/x-wmf":
		return ".wmf"
	default:
		return ".png"
	}
}

// readImageFile reads an image file and returns its data and content type.
func readImageFile(imagePath string) (data []byte, contentType string, err error) {
	data, err = os.ReadFile(imagePath)
	if err != nil {
		return nil, "", fmt.Errorf("pptx: reading image file: %w", err)
	}
	contentType = contentTypeFromExt(filepath.Ext(imagePath))
	return data, contentType, nil
}

// DefaultTitlePlaceholder creates a title placeholder with default settings.
func DefaultTitlePlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(0.3))
	ph.SetSize(dml.Inches(9), dml.Inches(1.2))
	return ph
}

// DefaultBodyPlaceholder creates a body placeholder with default settings.
func DefaultBodyPlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderBody)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(1.6))
	ph.SetSize(dml.Inches(9), dml.Inches(5.1))
	return ph
}

// DefaultCenteredTitlePlaceholder creates a centered title placeholder.
func DefaultCenteredTitlePlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderCenteredTitle)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(2.5))
	ph.SetSize(dml.Inches(9), dml.Inches(1.5))
	return ph
}

// DefaultSubtitlePlaceholder creates a subtitle placeholder.
func DefaultSubtitlePlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderSubtitle)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(4.0))
	ph.SetSize(dml.Inches(9), dml.Inches(1.0))
	return ph
}
