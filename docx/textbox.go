package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// ShapeType names a preset shape geometry (an OOXML a:prstGeom prst value).
// The four values below cover the common cases used by text boxes and basic
// shapes; the geometry is emitted verbatim as the prst attribute, so any other
// preset name can be passed through as well.
type ShapeType string

const (
	// ShapeRectangle is a plain rectangle (the default text box geometry).
	ShapeRectangle ShapeType = "rect"
	// ShapeRoundRectangle is a rounded rectangle.
	ShapeRoundRectangle ShapeType = "roundRect"
	// ShapeEllipse is an ellipse/oval.
	ShapeEllipse ShapeType = "ellipse"
	// ShapeLine is a straight line connector geometry.
	ShapeLine ShapeType = "line"
)

// Default text box geometry: 2 inch wide, 1 inch tall (EMU units, 914400/inch).
const (
	defaultTextBoxWidthEMU  = 2 * 914400
	defaultTextBoxHeightEMU = 914400
)

// shapeIDBase offsets generated docPr ids well above the small ids that images
// and charts hand out (image/chart numbers, typically < 1000), so a text box
// added alongside them does not collide on the drawing id.
const shapeIDBase = 100000

// defaultBorderWidthEMU is a 0.5pt border (12700 EMU per point).
const defaultBorderWidthEMU = 6350

