package xlsx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"strconv"
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

// svgBlipExtURI is the Office drawing extension GUID carrying the SVG variant
// of an image alongside its raster fallback (same as the docx/pptx paths).
const svgBlipExtURI = "{96DAC541-7B7A-43D3-8B79-37D633B846F1}"

// nsDrawingSVG2016 is the namespace of the asvg:svgBlip element.
const nsDrawingSVG2016 = "http://schemas.microsoft.com/office/drawing/2016/SVG/main"

// sheetImage is one image anchored to a worksheet. By default it is a one-cell
// anchor (top-left pinned to fromRow/fromCol, fixed size). When toCol/toRow are
// set (twoCell), the image spans from the anchor cell to the target cell and
// resizes with the cells.
type sheetImage struct {
	data        []byte
	ext         string // raster file extension without dot, e.g. "png"
	contentType string
	fromCol     int // 0-based column of the top-left anchor cell
	fromRow     int // 0-based row of the top-left anchor cell
	widthEMU    int64
	heightEMU   int64

	// Two-cell anchor: when twoCell is set the image spans to (toRow, toCol).
	twoCell bool
	toCol   int
	toRow   int

	// SVG variant: when svgData is set, data/ext/contentType hold the raster
	// fallback and svgData holds the SVG shown by SVG-aware Excel.
	svgData []byte
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
	// is zero, the image's intrinsic pixel dimension is used for that axis,
	// unless PreserveAspect is set (see below).
	WidthPx  int
	HeightPx int

	// PreserveAspect, when set with exactly one of WidthPx/HeightPx, scales the
	// unset axis to keep the image's intrinsic aspect ratio (instead of using
	// its intrinsic pixel size for that axis).
	PreserveAspect bool

	// ToCell, when set, makes the image a two-cell anchor spanning from the
	// anchor cell to ToCell (e.g. "D10"): the image moves and resizes with the
	// cells. WidthPx/HeightPx are ignored for a two-cell anchor.
	ToCell string
}

// AddImage anchors an image (PNG, JPEG, GIF or SVG) with its top-left corner at
// the given cell reference (e.g. "A1"). The image bytes are embedded in the
// workbook on save. SVG images are embedded with a transparent raster fallback
// for viewers that cannot render SVG.
//
// AddImage works on both created (Create) and opened (Open/OpenReader)
// workbooks. On an opened workbook the drawing, media, and relationship parts
// are added alongside whatever the package already carries, with part names
// chosen to avoid the existing ones.
//
// One caveat for opened workbooks: if the target sheet already has a drawing
// from the original file, the images added here are written to a new drawing
// that replaces it as the sheet's referenced drawing (the original drawing's
// shapes are no longer shown). Adding images to sheets without an existing
// drawing — the common templating case — is unaffected.
func (s *Sheet) AddImage(cellRef string, data []byte, opts ImageOptions) error {
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

	img := sheetImage{
		fromCol: col - 1,
		fromRow: row - 1,
	}

	// SVG is detected by content sniff; anything else goes through the raster
	// decoder for validation and intrinsic size.
	if isSVG(data) {
		img.svgData = bytes.Clone(data)
		img.data = bytes.Clone(transparentPNG)
		img.ext = "png"
		img.contentType = opc.ContentTypePNG
	} else {
		ext, contentType, derr := detectImageType(data)
		if derr != nil {
			return fmt.Errorf("xlsx: AddImage: %w", derr)
		}
		img.data = bytes.Clone(data)
		img.ext = ext
		img.contentType = contentType
	}

	// Intrinsic dimensions: decode the raster (for SVG, parse the SVG header).
	iw, ih, err := intrinsicSize(data)
	if err != nil {
		return fmt.Errorf("xlsx: AddImage: %w", err)
	}

	if opts.ToCell != "" {
		toRow, toCol, terr := ParseCellRef(opts.ToCell)
		if terr != nil {
			return fmt.Errorf("xlsx: AddImage: ToCell %q: %w", opts.ToCell, terr)
		}
		if toCol > maxExcelColumns || toRow > maxExcelRows {
			return fmt.Errorf("xlsx: AddImage: ToCell %q is out of range", opts.ToCell)
		}
		if toCol < col || toRow < row {
			return fmt.Errorf("xlsx: AddImage: ToCell %q must be at or below and right of %q", opts.ToCell, cellRef)
		}
		img.twoCell = true
		img.toCol = toCol - 1
		img.toRow = toRow - 1
		s.images = append(s.images, img)
		s.dirty = true
		return nil
	}

	widthPx, heightPx, err := resolveSize(opts, iw, ih)
	if err != nil {
		return fmt.Errorf("xlsx: AddImage: %w", err)
	}
	img.widthEMU = int64(widthPx) * emuPerPixel
	img.heightEMU = int64(heightPx) * emuPerPixel

	s.images = append(s.images, img)
	s.dirty = true
	return nil
}

