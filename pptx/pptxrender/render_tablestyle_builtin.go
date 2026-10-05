package pptxrender

// PowerPoint's built-in table styles.
//
// A table's a:tableStyleId may name one of 74 styles that Office knows by
// GUID. PowerPoint does not write their definitions into ppt/tableStyles.xml,
// so a renderer has to carry them. This file writes each one as the
// a:tblStyle it stands for: parts (a:wholeTbl, a:band1H, a:firstRow, ...) with
// theme colors, so that every style goes through the same resolution as a
// style a deck defines.
//
// The ten families and their accents come from the style names Office
// publishes. How each family looks was measured against PowerPoint's own
// export of every style: fills, border widths and colors, and the text's
// color and weight. The rules of the families are also described in
// LibreOffice's oox/source/drawingml/table/predefined-table-styles.cxx
// (MPL-2.0, Copyright the LibreOffice contributors), which was read as
// documentation; where PowerPoint's output disagreed (border widths, the
// fills of the dark styles, a few borders it suppresses), the output decided.
// No code or table from that file is reproduced.

import (
	"github.com/mgilbir/spine/common/dml"
)

// tableStyleFamily is a group of built-in styles that differ in their accent.
type tableStyleFamily int

const (
	themed1 tableStyleFamily = iota // "No Style, No Grid" and its accents
	themed2                         // "No Style, Table Grid" and its accents
	light1
	light2
	light3
	medium1
	medium2
	medium3
	medium4
	dark1
	dark2
)