// wpsShapeNamespaces are the namespace declarations placed on the wp:inline /
// wp:anchor element of a DrawingML shape drawing: the wordprocessingDrawing
// wrapper, the DrawingML main namespace, the wordprocessingShape (wps)
// namespace that carries the shape, and the WordprocessingML main namespace so
// the w:txbxContent body resolves regardless of the root's prefix binding.
const wpsShapeNamespaces = `xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" ` +
	`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

// wpsGraphicDataURI marks a drawing's a:graphicData as a wordprocessingShape.
const wpsGraphicDataURI = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"

// TextBoxOptions configures a text box or shape created with AddTextBox /
// AddShape. The zero value produces an inline rectangular text box with a white
// fill and a thin black border.
type TextBoxOptions struct {
	// WidthEMU and HeightEMU are the box size in EMU (914400 per inch). When
	// zero a default of 2in x 1in is used.
	WidthEMU  int64
	HeightEMU int64
	// Floating anchors the box (positioned relative to the page or paragraph)
	// instead of placing it inline in the text flow.
	Floating bool
	// Anchor positions the box when Floating is set (same semantics as images).
	Anchor Anchor
	// Shape selects the preset geometry; empty means ShapeRectangle.
	Shape ShapeType
	// FillColor is the fill color as a hex "RRGGBB" string; empty uses the
	// default (white). Set NoFill for a transparent shape.
	FillColor string
	// NoFill makes the shape transparent (no fill), overriding FillColor.
	NoFill bool
	// BorderColor is the outline color as a hex "RRGGBB" string; empty uses the
	// default (black). Set NoBorder for no outline.
	BorderColor string
	// BorderWidthEMU is the outline width in EMU; zero uses a 0.5pt default.
	BorderWidthEMU int64
	// NoBorder removes the outline, overriding BorderColor/BorderWidthEMU.
	NoBorder bool
	// NoVMLFallback writes the DrawingML shape alone, without the
	// mc:AlternateContent wrapper and legacy VML w:pict fallback that Word
	// emits beside it.
	//
	// The wrapper is the default because without it the shape is not markup a
	// conforming consumer can process: it sits in a:graphicData, whose wildcard
	// is processContents="strict", and wps: is a Microsoft extension no ISO
	// schema declares — so a reader that does not know the extension may
	// neither render it nor skip it, and shows nothing at all. That is what the
	// schema-conformance suite reported ("Element 'wsp': No matching global
	// element declaration available, but demanded by the strict wildcard").
	//
	// Set it only when the output is for a consumer known to understand wps and
	// the smaller part is worth more than down-level rendering.
	NoVMLFallback bool
}

// TextBox is a handle to a DrawingML (wps) or legacy VML text box or shape. It
// is returned by AddTextBox/AddShape and by Document.TextBoxes(), and exposes
// the box's text and geometry.
type TextBox struct {
	text      string
	widthEMU  int64
	heightEMU int64
	floating  bool
	shape     ShapeType
	vml       bool
	// drawing is the w:drawing element this handle authored (nil for a text box
	// read from a legacy VML w:pict or an mc:AlternateContent wrapper).
	drawing *oxml.CT_Drawing
}

// Text returns the plain text of the box, joining its paragraphs with newlines.
func (tb *TextBox) Text() string { return tb.text }

// WidthEMU returns the box width in EMU (914400 per inch).
func (tb *TextBox) WidthEMU() int64 { return tb.widthEMU }

// HeightEMU returns the box height in EMU.
func (tb *TextBox) HeightEMU() int64 { return tb.heightEMU }

// Width returns the box width in points.
func (tb *TextBox) Width() float64 { return emuToPoints(tb.widthEMU) }

// Height returns the box height in points.
func (tb *TextBox) Height() float64 { return emuToPoints(tb.heightEMU) }

// Floating reports whether the box is anchored/floating rather than inline.
func (tb *TextBox) Floating() bool { return tb.floating }

// Shape returns the box's preset geometry (e.g. ShapeRectangle), or "" if it
// was read from a legacy VML text box that carries no DrawingML preset.
func (tb *TextBox) Shape() ShapeType { return tb.shape }

// IsVML reports whether the box was read from a legacy VML w:pict drawing
// rather than a modern DrawingML (wps) drawing.
func (tb *TextBox) IsVML() bool { return tb.vml }

// --- create ---

// nextShapeID returns the next docPr id for a text box or shape. Ids start
// above shapeIDBase so they do not collide with the small drawing ids that
// images and charts derive from their part numbers.
//
// On first use the counter is seeded past the highest docPr id at or above
// shapeIDBase already present in the opened document's drawings. Without that
// seed every session restarted at shapeIDBase+1, so a text box added to a
// document that was itself produced by this library got the id a box in it
// already carries — save, reopen, AddTextBox, save yielded two
// <wp:docPr id="100001"> in one document (C409). AddShape, AddShapeGroup,
// AddWordArt, AddSignatureLine and AddOLEObject all share this counter and were
// all affected.
func (d *Document) nextShapeID() int {
	if !d.shapeIDInit {
		d.shapeIDInit = true
		if _, maxShape := d.maxExistingDocPrIDs(); maxShape >= shapeIDBase {
			d.shapeIDSeq = maxShape - shapeIDBase
		}
	}
	d.shapeIDSeq++
	return shapeIDBase + d.shapeIDSeq
}

// AddTextBox appends a paragraph containing a text box to the document body and
// returns the box handle. It is a convenience wrapper over Paragraph.AddTextBox.
func (d *Document) AddTextBox(text string, opts TextBoxOptions) *TextBox {
	return d.AddParagraph().AddTextBox(text, opts)
}

// AddTextBox inserts a DrawingML text box into a new run at the end of the
// paragraph. The box carries the given text (split into one paragraph per line)
// and honors the size, geometry, fill, and border in opts. Inline by default;
// set opts.Floating to anchor it. The box needs no extra parts or relationships,
// so it round-trips through save/open like an image drawing.
func (p *Paragraph) AddTextBox(text string, opts TextBoxOptions) *TextBox {
	return p.addShape(text, opts, true)
}

// AddShape inserts a basic DrawingML shape (rectangle, ellipse, rounded
// rectangle, or line — see opts.Shape) into a new run at the end of the
// paragraph, with optional text. It shares the text box drawing path; pass an
// empty text for a shape with no caption.
func (p *Paragraph) AddShape(text string, opts TextBoxOptions) *TextBox {
	return p.addShape(text, opts, false)
}

// AddShape appends a paragraph containing a basic shape to the document body.
func (d *Document) AddShape(text string, opts TextBoxOptions) *TextBox {
	return d.AddParagraph().AddShape(text, opts)
}

// addShape builds the shared wps drawing for a text box (isTextBox true, which
// sets the txBox marker and default white fill) or a basic shape.
func (p *Paragraph) addShape(text string, opts TextBoxOptions, isTextBox bool) *TextBox {
	width := opts.WidthEMU
	if width <= 0 {
		width = defaultTextBoxWidthEMU
	}
	height := opts.HeightEMU
	if height <= 0 {
		height = defaultTextBoxHeightEMU
	}
	shape := opts.Shape
	if shape == "" {
		shape = ShapeRectangle
	}

	tb := &TextBox{
		text:      text,
		widthEMU:  width,
		heightEMU: height,
		floating:  opts.Floating,
		shape:     shape,
	}
	id := p.document.nextShapeID()

	// The fallback applies to every shape, with or without text: an empty
	// v:textbox still carries a w:p, and a shape a down-level reader cannot
	// draw is exactly the one that needs the legacy form.
	if !opts.NoVMLFallback {
		ac := &oxml.CT_RawElement{RawContent: buildTextBoxAlternateContentXML(id, tb, opts, isTextBox)}
		tb.vml = false
		p.AddRun().mut().AppendAlternateContent(ac)
		return tb
	}

	drawing := &oxml.CT_Drawing{RawContent: buildShapeDrawingXML(id, tb, opts, isTextBox)}
	tb.drawing = drawing
	p.AddRun().mut().AppendDrawing(drawing)
	return tb
}

// buildShapeDrawingXML builds the w:drawing inner content (wp:inline or
// wp:anchor) for a wps shape: a:graphic → a:graphicData(uri=wps) → wps:wsp with
// geometry, fill, border, an optional w:txbxContent body, and a bodyPr.
func buildShapeDrawingXML(id int, tb *TextBox, opts TextBoxOptions, isTextBox bool) []byte {
	name := fmt.Sprintf("Text Box %d", id)
	if !isTextBox {
		name = fmt.Sprintf("Shape %d", id)
	}

	cNvSpPr := `<wps:cNvSpPr/>`
	if isTextBox {
		cNvSpPr = `<wps:cNvSpPr txBox="1"/>`
	}

	var txbx string
	if isTextBox || tb.text != "" {
		txbx = `<wps:txbx><w:txbxContent>` + txbxContentXML(tb.text) + `</w:txbxContent></wps:txbx>`
	}

	wsp := buildWspXML(id, name, cNvSpPr, txbx, 0, 0, tb.widthEMU, tb.heightEMU,
		tb.shape, fillXML(opts, isTextBox), lnXML(opts), "")

	graphic := fmt.Sprintf(
		`<a:graphic><a:graphicData uri="%s">%s</a:graphicData></a:graphic>`,
		wpsGraphicDataURI, wsp)

	if tb.floating {
		return buildShapeAnchorXML(id, name, tb, opts, graphic)
	}
	return []byte(fmt.Sprintf(
		`<wp:inline distT="0" distB="0" distL="0" distR="0" `+wpsShapeNamespaces+`>`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:docPr id="%d" name="%s"/>`+
			`<wp:cNvGraphicFramePr/>`+
			`%s`+
			`</wp:inline>`,
		tb.widthEMU, tb.heightEMU,
		id, xmlb.EscapeAttrValue(name),
		graphic,
	))
}