// resolveSize computes the final pixel size from the options and intrinsic
// dimensions, applying aspect-preserving scaling when requested.
func resolveSize(opts ImageOptions, intrinsicW, intrinsicH int) (w, h int, err error) {
	w, h = opts.WidthPx, opts.HeightPx
	switch {
	case opts.PreserveAspect && w > 0 && h <= 0 && intrinsicW > 0:
		h = int(int64(w) * int64(intrinsicH) / int64(intrinsicW))
	case opts.PreserveAspect && h > 0 && w <= 0 && intrinsicH > 0:
		w = int(int64(h) * int64(intrinsicW) / int64(intrinsicH))
	default:
		if w <= 0 {
			w = intrinsicW
		}
		if h <= 0 {
			h = intrinsicH
		}
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("image has non-positive dimensions (%dx%d)", w, h)
	}
	if w > maxImagePixels || h > maxImagePixels {
		return 0, 0, fmt.Errorf("image dimensions %dx%d exceed the maximum of %d px", w, h, maxImagePixels)
	}
	return w, h, nil
}

// intrinsicSize returns the intrinsic pixel dimensions of an image. Raster
// formats are decoded; SVG dimensions come from the root width/height or
// viewBox, defaulting to 300x150 (the CSS default) when neither is present.
func intrinsicSize(data []byte) (w, h int, err error) {
	if isSVG(data) {
		w, h = svgIntrinsicSize(data)
		return w, h, nil
	}
	cfg, _, derr := image.DecodeConfig(bytes.NewReader(data))
	if derr != nil {
		return 0, 0, fmt.Errorf("decode image: %w", derr)
	}
	return cfg.Width, cfg.Height, nil
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
		return "", "", fmt.Errorf("unsupported image format (want PNG, JPEG, GIF or SVG)")
	}
}

// imageRels holds the relationship ids a drawing uses to reference one image's
// media parts: the raster blip and, for SVG images, the svgBlip extension.
type imageRels struct {
	rasterRID string
	svgRID    string // empty for non-SVG images
}

