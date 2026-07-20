package docx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// wpgGraphicDataURI marks a drawing's a:graphicData as a wordprocessingGroup.
const wpgGraphicDataURI = "http://schemas.microsoft.com/office/word/2010/wordprocessingGroup"

// wpgShapeNamespaces extends the wps shape namespaces with the
// wordprocessingGroup (wpg) namespace that carries the group shape.
const wpgShapeNamespaces = wpsShapeNamespaces +
	` xmlns:wpg="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup"`

// GroupOptions configures a shape group created with AddShapeGroup. The zero
// value produces an inline group whose extent is the bounding box of its
// members.
type GroupOptions struct {
	// WidthEMU and HeightEMU are the group extent in EMU (914400 per inch). When
	// zero the extent is computed from the members' bounding box.
	WidthEMU  int64
	HeightEMU int64
	// Floating anchors the group (positioned relative to the page or paragraph)
	// instead of placing it inline in the text flow.
	Floating bool
	// Anchor positions the group when Floating is set (same semantics as images).
	Anchor Anchor
}

// GroupMember describes one shape inside a shape group. Its position is given in
// the group's child coordinate space (the same EMU space as the group extent),
// so XEMU/YEMU place the member relative to the group's top-left corner.
type GroupMember struct {
	// Text is the member's caption; empty for a plain shape. A member with text
	// carries a real w:txbxContent body.
	Text string
	// Shape selects the preset geometry; empty means ShapeRectangle.
	Shape ShapeType
	// XEMU and YEMU offset the member from the group's top-left corner.
	XEMU int64
	YEMU int64
	// WidthEMU and HeightEMU are the member size in EMU; zero uses the text box
	// default (2in x 1in).
	WidthEMU  int64
	HeightEMU int64
	// FillColor / NoFill and BorderColor / BorderWidthEMU / NoBorder mirror
	// TextBoxOptions: fill and outline styling for the member.
	FillColor      string
	NoFill         bool
	BorderColor    string
	BorderWidthEMU int64
	NoBorder       bool
}

// AddShapeGroup appends a paragraph containing a shape group to the document
// body and returns the group handle. It is a convenience wrapper over
// Paragraph.AddShapeGroup.
func (d *Document) AddShapeGroup(opts GroupOptions, members ...GroupMember) *TextBox {
	return d.AddParagraph().AddShapeGroup(opts, members...)
}

// AddShapeGroup inserts a DrawingML shape group (a wpg:wgp holding several wps
// shapes/text boxes) into a new run at the end of the paragraph. Each member is
// positioned in the group's child coordinate space. The group needs no extra
// parts or relationships, so it round-trips like a text box. The returned handle
// reports the group extent and the members' joined text.
func (p *Paragraph) AddShapeGroup(opts GroupOptions, members ...GroupMember) *TextBox {
	width, height := opts.WidthEMU, opts.HeightEMU
	if width <= 0 || height <= 0 {
		bw, bh := groupBounds(members)
		if width <= 0 {
			width = bw
		}
		if height <= 0 {
			height = bh
		}
	}

	var texts []string
	for _, m := range members {
		if m.Text != "" {
			texts = append(texts, m.Text)
		}
	}

	tb := &TextBox{
		text:      strings.Join(texts, "\n"),
		widthEMU:  width,
		heightEMU: height,
		floating:  opts.Floating,
		shape:     ShapeRectangle,
	}
	id := p.document.nextShapeID()
	drawing := &oxml.CT_Drawing{RawContent: buildGroupDrawingXML(id, tb, opts, members, p.document)}
	tb.drawing = drawing
	p.AddRun().r.AppendDrawing(drawing)
	return tb
}