// buildShapeAnchorXML wraps the shape graphic in a floating wp:anchor, mirroring
// the anchored-image layout (wrapNone, z-order derived from the id).
func buildShapeAnchorXML(id int, name string, tb *TextBox, opts TextBoxOptions, graphic string) []byte {
	behindDoc := "0"
	if opts.Anchor.BehindText {
		behindDoc = "1"
	}
	hRel, vRel := "column", "paragraph"
	if opts.Anchor.RelativeToPage {
		hRel, vRel = "page", "page"
	}
	relHeight := 251658240 + int64(id)

	return []byte(fmt.Sprintf(
		`<wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" `+
			`relativeHeight="%d" behindDoc="%s" locked="0" layoutInCell="1" allowOverlap="1" `+
			wpsShapeNamespaces+`>`+
			`<wp:simplePos x="0" y="0"/>`+
			`<wp:positionH relativeFrom="%s"><wp:posOffset>%d</wp:posOffset></wp:positionH>`+
			`<wp:positionV relativeFrom="%s"><wp:posOffset>%d</wp:posOffset></wp:positionV>`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:wrapNone/>`+
			`<wp:docPr id="%d" name="%s"/>`+
			`<wp:cNvGraphicFramePr/>`+
			`%s`+
			`</wp:anchor>`,
		relHeight, behindDoc,
		hRel, pointsToEMU(opts.Anchor.X),
		vRel, pointsToEMU(opts.Anchor.Y),
		tb.widthEMU, tb.heightEMU,
		id, xmlb.EscapeAttrValue(name),
		graphic,
	))
}

// buildWspXML builds a single <wps:wsp> shape element: its non-visual
// properties, an a:xfrm placed at (xEMU,yEMU) with size cx x cy, the preset
// geometry, the fill and outline fragments, an optional w:txbxContent body, and
// an optional extra bodyPr fragment (e.g. an a:prstTxWarp for WordArt). It is
// shared by inline/anchored text boxes and by the members of a shape group.
func buildWspXML(id int, name, cNvSpPr, txbx string, xEMU, yEMU, cx, cy int64, shape ShapeType, fill, ln, bodyPrExtra string) string {
	return fmt.Sprintf(
		`<wps:wsp>`+
			`<wps:cNvPr id="%d" name="%s"/>`+
			`%s`+
			`<wps:spPr>`+
			`<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`+
			`<a:prstGeom prst="%s"><a:avLst/></a:prstGeom>`+
			`%s%s`+
			`</wps:spPr>`+
			`%s`+
			`<wps:bodyPr rot="0" vert="horz" wrap="square" lIns="91440" tIns="45720" rIns="91440" bIns="45720" anchor="t" anchorCtr="0">%s<a:noAutofit/></wps:bodyPr>`+
			`</wps:wsp>`,
		id, xmlb.EscapeAttrValue(name),
		cNvSpPr,
		xEMU, yEMU, cx, cy,
		xmlb.EscapeAttrValue(string(shape)),
		fill, ln,
		txbx,
		bodyPrExtra,
	)
}

// vmlTextBoxNamespaces are the namespace declarations placed on the VML v:shape
// of a down-level text box fallback: the VML namespace and the Office drawing
// namespace, plus the WordprocessingML namespace so the w:txbxContent body
// resolves. r: is already bound at the document root.
const vmlTextBoxNamespaces = `xmlns:v="urn:schemas-microsoft-com:vml" ` +
	`xmlns:o="urn:schemas-microsoft-com:office:office" ` +
	`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

// buildTextBoxAlternateContentXML builds the inner content of an
// mc:AlternateContent element: a Choice (Requires="wps") carrying the modern
// DrawingML w:drawing and a Fallback carrying an equivalent legacy VML w:pict
// text box. Word writes this pair so readers without wps support still render
// the box. The mc prefix is bound at the document root.
func buildTextBoxAlternateContentXML(id int, tb *TextBox, opts TextBoxOptions, isTextBox bool) []byte {
	drawing := buildShapeDrawingXML(id, tb, opts, isTextBox)
	fallback := buildVMLTextBoxXML(id, tb, opts, isTextBox)
	return []byte(`<mc:Choice Requires="wps"><w:drawing>` + string(drawing) + `</w:drawing></mc:Choice>` +
		`<mc:Fallback>` + fallback + `</mc:Fallback>`)
}

// buildVMLTextBoxXML builds a legacy VML w:pict text box equivalent to the wps
// shape: a v:shape (or v:rect) sized in points with a fill/stroke and a
// v:textbox wrapping the same w:txbxContent body.
func buildVMLTextBoxXML(id int, tb *TextBox, opts TextBoxOptions, isTextBox bool) string {
	widthPt := emuToPoints(tb.widthEMU)
	heightPt := emuToPoints(tb.heightEMU)

	fill := `fillcolor="#ffffff"`
	if opts.NoFill {
		fill = `filled="f"`
	} else if opts.FillColor != "" {
		fill = `fillcolor="#` + xmlb.EscapeAttrValue(strings.ToLower(opts.FillColor)) + `"`
	} else if !isTextBox {
		fill = `filled="f"`
	}

	stroke := `strokecolor="#000000"`
	if opts.NoBorder {
		stroke = `stroked="f"`
	} else if opts.BorderColor != "" {
		stroke = `strokecolor="#` + xmlb.EscapeAttrValue(strings.ToLower(opts.BorderColor)) + `"`
	}

	style := fmt.Sprintf("width:%.2fpt;height:%.2fpt", widthPt, heightPt)
	return fmt.Sprintf(
		`<w:pict>`+
			`<v:shape id="TextBox_%d" %s style="%s" %s %s>`+
			`<v:textbox><w:txbxContent>%s</w:txbxContent></v:textbox>`+
			`</v:shape>`+
			`</w:pict>`,
		id, vmlTextBoxNamespaces, style, fill, stroke,
		txbxContentXML(tb.text),
	)
}

// fillXML builds the a:solidFill / a:noFill fragment for a shape's spPr. Text
// boxes default to a white fill; basic shapes default to no fill.
func fillXML(opts TextBoxOptions, isTextBox bool) string {
	if opts.NoFill {
		return `<a:noFill/>`
	}
	color := opts.FillColor
	if color == "" {
		if !isTextBox {
			return ""
		}
		color = "FFFFFF"
	}
	return fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, xmlb.EscapeAttrValue(color))
}

// lnXML builds the a:ln (outline) fragment for a shape's spPr.
func lnXML(opts TextBoxOptions) string {
	if opts.NoBorder {
		return `<a:ln><a:noFill/></a:ln>`
	}
	color := opts.BorderColor
	if color == "" {
		color = "000000"
	}
	width := opts.BorderWidthEMU
	if width <= 0 {
		width = defaultBorderWidthEMU
	}
	return fmt.Sprintf(
		`<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln>`,
		width, xmlb.EscapeAttrValue(color))
}

// txbxContentXML builds the WordprocessingML body of a text box: one w:p per
// line of text (splitting on newlines), each carrying a single run. An empty
// text still yields one empty paragraph, which w:txbxContent requires.
func txbxContentXML(text string) string {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(`<w:p>`)
		if line != "" {
			b.WriteString(`<w:r><w:t xml:space="preserve">`)
			b.WriteString(xmlb.EscapeText(line))
			b.WriteString(`</w:t></w:r>`)
		}
		b.WriteString(`</w:p>`)
	}
	return b.String()
}

// --- read ---

// TextBoxes returns every text box in the document, in document order,
// including boxes nested in tables, headers, and footers. Both modern
// DrawingML (wps) text boxes and legacy VML (w:pict/v:textbox) text boxes are
// returned; each handle exposes the box's text and geometry. Shapes without a
// text body are not returned.
func (d *Document) TextBoxes() []*TextBox {
	var out []*TextBox
	if d.doc() != nil && d.doc().Body != nil {
		for _, p := range d.doc().Body.AllParagraphs() {
			out = appendParagraphTextBoxes(out, p)
		}
	}
	// AllParagraphs, not the top-level P slice: a text box inside a header
	// table (a very common layout) was invisible while the godoc promised
	// "including boxes nested in tables, headers, and footers" (C490).
	for _, hp := range d.sortedHeaderParts() {
		if hp == nil || hp.hdr == nil {
			continue
		}
		for _, p := range hp.hdr.AllParagraphs() {
			out = appendParagraphTextBoxes(out, p)
		}
	}
	for _, fp := range d.sortedFooterParts() {
		if fp == nil || fp.ftr == nil {
			continue
		}
		for _, p := range fp.ftr.AllParagraphs() {
			out = appendParagraphTextBoxes(out, p)
		}
	}
	return out
}

// appendParagraphTextBoxes appends the text boxes from a paragraph's runs
// (including runs nested in hyperlinks and tracked changes) to out. A run can
// carry a text box as a direct w:drawing, inside an mc:AlternateContent
// wrapper, or as a legacy VML w:pict.
func appendParagraphTextBoxes(out []*TextBox, p *oxml.CT_P) []*TextBox {
	for _, cr := range oxmlParagraphRuns(p) {
		for _, dr := range cr.Drawing {
			if dr == nil {
				continue
			}
			if tb := parseTextBox(dr.RawContent, false, dr); tb != nil {
				out = append(out, tb)
			}
		}
		for _, ac := range cr.AlternateContent {
			if ac == nil {
				continue
			}
			if tb := parseTextBox(ac.RawContent, false, nil); tb != nil {
				out = append(out, tb)
			}
		}
		for _, pict := range cr.Pict {
			if pict == nil {
				continue
			}
			if tb := parseTextBox(pict.RawContent, true, nil); tb != nil {
				out = append(out, tb)
			}
		}
	}
	return out
}

// parseTextBox builds a TextBox handle from a drawing's raw content when it
// carries a w:txbxContent body, extracting the text and geometry. It returns
// nil for drawings that are not text boxes (images, plain shapes, charts).
// Only the first w:txbxContent is read, so an mc:AlternateContent that repeats
// the body in a VML fallback yields the text once.
func parseTextBox(raw []byte, vml bool, dr *oxml.CT_Drawing) *TextBox {
	if !bytes.Contains(raw, []byte("txbxContent")) {
		return nil
	}
	floating, cx, cy, prst := scanDrawingGeometry(raw)
	tb := &TextBox{
		text:     extractTxbxText(raw),
		floating: floating,
		vml:      vml,
		shape:    ShapeType(prst),
		drawing:  dr,
	}
	if vml {
		tb.widthEMU, tb.heightEMU = vmlSizeEMU(raw)
	} else {
		tb.widthEMU = cx
		tb.heightEMU = cy
	}
	return tb
}

// scanDrawingGeometry decodes a drawing fragment to read its wrapper geometry:
// whether it is floating (a wp:anchor wrapper rather than wp:inline), its
// wp:extent size, and its a:prstGeom preset shape. It walks the fragment with
// the XML decoder rather than substring-matching raw bytes, because the box's
// own escaped body text can contain literals like ":anchor" or a prst="…"
// attribute that a byte scan would misread. Elements inside the w:txbxContent
// body are ignored so only the wrapper's own geometry is reported. Undeclared
// prefixes (wp:, a:, w:) are tolerated: the decoder still reports the local
// name. Values are 0/"" when the fragment carries no such element.
func scanDrawingGeometry(raw []byte) (floating bool, cx, cy int64, prst string) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	txbxDepth := 0
	haveExtent := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "txbxContent" {
				txbxDepth++
				continue
			}
			if txbxDepth > 0 {
				// Inside the text body: not part of the wrapper geometry.
				continue
			}
			switch t.Name.Local {
			case "anchor":
				floating = true
			case "extent":
				if !haveExtent {
					cx = attrInt(t.Attr, "cx")
					cy = attrInt(t.Attr, "cy")
					haveExtent = true
				}
			case "prstGeom":
				if prst == "" {
					prst = attrStr(t.Attr, "prst")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "txbxContent" && txbxDepth > 0 {
				txbxDepth--
			}
		}
	}
	return floating, cx, cy, prst
}