// builtinTableStyleIDs maps a built-in style's GUID to its family and
// accent; the accent is 0 for the style with none.
var builtinTableStyleIDs = map[string]struct {
	family tableStyleFamily
	accent int
}{
	"{2D5ABB26-0587-4C30-8999-92F81FD0307C}": {themed1, 0},
	"{3C2FFA5D-87B4-456A-9821-1D502468CF0F}": {themed1, 1},
	"{284E427A-3D55-4303-BF80-6455036E1DE7}": {themed1, 2},
	"{69C7853C-536D-4A76-A0AE-DD22124D55A5}": {themed1, 3},
	"{775DCB02-9BB8-47FD-8907-85C794F793BA}": {themed1, 4},
	"{35758FB7-9AC5-4552-8A53-C91805E547FA}": {themed1, 5},
	"{08FB837D-C827-4EFA-A057-4D05807E0F7C}": {themed1, 6},
	"{5940675A-B579-460E-94D1-54222C63F5DA}": {themed2, 0},
	"{D113A9D2-9D6B-4929-AA2D-F23B5EE8CBE7}": {themed2, 1},
	"{18603FDC-E32A-4AB5-989C-0864C3EAD2B8}": {themed2, 2},
	"{306799F8-075E-4A3A-A7F6-7FBC6576F1A4}": {themed2, 3},
	"{E269D01E-BC32-4049-B463-5C60D7B0CCD2}": {themed2, 4},
	"{327F97BB-C833-4FB7-BDE5-3F7075034690}": {themed2, 5},
	"{638B1855-1B75-4FBE-930C-398BA8C253C6}": {themed2, 6},
	"{9D7B26C5-4107-4FEC-AEDC-1716B250A1EF}": {light1, 0},
	"{3B4B98B0-60AC-42C2-AFA5-B58CD77FA1E5}": {light1, 1},
	"{0E3FDE45-AF77-4B5C-9715-49D594BDF05E}": {light1, 2},
	"{C083E6E3-FA7D-4D7B-A595-EF9225AFEA82}": {light1, 3},
	"{D27102A9-8310-4765-A935-A1911B00CA55}": {light1, 4},
	"{5FD0F851-EC5A-4D38-B0AD-8093EC10F338}": {light1, 5},
	"{68D230F3-CF80-4859-8CE7-A43EE81993B5}": {light1, 6},
	"{7E9639D4-E3E2-4D34-9284-5A2195B3D0D7}": {light2, 0},
	"{69012ECD-51FC-41F1-AA8D-1B2483CD663E}": {light2, 1},
	"{72833802-FEF1-4C79-8D5D-14CF1EAF98D9}": {light2, 2},
	"{F2DE63D5-997A-4646-A377-4702673A728D}": {light2, 3},
	"{17292A2E-F333-43FB-9621-5CBBE7FDCDCB}": {light2, 4},
	"{5A111915-BE36-4E01-A7E5-04B1672EAD32}": {light2, 5},
	"{912C8C85-51F0-491E-9774-3900AFEF0FD7}": {light2, 6},
	"{616DA210-FB5B-4158-B5E0-FEB733F419BA}": {light3, 0},
	"{BC89EF96-8CEA-46FF-86C4-4CE0E7609802}": {light3, 1},
	"{5DA37D80-6434-44D0-A028-1B22A696006F}": {light3, 2},
	"{8799B23B-EC83-4686-B30A-512413B5E67A}": {light3, 3},
	"{ED083AE6-46FA-4A59-8FB0-9F97EB10719F}": {light3, 4},
	"{BDBED569-4797-4DF1-A0F4-6AAB3CD982D8}": {light3, 5},
	"{E8B1032C-EA38-4F05-BA0D-38AFFFC7BED3}": {light3, 6},
	"{793D81CF-94F2-401A-BA57-92F5A7B2D0C5}": {medium1, 0},
	"{B301B821-A1FF-4177-AEE7-76D212191A09}": {medium1, 1},
	"{9DCAF9ED-07DC-4A11-8D7F-57B35C25682E}": {medium1, 2},
	"{1FECB4D8-DB02-4DC6-A0A2-4F2EBAE1DC90}": {medium1, 3},
	"{1E171933-4619-4E11-9A3F-F7608DF75F80}": {medium1, 4},
	"{FABFCF23-3B69-468F-B69F-88F6DE6A72F2}": {medium1, 5},
	"{10A1B5D5-9B99-4C35-A422-299274C87663}": {medium1, 6},
	"{073A0DAA-6AF3-43AB-8588-CEC1D06C72B9}": {medium2, 0},
	"{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}": {medium2, 1},
	"{21E4AEA4-8DFA-4A89-87EB-49C32662AFE0}": {medium2, 2},
	"{F5AB1C69-6EDB-4FF4-983F-18BD219EF322}": {medium2, 3},
	"{00A15C55-8517-42AA-B614-E9B94910E393}": {medium2, 4},
	"{7DF18680-E054-41AD-8BC1-D1AEF772440D}": {medium2, 5},
	"{93296810-A885-4BE3-A3E7-6D5BEEA58F35}": {medium2, 6},
	"{8EC20E35-A176-4012-BC5E-935CFFF8708E}": {medium3, 0},
	"{6E25E649-3F16-4E02-A733-19D2CDBF48F0}": {medium3, 1},
	"{85BE263C-DBD7-4A20-BB59-AAB30ACAA65A}": {medium3, 2},
	"{EB344D84-9AFB-497E-A393-DC336BA19D2E}": {medium3, 3},
	"{EB9631B5-78F2-41C9-869B-9F39066F8104}": {medium3, 4},
	"{74C1A8A3-306A-4EB7-A6B1-4F7E0EB9C5D6}": {medium3, 5},
	"{2A488322-F2BA-4B5B-9748-0D474271808F}": {medium3, 6},
	"{D7AC3CCA-C797-4891-BE02-D94E43425B78}": {medium4, 0},
	"{69CF1AB2-1976-4502-BF36-3FF5EA218861}": {medium4, 1},
	"{8A107856-5554-42FB-B03E-39F5DBC370BA}": {medium4, 2},
	"{0505E3EF-67EA-436B-97B2-0124C06EBD24}": {medium4, 3},
	"{C4B1156A-380E-4F78-BDF5-A606A8083BF9}": {medium4, 4},
	"{22838BEF-8BB2-4498-84A7-C5851F593DF1}": {medium4, 5},
	"{16D9F66E-5EB9-4882-86FB-DCBF35E3C3E4}": {medium4, 6},
	"{E8034E78-7F5D-4C2E-B375-FC64B27BC917}": {dark1, 0},
	"{125E5076-3810-47DD-B79F-674D7AD40C01}": {dark1, 1},
	"{37CE84F3-28C3-443E-9E96-99CF82512B78}": {dark1, 2},
	"{D03447BB-5D67-496B-8E87-E561075AD55C}": {dark1, 3},
	"{E929F9F4-4A8F-4326-A1B4-22849713DDAB}": {dark1, 4},
	"{8FD4443E-F989-4FC4-A0C8-D5A2AF1F390B}": {dark1, 5},
	"{AF606853-7671-496A-8E4F-DF71F8EC918B}": {dark1, 6},
	"{5202B0CA-FC54-4496-8BCA-5EF66A818D29}": {dark2, 0},
	"{0660B408-B3CF-4A94-85FC-2B1E0A45F4A2}": {dark2, 1},
	"{91EBBBCC-DAD2-459C-BE2E-F6DE35CF9A28}": {dark2, 3},
	"{46F890A9-2807-4EBB-B81D-B2AA78EC7F39}": {dark2, 5},
}

