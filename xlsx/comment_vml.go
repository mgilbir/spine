package xlsx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// vmlNoteShapetype is the shared textbox shapetype (_x0000_t202) every legacy
// comment note shape references. It is emitted once per VML drawing that holds
// note shapes.
const vmlNoteShapetype = `<v:shapetype id="_x0000_t202" coordsize="21600,21600" o:spt="202"` + "\n" +
	`  path="m0,0l0,21600,21600,21600,21600,0xe">` + "\n" +
	`  <v:stroke joinstyle="miter"/>` + "\n" +
	`  <v:path gradientshapeok="t" o:connecttype="rect"/>` + "\n" +
	` </v:shapetype>`

// buildCommentVML renders the legacy VML drawing that gives each legacy comment
// a note box shape. The output is a valid vmlDrawing part: a shared shapelayout
// and textbox shapetype, followed by one hidden textbox shape per comment,
// anchored to the commented cell via <x:ClientData ObjectType="Note">.
//
// The geometry is approximate (Excel recomputes it from column widths on open);
// the shapes are hidden until hovered, so exact placement is not important for
// correctness. Byte-identical fidelity is only required for unmodified saves,
// where the original VML is preserved verbatim instead of regenerated.
func buildCommentVML(c *oxml.CT_Comments) []byte {
	var b strings.Builder
	b.WriteString(`<xml xmlns:v="urn:schemas-microsoft-com:vml"` + "\n")
	b.WriteString(` xmlns:o="urn:schemas-microsoft-com:office:office"` + "\n")
	b.WriteString(` xmlns:x="urn:schemas-microsoft-com:office:excel">` + "\n")
	b.WriteString(` <o:shapelayout v:ext="edit">` + "\n")
	b.WriteString(`  <o:idmap v:ext="edit" data="1"/>` + "\n")
	b.WriteString(` </o:shapelayout>`)
	b.WriteString(vmlNoteShapetype)
	b.WriteString(buildCommentVMLShapes(c))
	b.WriteString(`</xml>` + "\n")
	return []byte(b.String())
}

// buildCommentVMLShapes renders just the per-comment note box shapes (no root,
// shapelayout, or shapetype), so they can be appended to an existing VML drawing
// that already carries other shapes (form controls, OLE pictures). Each shape's
// id uses the comment's assigned ShapeID (falling back to 1025+i only when
// unset).
func buildCommentVMLShapes(c *oxml.CT_Comments) string {
	var b strings.Builder
	for i := range c.Comments {
		cm := &c.Comments[i]
		row, col, err := ParseCellRef(cm.Ref)
		if err != nil {
			continue
		}
		row0, col0 := row-1, col-1
		shapeID := cm.ShapeID
		if shapeID == "" {
			shapeID = fmt.Sprintf("%d", 1025+i)
		}
		marginLeft := (col0 + 1) * 48
		marginTop := row0 * 15
		anchor := fmt.Sprintf("%d, 15, %d, 2, %d, 15, %d, 16",
			col0+1, row0, col0+3, row0+4)

		fmt.Fprintf(&b, `<v:shape id="_x0000_s%s" type="#_x0000_t202" style='position:absolute;`+"\n", shapeID)
		fmt.Fprintf(&b, `  margin-left:%dpt;margin-top:%dpt;width:108pt;height:59.25pt;z-index:%d;`+"\n", marginLeft, marginTop, i+1)
		b.WriteString(`  visibility:hidden' fillcolor="#ffffe1" o:insetmode="auto">` + "\n")
		b.WriteString(`  <v:fill color2="#ffffe1"/>` + "\n")
		b.WriteString(`  <v:shadow on="t" color="black" obscured="t"/>` + "\n")
		b.WriteString(`  <v:path o:connecttype="none"/>` + "\n")
		b.WriteString(`  <v:textbox style='mso-direction-alt:auto'>` + "\n")
		b.WriteString(`   <div style='text-align:left'></div>` + "\n")
		b.WriteString(`  </v:textbox>` + "\n")
		b.WriteString(`  <x:ClientData ObjectType="Note">` + "\n")
		b.WriteString(`   <x:MoveWithCells/>` + "\n")
		b.WriteString(`   <x:SizeWithCells/>` + "\n")
		fmt.Fprintf(&b, `   <x:Anchor>%s</x:Anchor>`+"\n", anchor)
		b.WriteString(`   <x:AutoFill>False</x:AutoFill>` + "\n")
		fmt.Fprintf(&b, `   <x:Row>%d</x:Row>`+"\n", row0)
		fmt.Fprintf(&b, `   <x:Column>%d</x:Column>`+"\n", col0)
		b.WriteString(`  </x:ClientData>` + "\n")
		b.WriteString(` </v:shape>`)
	}
	return b.String()
}