// attrStr returns the value of the attribute with the given local name, or "".
func attrStr(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// attrInt returns the attribute with the given local name parsed as an int64,
// or 0 when absent or unparseable.
func attrInt(attrs []xml.Attr, local string) int64 {
	n, err := strconv.ParseInt(attrStr(attrs, local), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// extractTxbxText extracts the plain text of the first w:txbxContent in raw,
// joining its paragraphs with newlines. It decodes the fragment with the XML
// decoder (undeclared prefixes such as w: are tolerated: the local name is
// still reported), so it is robust to attributes and escaping and works for
// both DrawingML and VML text bodies.
func extractTxbxText(raw []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var lines []string
	var cur strings.Builder
	inTxbx := false
	inText := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "txbxContent":
				if !inTxbx {
					inTxbx = true
				}
			case "t", "delText":
				if inTxbx {
					inText++
				}
			case "br", "cr":
				if inTxbx {
					cur.WriteByte('\n')
				}
			case "tab", "ptab":
				if inTxbx {
					cur.WriteByte('\t')
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "txbxContent":
				if inTxbx {
					if cur.Len() > 0 {
						lines = append(lines, cur.String())
					}
					return strings.Join(lines, "\n")
				}
			case "t", "delText":
				if inText > 0 {
					inText--
				}
			case "p":
				if inTxbx {
					lines = append(lines, cur.String())
					cur.Reset()
				}
			}
		case xml.CharData:
			if inTxbx && inText > 0 {
				cur.Write(t)
			}
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return strings.Join(lines, "\n")
}

// vmlSizeEMU extracts a VML shape's width and height from its style attribute
// (e.g. style="width:212.4pt;height:106.2pt"), converting points to EMU. It
// returns 0,0 when the dimensions are absent or not point-valued.
func vmlSizeEMU(raw []byte) (widthEMU, heightEMU int64) {
	i := bytes.Index(raw, []byte(` style="`))
	if i < 0 {
		return 0, 0
	}
	seg := raw[i+len(` style="`):]
	if end := bytes.IndexByte(seg, '"'); end >= 0 {
		seg = seg[:end]
	}
	return vmlDimEMU(seg, "width:"), vmlDimEMU(seg, "height:")
}

// vmlDimEMU parses a single "width:"/"height:" declaration expressed in points
// from a VML style string, returning its EMU value (0 if absent or non-pt).
func vmlDimEMU(style []byte, key string) int64 {
	i := bytes.Index(style, []byte(key))
	if i < 0 {
		return 0
	}
	rest := style[i+len(key):]
	if end := bytes.IndexByte(rest, ';'); end >= 0 {
		rest = rest[:end]
	}
	v := strings.TrimSpace(string(rest))
	if !strings.HasSuffix(v, "pt") {
		return 0
	}
	pt, err := strconv.ParseFloat(strings.TrimSuffix(v, "pt"), 64)
	if err != nil {
		return 0
	}
	return pointsToEMU(pt)
}