// builtinTableStyle returns the built-in style a GUID names, or nil. The
// result is built anew; callers may keep it.
func builtinTableStyle(id string) *dml.TableStyle {
	b, ok := builtinTableStyleIDs[id]
	if !ok {
		return nil
	}
	return buildTableStyle(id, b.family, b.accent)
}

// --- building blocks

// bClr is a theme color with at most one transform.
func bClr(name string) *dml.SchemeClrTransform { return &dml.SchemeClrTransform{Val: name} }

func bTint(name string, thousandths int32) *dml.SchemeClrTransform {
	return &dml.SchemeClrTransform{Val: name, Tint: []*dml.ColorTransform{{Val: dml.NewPercentage(thousandths)}}}
}

func bShade(name string, thousandths int32) *dml.SchemeClrTransform {
	return &dml.SchemeClrTransform{Val: name, Shade: []*dml.ColorTransform{{Val: dml.NewPercentage(thousandths)}}}
}

func bAlpha(name string, thousandths int32) *dml.SchemeClrTransform {
	return &dml.SchemeClrTransform{Val: name, Alpha: []*dml.ColorTransform{{Val: dml.NewPercentage(thousandths)}}}
}

// Line widths, in EMU.
const (
	bPt1   int64 = 12700
	bPt1_5 int64 = 19050
	bPt2   int64 = 25400
	bPt3   int64 = 38100
	bPt4   int64 = 50800
)

// bLine is a border: a single line of the given width, or a double one.
func bLine(w int64, c *dml.SchemeClrTransform) *dml.ThemeableLineStyle {
	return &dml.ThemeableLineStyle{Ln: &dml.Ln{W: &w, Cmpd: "sng", SolidFill: &dml.SolidFill{SchemeClr: c}}}
}

func bDouble(w int64, c *dml.SchemeClrTransform) *dml.ThemeableLineStyle {
	l := bLine(w, c)
	l.Ln.Cmpd = "dbl"
	return l
}

// bNone is a border that is explicitly absent, which overrides the parts
// below it.
func bNone() *dml.ThemeableLineStyle {
	return &dml.ThemeableLineStyle{Ln: &dml.Ln{NoFill: &dml.NoFillXML{}}}
}

// bPart is one table part: its text, fill and borders.
type bPart struct {
	bold   bool
	text   *dml.SchemeClrTransform
	fill   *dml.SchemeClrTransform
	noFill bool
	bdr    dml.TcBdr
}

func (p bPart) build() *dml.TablePartStyle {
	out := &dml.TablePartStyle{}
	if p.bold || p.text != nil {
		tx := &dml.TcTxStyle{SchemeClr: p.text}
		if p.bold {
			tx.B = "on"
		}
		out.TcTxStyle = tx
	}
	cell := &dml.TcStyle{}
	if p.bdr != (dml.TcBdr{}) {
		bdr := p.bdr
		cell.TcBdr = &bdr
	}
	switch {
	case p.fill != nil:
		cell.SolidFill = &dml.SolidFill{SchemeClr: p.fill}
	case p.noFill:
		cell.NoFill = &dml.NoFillXML{}
	}
	if cell.TcBdr != nil || cell.SolidFill != nil || cell.NoFill != nil {
		out.TcStyle = cell
	}
	return out
}

// bAll sets all six borders of a part to one line.
func bAll(l func() *dml.ThemeableLineStyle) dml.TcBdr {
	return dml.TcBdr{Left: l(), Right: l(), Top: l(), Bottom: l(), InsideH: l(), InsideV: l()}
}

