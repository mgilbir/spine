package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

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
	b.WriteString(`<v:shapetype id="_x0000_t202" coordsize="21600,21600" o:spt="202"` + "\n")
	b.WriteString(`  path="m0,0l0,21600,21600,21600,21600,0xe">` + "\n")
	b.WriteString(`  <v:stroke joinstyle="miter"/>` + "\n")
	b.WriteString(`  <v:path gradientshapeok="t" o:connecttype="rect"/>` + "\n")
	b.WriteString(` </v:shapetype>`)

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
	b.WriteString(`</xml>` + "\n")
	return []byte(b.String())
}