// marshalDrawingXML renders the xl/drawings/drawingN.xml part for a sheet's
// images and charts, using the supplied per-image relationship ids and per-chart
// relationship ids. Images and charts share one drawing part so they coexist on
// a sheet; shape ids are unique across both.
func marshalDrawingXML(images []sheetImage, rels []imageRels, charts []sheetChart, chartRIDs []string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`)
	b.WriteString(drawingAnchorsXML(images, rels, charts, chartRIDs, 0))
	b.WriteString(`</xdr:wsDr>`)
	return []byte(b.String())
}

// drawingAnchorsXML renders just the anchor elements (no wsDr wrapper) for a
// sheet's images and charts. shapeIDBase offsets the generated cNvPr shape ids
// so they stay unique when the anchors are spliced into a drawing that already
// carries shapes; pass 0 for a fresh drawing.
func drawingAnchorsXML(images []sheetImage, rels []imageRels, charts []sheetChart, chartRIDs []string, shapeIDBase int) string {
	var b strings.Builder
	for i, img := range images {
		shapeID := shapeIDBase + i + 1
		pic := picXML(shapeID, rels[i], img)
		if img.twoCell {
			fmt.Fprintf(&b, `<xdr:twoCellAnchor editAs="oneCell">`+
				`<xdr:from><xdr:col>%d</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>`+
				`<xdr:to><xdr:col>%d</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>`+
				`%s`+
				`<xdr:clientData/>`+
				`</xdr:twoCellAnchor>`,
				img.fromCol, img.fromRow, img.toCol, img.toRow, pic)
			continue
		}
		fmt.Fprintf(&b, `<xdr:oneCellAnchor>`+
			`<xdr:from><xdr:col>%d</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>`+
			`<xdr:ext cx="%d" cy="%d"/>`+
			`%s`+
			`<xdr:clientData/>`+
			`</xdr:oneCellAnchor>`,
			img.fromCol, img.fromRow, img.widthEMU, img.heightEMU, pic)
	}
	for i, ch := range charts {
		shapeID := shapeIDBase + len(images) + i + 1
		b.WriteString(graphicFrameXML(shapeID, chartRIDs[i], ch))
	}
	return b.String()
}

// spliceDrawingAnchors inserts anchor XML immediately before the closing
// </xdr:wsDr> (or namespace-agnostic </wsDr>) of an existing drawing part,
// preserving the original anchors. If no close tag is found the anchors are
// appended (the drawing is then likely malformed, but nothing is dropped).
func spliceDrawingAnchors(drawing []byte, anchors string) []byte {
	if anchors == "" {
		return drawing
	}
	idx := bytes.LastIndex(drawing, []byte("</xdr:wsDr>"))
	if idx < 0 {
		if i := bytes.LastIndex(drawing, []byte(":wsDr>")); i >= 0 {
			if j := bytes.LastIndex(drawing[:i], []byte("</")); j >= 0 {
				idx = j
			}
		}
	}
	if idx < 0 {
		if i := bytes.LastIndex(drawing, []byte("</wsDr>")); i >= 0 {
			idx = i
		}
	}
	if idx < 0 {
		return append(append([]byte{}, drawing...), anchors...)
	}
	out := make([]byte, 0, len(drawing)+len(anchors))
	out = append(out, drawing[:idx]...)
	out = append(out, anchors...)
	out = append(out, drawing[idx:]...)
	return out
}

// maxDrawingShapeID returns the largest cNvPr id in a spreadsheet-drawing part,
// or 0 when none is present or the part does not parse. Session anchors take ids
// above it so shape ids stay unique within a merged drawing.
func maxDrawingShapeID(drawing []byte) int {
	dec := xml.NewDecoder(bytes.NewReader(drawing))
	max := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "cNvPr" {
			continue
		}
		for _, a := range se.Attr {
			if a.Name.Local != "id" {
				continue
			}
			if n, err := strconv.Atoi(a.Value); err == nil && n > max {
				max = n
			}
		}
	}
	return max
}

// graphicFrameXML renders the xdr:twoCellAnchor holding a graphicFrame that
// references a chart part by relationship id (a:graphicData uri=chart >
// c:chart r:id). Excel derives the frame size from the from/to cells, so the
// xfrm extent is a placeholder.
func graphicFrameXML(shapeID int, chartRID string, ch sheetChart) string {
	return fmt.Sprintf(`<xdr:twoCellAnchor>`+
		`<xdr:from><xdr:col>%d</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>`+
		`<xdr:to><xdr:col>%d</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>`+
		`<xdr:graphicFrame macro="">`+
		`<xdr:nvGraphicFramePr><xdr:cNvPr id="%d" name="Chart %d"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr>`+
		`<xdr:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></xdr:xfrm>`+
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">`+
		`<c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="%s"/>`+
		`</a:graphicData></a:graphic>`+
		`</xdr:graphicFrame>`+
		`<xdr:clientData/>`+
		`</xdr:twoCellAnchor>`,
		ch.fromCol, ch.fromRow, ch.toCol, ch.toRow, shapeID, shapeID, chartRID)
}

// picXML renders the xdr:pic element (blip fill + shape props). For a two-cell
// anchor the shape size is a placeholder (0x0); Excel derives it from the
// from/to cells.
func picXML(shapeID int, rel imageRels, img sheetImage) string {
	blip := fmt.Sprintf(`<a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="%s"`, rel.rasterRID)
	if rel.svgRID != "" {
		blip += fmt.Sprintf(`>`+
			`<a:extLst><a:ext uri="%s">`+
			`<asvg:svgBlip xmlns:asvg="%s" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="%s"/>`+
			`</a:ext></a:extLst>`+
			`</a:blip>`, svgBlipExtURI, nsDrawingSVG2016, rel.svgRID)
	} else {
		blip += `/>`
	}
	return fmt.Sprintf(`<xdr:pic>`+
		`<xdr:nvPicPr><xdr:cNvPr id="%d" name="Image %d"/><xdr:cNvPicPr><a:picLocks noChangeAspect="1"/></xdr:cNvPicPr></xdr:nvPicPr>`+
		`<xdr:blipFill>%s<a:stretch><a:fillRect/></a:stretch></xdr:blipFill>`+
		`<xdr:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr>`+
		`</xdr:pic>`,
		shapeID, shapeID, blip, img.widthEMU, img.heightEMU)
}

// transparentPNG is a 1x1 transparent PNG used as the raster fallback for SVG
// images.
var transparentPNG = []byte{
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