// buildTableStyle assembles a built-in style. accent is 1 to 6, or 0 for the
// style that uses the text color instead.
func buildTableStyle(id string, family tableStyleFamily, accent int) *dml.TableStyle {
	ts := &dml.TableStyle{StyleId: id, StyleName: tableStyleName(family, accent)}
	// a is the family's color; plain styles use the dark text color.
	a := "dk1"
	if accent > 0 {
		a = "accent" + string(rune('0'+accent))
	}
	lt, dk := "lt1", "dk1"
	line := func(w int64, c *dml.SchemeClrTransform) func() *dml.ThemeableLineStyle {
		return func() *dml.ThemeableLineStyle { return bLine(w, c) }
	}
	none := func() *dml.ThemeableLineStyle { return bNone() }
	switch family {
	case themed1:
		if accent == 0 {
			// No style, no grid.
			ts.WholeTbl = bPart{text: bClr(dk), noFill: true, bdr: bAll(none)}.build()
			break
		}
		ts.TblBg = &dml.TableBgStyle{
			FillRef:   &dml.FillRef{Idx: 2, SchemeClr: bClr(a)},
			EffectRef: &dml.StyleEffectRef{Idx: 1, SchemeClr: bClr(a)},
		}
		ts.WholeTbl = bPart{text: bClr(dk), noFill: true, bdr: bAll(line(bPt1, bClr(a)))}.build()
		ts.Band1H = bPart{fill: bAlpha(a, 40000)}.build()
		ts.Band1V = bPart{fill: bAlpha(a, 40000)}.build()
		ts.FirstRow = bPart{bold: true, text: bClr(lt), fill: bClr(a), bdr: dml.TcBdr{Bottom: bLine(bPt1_5, bClr(lt))}}.build()
		ts.LastRow = bPart{bold: true, bdr: dml.TcBdr{Top: bLine(bPt1_5, bClr(a)), Bottom: bLine(bPt1_5, bClr(a)), InsideV: bNone()}}.build()
		ts.FirstCol = bPart{bold: true, bdr: dml.TcBdr{Right: bLine(bPt1_5, bClr(a))}}.build()
		ts.LastCol = bPart{bold: true, bdr: dml.TcBdr{Left: bLine(bPt1_5, bClr(a))}}.build()
	case themed2:
		if accent == 0 {
			// No style, table grid.
			ts.WholeTbl = bPart{text: bClr(dk), noFill: true, bdr: bAll(line(bPt1, bClr(dk)))}.build()
			break
		}
		ts.TblBg = &dml.TableBgStyle{
			FillRef:   &dml.FillRef{Idx: 3, SchemeClr: bClr(a)},
			EffectRef: &dml.StyleEffectRef{Idx: 3, SchemeClr: bClr(a)},
		}
		edge := bTint(a, 50000)
		ts.WholeTbl = bPart{text: bClr(lt), noFill: true, bdr: dml.TcBdr{Left: bLine(bPt1, edge), Right: bLine(bPt1, edge), Top: bLine(bPt1, edge), Bottom: bLine(bPt1, edge), InsideH: bNone(), InsideV: bNone()}}.build()
		ts.Band1H = bPart{fill: bAlpha(lt, 20000)}.build()
		ts.Band1V = bPart{fill: bAlpha(lt, 20000)}.build()
		ts.FirstRow = bPart{bold: true, bdr: dml.TcBdr{Bottom: bLine(bPt2, bClr(lt))}}.build()
		ts.LastRow = bPart{bold: true, bdr: dml.TcBdr{Top: bLine(bPt1_5, bClr(lt))}}.build()
		ts.FirstCol = bPart{bold: true, bdr: dml.TcBdr{Right: bLine(bPt1_5, bClr(lt))}}.build()
		ts.LastCol = bPart{bold: true, bdr: dml.TcBdr{Left: bLine(bPt1_5, bClr(lt))}}.build()
		// PowerPoint draws none of the header's line under the last column,
		// nor the total row's top and the column lines beside it at the
		// first and last column.
		ts.NeCell = bPart{bdr: dml.TcBdr{Bottom: bNone()}}.build()
		ts.SwCell = bPart{bdr: dml.TcBdr{Top: bNone(), Right: bNone()}}.build()
		ts.SeCell = bPart{bdr: dml.TcBdr{Top: bNone(), Left: bNone()}}.build()
	case light1:
		ts.WholeTbl = bPart{text: bClr(dk), noFill: true, bdr: dml.TcBdr{Left: bNone(), Right: bNone(), Top: bLine(bPt1, bClr(a)), Bottom: bLine(bPt1, bClr(a)), InsideH: bNone(), InsideV: bNone()}}.build()
		ts.Band1H = bPart{fill: bAlpha(a, 20000)}.build()
		ts.Band1V = bPart{fill: bAlpha(a, 20000)}.build()
		ts.FirstRow = bPart{bold: true, bdr: dml.TcBdr{Bottom: bLine(bPt1, bClr(a))}}.build()
		ts.LastRow = bPart{bold: true, bdr: dml.TcBdr{Top: bLine(bPt1, bClr(a))}}.build()
		ts.FirstCol = bPart{bold: true}.build()
		ts.LastCol = bPart{bold: true}.build()
	case light2:
		ts.WholeTbl = bPart{text: bClr(dk), noFill: true, bdr: dml.TcBdr{Left: bLine(bPt1, bClr(a)), Right: bLine(bPt1, bClr(a)), Top: bLine(bPt1, bClr(a)), Bottom: bLine(bPt1, bClr(a)), InsideH: bNone(), InsideV: bNone()}}.build()
		ts.Band1H = bPart{bdr: dml.TcBdr{Top: bLine(bPt1, bClr(a)), Bottom: bLine(bPt1, bClr(a))}}.build()
		ts.Band1V = bPart{bdr: dml.TcBdr{Left: bLine(bPt1, bClr(a)), Right: bLine(bPt1, bClr(a))}}.build()
		ts.FirstRow = bPart{bold: true, text: bClr(lt), fill: bClr(a)}.build()
		ts.LastRow = bPart{bold: true, bdr: dml.TcBdr{Top: bDouble(bPt4, bClr(a))}}.build()
		ts.FirstCol = bPart{bold: true}.build()
		ts.LastCol = bPart{bold: true}.build()
	case light3:
		ts.WholeTbl = bPart{text: bClr(dk), noFill: true, bdr: bAll(line(bPt1, bClr(a)))}.build()
		ts.Band1H = bPart{fill: bAlpha(a, 20000)}.build()
		ts.Band1V = bPart{fill: bAlpha(a, 20000)}.build()
		ts.FirstRow = bPart{bold: true, bdr: dml.TcBdr{Bottom: bLine(bPt2, bClr(a))}}.build()
		ts.LastRow = bPart{bold: true, bdr: dml.TcBdr{Top: bDouble(bPt4, bClr(a))}}.build()
		ts.FirstCol = bPart{bold: true}.build()
		ts.LastCol = bPart{bold: true}.build()
	case medium1:
		ts.WholeTbl = bPart{text: bClr(dk), fill: bClr(lt), bdr: dml.TcBdr{Left: bLine(bPt1, bClr(a)), Right: bLine(bPt1, bClr(a)), Top: bLine(bPt1, bClr(a)), Bottom: bLine(bPt1, bClr(a)), InsideH: bLine(bPt1, bClr(a)), InsideV: bNone()}}.build()
		ts.Band1H = bPart{fill: bTint(a, 20000)}.build()
		ts.Band1V = bPart{fill: bTint(a, 20000)}.build()
		ts.FirstRow = bPart{bold: true, text: bClr(lt), fill: bClr(a)}.build()
		ts.LastRow = bPart{bold: true, fill: bClr(lt), bdr: dml.TcBdr{Top: bDouble(bPt4, bClr(a))}}.build()
		ts.FirstCol = bPart{bold: true}.build()
		ts.LastCol = bPart{bold: true}.build()
	case medium2:
		ts.WholeTbl = bPart{text: bClr(dk), fill: bTint(a, 20000), bdr: bAll(line(bPt1, bClr(lt)))}.build()
		ts.Band1H = bPart{fill: bTint(a, 40000)}.build()
		ts.Band1V = bPart{fill: bTint(a, 40000)}.build()
		ts.FirstRow = bPart{bold: true, text: bClr(lt), fill: bClr(a), bdr: dml.TcBdr{Bottom: bLine(bPt3, bClr(lt))}}.build()
		ts.LastRow = bPart{bold: true, text: bClr(lt), fill: bClr(a), bdr: dml.TcBdr{Top: bLine(bPt3, bClr(lt))}}.build()
		ts.FirstCol = bPart{bold: true, text: bClr(lt), fill: bClr(a)}.build()
		ts.LastCol = bPart{bold: true, text: bClr(lt), fill: bClr(a)}.build()
	case medium3:
		ts.WholeTbl = bPart{text: bClr(dk), fill: bClr(lt), bdr: dml.TcBdr{Top: bLine(bPt2, bClr(dk)), Bottom: bLine(bPt2, bClr(dk))}}.build()
		ts.Band1H = bPart{fill: bTint(dk, 20000)}.build()
		ts.Band1V = bPart{fill: bTint(dk, 20000)}.build()
		ts.FirstRow = bPart{bold: true, text: bClr(lt), fill: bClr(a), bdr: dml.TcBdr{Bottom: bLine(bPt2, bClr(dk))}}.build()
		ts.LastRow = bPart{bold: true, text: bClr(dk), fill: bClr(lt), bdr: dml.TcBdr{Top: bDouble(bPt4, bClr(dk))}}.build()
		ts.FirstCol = bPart{bold: true, text: bClr(lt), fill: bClr(a)}.build()
		ts.LastCol = bPart{bold: true, text: bClr(lt), fill: bClr(a)}.build()
	case medium4:
		ts.WholeTbl = bPart{text: bClr(dk), fill: bTint(a, 20000), bdr: bAll(line(bPt1, bClr(a)))}.build()
		ts.Band1H = bPart{fill: bTint(a, 40000)}.build()
		ts.Band1V = bPart{fill: bTint(a, 40000)}.build()
		ts.FirstRow = bPart{bold: true}.build()
		ts.LastRow = bPart{bold: true, bdr: dml.TcBdr{Top: bLine(bPt2, bClr(a))}}.build()
		ts.FirstCol = bPart{bold: true}.build()
		ts.LastCol = bPart{bold: true}.build()
	case dark1:
		// The plain style lightens the text color in steps; the others
		// darken their accent in steps, from the accent itself.
		whole, band, col, last := bClr(a), bShade(a, 60000), bShade(a, 60000), bShade(a, 40000)
		if accent == 0 {
			whole, band, col, last = bTint(a, 20000), bTint(a, 40000), bTint(a, 60000), bTint(a, 60000)
		}
		ts.WholeTbl = bPart{text: bClr(lt), fill: whole}.build()
		ts.Band1H = bPart{fill: band}.build()
		ts.Band1V = bPart{fill: band}.build()
		// The header and total rows draw no lines between their cells, and
		// the lines beside the first and last column stop short of them.
		ts.FirstRow = bPart{bold: true, text: bClr(lt), fill: bClr(dk), bdr: dml.TcBdr{Bottom: bLine(bPt2, bClr(lt)), InsideV: bNone()}}.build()
		ts.LastRow = bPart{bold: true, fill: last, bdr: dml.TcBdr{Top: bLine(bPt2, bClr(lt)), InsideV: bNone()}}.build()
		ts.FirstCol = bPart{bold: true, fill: col, bdr: dml.TcBdr{Right: bLine(bPt2, bClr(lt))}}.build()
		ts.LastCol = bPart{bold: true, fill: col, bdr: dml.TcBdr{Left: bLine(bPt2, bClr(lt))}}.build()
	case dark2:
		head := dk
		if accent > 0 {
			head = "accent" + string(rune('0'+accent+1))
		}
		ts.WholeTbl = bPart{text: bClr(dk), fill: bTint(a, 20000)}.build()
		ts.Band1H = bPart{fill: bTint(a, 40000)}.build()
		ts.Band1V = bPart{fill: bTint(a, 40000)}.build()
		ts.FirstRow = bPart{bold: true, text: bClr(lt), fill: bClr(head)}.build()
		ts.LastRow = bPart{bold: true, bdr: dml.TcBdr{Top: bDouble(bPt4, bClr(dk))}}.build()
		ts.FirstCol = bPart{bold: true}.build()
		ts.LastCol = bPart{bold: true}.build()
	}
	return ts
}

var tableStyleFamilyNames = [...]string{"Themed Style 1", "Themed Style 2", "Light Style 1", "Light Style 2", "Light Style 3", "Medium Style 1", "Medium Style 2", "Medium Style 3", "Medium Style 4", "Dark Style 1", "Dark Style 2"}

func tableStyleName(family tableStyleFamily, accent int) string {
	name := tableStyleFamilyNames[family]
	if accent > 0 {
		name += " - Accent " + string(rune('0'+accent))
	}
	return name
}