// groupBounds returns the bounding-box width and height of the members (the
// maximum of XEMU+WidthEMU and YEMU+HeightEMU), used as the group extent when
// GroupOptions does not set one. It falls back to the text box default size for
// members that omit their own.
func groupBounds(members []GroupMember) (widthEMU, heightEMU int64) {
	for _, m := range members {
		w, h := m.WidthEMU, m.HeightEMU
		if w <= 0 {
			w = defaultTextBoxWidthEMU
		}
		if h <= 0 {
			h = defaultTextBoxHeightEMU
		}
		if right := m.XEMU + w; right > widthEMU {
			widthEMU = right
		}
		if bottom := m.YEMU + h; bottom > heightEMU {
			heightEMU = bottom
		}
	}
	if widthEMU <= 0 {
		widthEMU = defaultTextBoxWidthEMU
	}
	if heightEMU <= 0 {
		heightEMU = defaultTextBoxHeightEMU
	}
	return widthEMU, heightEMU
}

// buildGroupDrawingXML builds the w:drawing inner content for a wpg group: a
// graphic → graphicData(uri=wpg) → wpg:wgp with a group xfrm and one wps:wsp per
// member.
func buildGroupDrawingXML(id int, tb *TextBox, opts GroupOptions, members []GroupMember, doc *Document) []byte {
	name := fmt.Sprintf("Group %d", id)

	var membersXML strings.Builder
	for _, m := range members {
		membersXML.WriteString(buildGroupMemberXML(doc.nextShapeID(), m))
	}

	wgp := fmt.Sprintf(
		`<wpg:wgp>`+
			`<wpg:cNvGrpSpPr/>`+
			`<wpg:grpSpPr>`+
			`<a:xfrm>`+
			`<a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/>`+
			`<a:chOff x="0" y="0"/><a:chExt cx="%d" cy="%d"/>`+
			`</a:xfrm>`+
			`</wpg:grpSpPr>`+
			`%s`+
			`</wpg:wgp>`,
		tb.widthEMU, tb.heightEMU, tb.widthEMU, tb.heightEMU,
		membersXML.String(),
	)

	graphic := fmt.Sprintf(
		`<a:graphic><a:graphicData uri="%s">%s</a:graphicData></a:graphic>`,
		wpgGraphicDataURI, wgp)

	if tb.floating {
		return buildGroupAnchorXML(id, name, tb, opts, graphic)
	}
	return []byte(fmt.Sprintf(
		`<wp:inline distT="0" distB="0" distL="0" distR="0" `+wpgShapeNamespaces+`>`+
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

// buildGroupMemberXML builds a single wps:wsp for a group member, placed at its
// offset in the group's child coordinate space.
func buildGroupMemberXML(id int, m GroupMember) string {
	shape := m.Shape
	if shape == "" {
		shape = ShapeRectangle
	}
	w, h := m.WidthEMU, m.HeightEMU
	if w <= 0 {
		w = defaultTextBoxWidthEMU
	}
	if h <= 0 {
		h = defaultTextBoxHeightEMU
	}
	isTextBox := m.Text != ""

	name := fmt.Sprintf("Shape %d", id)
	cNvSpPr := `<wps:cNvSpPr/>`
	var txbx string
	if isTextBox {
		name = fmt.Sprintf("Text Box %d", id)
		cNvSpPr = `<wps:cNvSpPr txBox="1"/>`
		txbx = `<wps:txbx><w:txbxContent>` + txbxContentXML(m.Text) + `</w:txbxContent></wps:txbx>`
	}

	memberOpts := TextBoxOptions{
		FillColor:      m.FillColor,
		NoFill:         m.NoFill,
		BorderColor:    m.BorderColor,
		BorderWidthEMU: m.BorderWidthEMU,
		NoBorder:       m.NoBorder,
	}
	return buildWspXML(id, name, cNvSpPr, txbx, m.XEMU, m.YEMU, w, h,
		shape, fillXML(memberOpts, isTextBox), lnXML(memberOpts), "")
}

// buildGroupAnchorXML wraps a group graphic in a floating wp:anchor, mirroring
// buildShapeAnchorXML but with the wpg namespace declared.
func buildGroupAnchorXML(id int, name string, tb *TextBox, opts GroupOptions, graphic string) []byte {
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
			wpgShapeNamespaces+`>`+
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
		id, xmlEscapeAttr(name),
		graphic,
	))
}