// composeLegacyVML regenerates the sheet's legacy VML drawing for its comment
// notes while preserving every shape it does not own (form controls, OLE
// pictures). A sheet's comments, controls, and OLE objects all share one legacy
// VML part; a naive rewrite that emits only note shapes would destroy the other
// shapes and cross-wire their shape ids (a worksheet <control shapeId="1025">
// would resolve to a comment note box).
//
// When original is empty or unparseable the drawing is emitted fresh (note shape
// ids from 1025), matching buildCommentVML byte-for-byte. Otherwise the original
// non-note shapes (and its shapelayout/shapetypes) are re-emitted verbatim, the
// note-box shapetype is ensured present, and the comment note shapes are appended
// with ids allocated strictly above the max shape id already present so they
// never collide with a preserved shape.
func composeLegacyVML(original []byte, c *oxml.CT_Comments) []byte {
	header, preserved, maxID, ok := splitVMLNonNoteShapes(original)
	if !ok {
		assignShapeIDs(c)
		return buildCommentVML(c)
	}
	assignShapeIDsFrom(c, maxID+1)

	var b strings.Builder
	b.WriteString(header)
	b.WriteString(preserved)
	if !bytes.Contains(original, []byte(`id="_x0000_t202"`)) {
		b.WriteString(vmlNoteShapetype)
	}
	b.WriteString(buildCommentVMLShapes(c))
	b.WriteString(`</xml>` + "\n")
	return []byte(b.String())
}

// splitVMLNonNoteShapes parses a legacy VML drawing and returns its root start
// tag (header), the verbatim bytes of every direct child except comment note
// shapes (preserved), and the maximum numeric shape id present. It reports ok
// only when the drawing has a usable <xml> root; note shapes (a <v:shape> whose
// x:ClientData ObjectType is "Note") are dropped so they can be regenerated from
// the comment model.
func splitVMLNonNoteShapes(data []byte) (header, preserved string, maxID int, ok bool) {
	if len(data) == 0 {
		return "", "", 0, false
	}
	dec := xml.NewDecoder(bytes.NewReader(data))

	depth := 0
	var rootOpenEnd int64 = -1
	var childStart int64
	var childIsNote bool
	var b strings.Builder

	for {
		off := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case depth == 1:
				rootOpenEnd = dec.InputOffset()
			case depth == 2:
				childStart = off
				childIsNote = false
				if t.Name.Local == "shape" {
					for _, a := range t.Attr {
						if a.Name.Local == "id" {
							if n := numericSuffix(a.Value); n > maxID {
								maxID = n
							}
						}
					}
				}
			case t.Name.Local == "ClientData":
				for _, a := range t.Attr {
					if a.Name.Local == "ObjectType" && a.Value == "Note" {
						childIsNote = true
					}
				}
			}
		case xml.EndElement:
			if depth == 2 && !childIsNote {
				span := strings.TrimSpace(string(data[childStart:dec.InputOffset()]))
				if span != "" {
					b.WriteString("\n ")
					b.WriteString(span)
				}
			}
			depth--
		}
	}

	if rootOpenEnd < 0 {
		return "", "", 0, false
	}
	return string(data[:rootOpenEnd]), b.String(), maxID, true
}

// numericSuffix returns the trailing decimal digits of a VML shape id
// (e.g. "_x0000_s1025" -> 1025), or 0 when there are none.
func numericSuffix(id string) int {
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	if i == len(id) {
		return 0
	}
	n := 0
	for _, r := range id[i:] {
		n = n*10 + int(r-'0')
	}
	return n
}
