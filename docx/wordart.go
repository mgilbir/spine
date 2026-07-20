package docx

import (
	"fmt"
	"math"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// WarpPreset names a DrawingML preset text-warp geometry (an a:prstTxWarp prst
// value). The value is emitted verbatim, so any other preset name Word
// recognizes (textStop, textTriangle, ...) can be passed through as well.
type WarpPreset string

const (
	// WarpNone applies no warp: the text is laid out straight (still a WordArt
	// shape, with the fill/outline styling).
	WarpNone WarpPreset = ""
	// WarpArchUp bends the text into an upward arch.
	WarpArchUp WarpPreset = "textArchUp"
	// WarpArchDown bends the text into a downward arch.
	WarpArchDown WarpPreset = "textArchDown"
	// WarpCircle wraps the text around a full circle.
	WarpCircle WarpPreset = "textCircle"
	// WarpInflate inflates the text (bulging outward top and bottom).
	WarpInflate WarpPreset = "textInflate"
	// WarpDeflate deflates the text (pinching inward top and bottom).
	WarpDeflate WarpPreset = "textDeflate"
	// WarpChevronUp bends the text into an upward chevron.
	WarpChevronUp WarpPreset = "textChevron"
	// WarpWave1 gives the text a single wave.
	WarpWave1 WarpPreset = "textWave1"
)

// Default WordArt geometry: 3 inch wide, 0.75 inch tall (EMU, 914400/inch).
const (
	defaultWordArtWidthEMU  = 3 * 914400
	defaultWordArtHeightEMU = 914400 * 3 / 4
)

// defaultWordArtColor is the text fill used when WordArtOptions.FillColor is
// empty (the Office accent-1 blue, matching Word's default WordArt swatch).
const defaultWordArtColor = "4472C4"

// defaultWordArtFontSizePt is the point size used when FontSizePt is unset.
const defaultWordArtFontSizePt = 36

// WordArtOptions configures a WordArt shape created with AddWordArt. The zero
// value produces an inline, un-warped, 36pt blue caption 3in x 0.75in.
type WordArtOptions struct {
	// WidthEMU and HeightEMU are the shape size in EMU (914400 per inch). When
	// zero a default of 3in x 0.75in is used.
	WidthEMU  int64
	HeightEMU int64
	// Floating anchors the shape (positioned relative to the page or paragraph)
	// instead of placing it inline in the text flow.
	Floating bool
	// Anchor positions the shape when Floating is set (same semantics as images).
	Anchor Anchor
	// Warp selects the preset text-warp geometry; WarpNone lays the text out
	// straight.
	Warp WarpPreset
	// FillColor is the text fill as a hex "RRGGBB" string; empty uses the default
	// WordArt blue.
	FillColor string
	// FontSizePt is the text size in points; zero uses 36pt.
	FontSizePt float64
	// Bold makes the WordArt text bold.
	Bold bool
}

// AddWordArt appends a paragraph containing a WordArt shape to the document body
// and returns the box handle. It is a convenience wrapper over
// Paragraph.AddWordArt.
func (d *Document) AddWordArt(text string, opts WordArtOptions) *TextBox {
	return d.AddParagraph().AddWordArt(text, opts)
}

// AddWordArt inserts a WordArt shape into a new run at the end of the paragraph.
// A WordArt shape is a DrawingML (wps) text effect: a borderless, fill-less
// shape whose text carries a solid fill and an optional preset text warp
// (opts.Warp). Inline by default; set opts.Floating to anchor it. The shape
// needs no extra parts or relationships, so it round-trips like a text box, and
// its text is reported by Document.TextBoxes().
func (p *Paragraph) AddWordArt(text string, opts WordArtOptions) *TextBox {
	width := opts.WidthEMU
	if width <= 0 {
		width = defaultWordArtWidthEMU
	}
	height := opts.HeightEMU
	if height <= 0 {
		height = defaultWordArtHeightEMU
	}

	tb := &TextBox{
		text:      text,
		widthEMU:  width,
		heightEMU: height,
		floating:  opts.Floating,
		shape:     ShapeRectangle,
	}
	id := p.document.nextShapeID()
	drawing := &oxml.CT_Drawing{RawContent: buildWordArtDrawingXML(id, tb, opts)}
	tb.drawing = drawing
	p.AddRun().r.AppendDrawing(drawing)
	return tb
}

// buildWordArtDrawingXML builds the w:drawing inner content for a WordArt wps
// shape: a fill-less, outline-less shape with an optional a:prstTxWarp on its
// bodyPr and a centered, styled w:txbxContent body.
func buildWordArtDrawingXML(id int, tb *TextBox, opts WordArtOptions) []byte {
	name := fmt.Sprintf("WordArt %d", id)

	var warp string
	if opts.Warp != WarpNone {
		warp = fmt.Sprintf(`<a:prstTxWarp prst="%s"><a:avLst/></a:prstTxWarp>`, xmlEscapeAttr(string(opts.Warp)))
	}

	txbx := `<wps:txbx><w:txbxContent>` + wordArtTxbxContentXML(tb.text, opts) + `</w:txbxContent></wps:txbx>`
	wsp := buildWspXML(id, name, `<wps:cNvSpPr/>`, txbx, 0, 0, tb.widthEMU, tb.heightEMU,
		ShapeRectangle, `<a:noFill/>`, `<a:ln><a:noFill/></a:ln>`, warp)

	graphic := fmt.Sprintf(
		`<a:graphic><a:graphicData uri="%s">%s</a:graphicData></a:graphic>`,
		wpsGraphicDataURI, wsp)

	if tb.floating {
		return buildShapeAnchorXML(id, name, tb, TextBoxOptions{Anchor: opts.Anchor}, graphic)
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
		id, xmlEscapeAttr(name),
		graphic,
	))
}

// wordArtTxbxContentXML builds the WordprocessingML body of a WordArt shape: one
// centered w:p per line of text, each run carrying the fill color, size, and
// optional bold. The text fill is a plain w:color so the caption is legible in
// every reader; the WordArt character is carried by the shape geometry and warp.
func wordArtTxbxContentXML(text string, opts WordArtOptions) string {
	color := opts.FillColor
	if color == "" {
		color = defaultWordArtColor
	}
	sizePt := opts.FontSizePt
	if sizePt <= 0 {
		sizePt = defaultWordArtFontSizePt
	}
	sizeHalf := int(math.Round(sizePt * 2))

	var bold string
	if opts.Bold {
		bold = `<w:b/><w:bCs/>`
	}
	rPr := fmt.Sprintf(
		`<w:rPr>%s<w:color w:val="%s"/><w:sz w:val="%d"/><w:szCs w:val="%d"/></w:rPr>`,
		bold, xmlEscapeAttr(color), sizeHalf, sizeHalf)

	lines := strings.Split(text, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr>`)
		if line != "" {
			b.WriteString(`<w:r>`)
			b.WriteString(rPr)
			b.WriteString(`<w:t xml:space="preserve">`)
			b.WriteString(xmlEscapeText(line))
			b.WriteString(`</w:t></w:r>`)
		}
		b.WriteString(`</w:p>`)
	}
	return b.String()
}
