package xlsx

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	// Register decoders so AddImage can read intrinsic pixel dimensions.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/mgilbir/spine/opc"
)

// emuPerPixel is the EMU-per-pixel conversion at 96 DPI. DrawingML measures in
// English Metric Units (914400 per inch); at 96 px/inch that is 9525 EMU/px.
const emuPerPixel = 9525

// sheetImage is one image anchored to a worksheet. The anchor is a one-cell
// anchor: the top-left corner is pinned to (fromRow, fromCol) and the image
// keeps a fixed size (widthEMU x heightEMU), matching openpyxl's default
// add_image behaviour.
type sheetImage struct {
	data        []byte
	ext         string // file extension without dot, e.g. "png"
	contentType string
	fromCol     int // 0-based column of the top-left anchor cell
	fromRow     int // 0-based row of the top-left anchor cell
	widthEMU    int64
	heightEMU   int64
}

// Excel worksheet bounds and a sanity cap on image dimensions. The pixel cap
// keeps the EMU conversion (px * 9525) well within ST_PositiveCoordinate's
// range and rejects absurd/overflowing sizes.
const (
	maxExcelColumns = 16384   // XFD
	maxExcelRows    = 1048576 // 2^20
	maxImagePixels  = 1 << 20 // 1,048,576 px -> ~9.98e9 EMU, well under the ~2.7e13 max
)

// ImageOptions configures how an image is placed on a sheet.
type ImageOptions struct {
	// WidthPx and HeightPx set the rendered image size in pixels. When either
	// is zero, the image's intrinsic pixel dimension is used for that axis.
	// Note that a single provided dimension is NOT scaled to preserve aspect
	// ratio: the other axis still uses the intrinsic pixel size, so set both
	// when you want a specific aspect ratio.
	WidthPx  int
	HeightPx int
}

// AddImage anchors an image (PNG, JPEG or GIF) with its top-left corner at the
// given cell reference (e.g. "A1"). The image bytes are embedded in the
// workbook on save.
//
// AddImage is currently supported only on workbooks created with Create().
// Embedding into a workbook opened from disk (Open/OpenReader) is not yet
// implemented, so to avoid silently dropping the image on save AddImage returns
// an error in that case rather than accepting an image that would be lost.
func (s *Sheet) AddImage(cellRef string, data []byte, opts ImageOptions) error {
	if s.workbook != nil && s.workbook.reader != nil {
		return fmt.Errorf("xlsx: AddImage: embedding images into an opened workbook is not supported yet (only workbooks created with Create())")
	}
	if len(data) == 0 {
		return fmt.Errorf("xlsx: AddImage: empty image data")
	}
	row, col, err := ParseCellRef(cellRef)
	if err != nil {
		return fmt.Errorf("xlsx: AddImage: %w", err)
	}
	if col > maxExcelColumns || row > maxExcelRows {
		return fmt.Errorf("xlsx: AddImage: cell %q is out of range", cellRef)
	}

	ext, contentType, err := detectImageType(data)
	if err != nil {
		return fmt.Errorf("xlsx: AddImage: %w", err)
	}

	// Always decode the header: it validates that the bytes are a real,
	// decodable image (not just a valid magic prefix) and supplies the
	// intrinsic size for any dimension the caller left unset.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("xlsx: AddImage: decode image: %w", err)
	}
	widthPx, heightPx := opts.WidthPx, opts.HeightPx
	if widthPx <= 0 {
		widthPx = cfg.Width
	}
	if heightPx <= 0 {
		heightPx = cfg.Height
	}
	if widthPx <= 0 || heightPx <= 0 {
		return fmt.Errorf("xlsx: AddImage: image has non-positive dimensions (%dx%d)", widthPx, heightPx)
	}
	if widthPx > maxImagePixels || heightPx > maxImagePixels {
		return fmt.Errorf("xlsx: AddImage: image dimensions %dx%d exceed the maximum of %d px", widthPx, heightPx, maxImagePixels)
	}

	// Copy the bytes: the image is embedded at save time, which may be long
	// after this call, so we must not alias a slice the caller might reuse.
	s.images = append(s.images, sheetImage{
		data:        bytes.Clone(data),
		ext:         ext,
		contentType: contentType,
		fromCol:     col - 1,
		fromRow:     row - 1,
		widthEMU:    int64(widthPx) * emuPerPixel,
		heightEMU:   int64(heightPx) * emuPerPixel,
	})
	s.dirty = true
	return nil
}

// detectImageType sniffs the image format from its magic bytes and returns the
// file extension and OPC content type.
func detectImageType(data []byte) (ext, contentType string, err error) {
	switch {
	case len(data) >= 8 && bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png", opc.ContentTypePNG, nil
	case len(data) >= 3 && bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "jpeg", opc.ContentTypeJPEG, nil
	case len(data) >= 6 && (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))):
		return "gif", opc.ContentTypeGIF, nil
	default:
		return "", "", fmt.Errorf("unsupported image format (want PNG, JPEG or GIF)")
	}
}

// marshalDrawingXML renders the xl/drawings/drawingN.xml part for a sheet's
// images. Each image becomes a one-cell anchor referencing a media part via the
// relationship id assigned in relIDForImage (rId1, rId2, ...).
func marshalDrawingXML(images []sheetImage) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`)
	for i, img := range images {
		relID := relIDForImage(i)
		shapeID := i + 1
		fmt.Fprintf(&b, `<xdr:oneCellAnchor>`+
			`<xdr:from><xdr:col>%d</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>`+
			`<xdr:ext cx="%d" cy="%d"/>`+
			`<xdr:pic>`+
			`<xdr:nvPicPr><xdr:cNvPr id="%d" name="Image %d"/><xdr:cNvPicPr><a:picLocks noChangeAspect="1"/></xdr:cNvPicPr></xdr:nvPicPr>`+
			`<xdr:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></xdr:blipFill>`+
			`<xdr:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr>`+
			`</xdr:pic>`+
			`<xdr:clientData/>`+
			`</xdr:oneCellAnchor>`,
			img.fromCol, img.fromRow, img.widthEMU, img.heightEMU, shapeID, shapeID, relID, img.widthEMU, img.heightEMU)
	}
	b.WriteString(`</xdr:wsDr>`)
	return []byte(b.String())
}

// relIDForImage returns the drawing-local relationship id for the i-th image.
func relIDForImage(i int) string {
	return fmt.Sprintf("rId%d", i+1)
}
