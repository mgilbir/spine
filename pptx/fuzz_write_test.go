package pptx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/internal/fuzzseed"
)

// fuzzFill maps a selector byte to one of the fill kinds a background or shape
// accepts, exercising the solid/gradient/pattern/none routing.
func fuzzFill(sel uint8, hex string) dml.Fill {
	rgb, err := dml.ParseRGB(hex)
	if err != nil {
		rgb = dml.NewRGB(uint8(sel), uint8(sel*3), uint8(sel*7))
	}
	c := dml.Color{Type: dml.ColorTypeRGB, RGB: rgb}
	switch sel % 4 {
	case 0:
		return dml.NewSolidFill(c)
	case 1:
		return dml.NewGradientFill(float64(sel)*13,
			dml.GradientStop{Position: 0, Color: c},
			dml.GradientStop{Position: 1, Color: dml.Color{Type: dml.ColorTypeRGB}})
	case 2:
		return dml.NewPatternFill("pct50", c, dml.Color{Type: dml.ColorTypeRGB})
	default:
		return dml.NewNoFill()
	}
}

// FuzzPptxAddConnector fuzzes Slide.AddConnector plus SetPoints, the shape
// bindings (Connect/SetStartShape/SetEndShape), and the line styling setters.
// The four fuzzed EMU coordinates place a free connector; connection-site
// indexes and the routing kind are fuzzed too. It saves and re-opens, reading
// the connectors back. No panic; a self-consistent read-back.
func FuzzPptxAddConnector(f *testing.F) {
	f.Add(int8(0), int64(0), int64(0), int64(914400), int64(914400), uint32(0), uint32(1), 2.5, false)
	f.Add(int8(1), int64(-99999999999), int64(5), int64(0), int64(-7), uint32(999999), uint32(0), 0.0, true)
	f.Add(int8(2), int64(1), int64(1), int64(1), int64(1), uint32(2), uint32(2), 99999.0, false)
	f.Add(int8(9), int64(9223372036854775807), int64(0), int64(0), int64(0), uint32(0), uint32(0), -3.0, true)

	f.Fuzz(func(t *testing.T, kindSel int8, x1, y1, x2, y2 int64, startSite, endSite uint32, width float64, bind bool) {
		p := Create()
		slide := p.AddSlide()
		a := slide.AddTextBox()
		a.TextFrame().SetText("A")
		b := slide.AddTextBox()
		b.TextFrame().SetText("B")

		kind := ConnectorKind(int(kindSel) % 3)
		c := slide.AddConnector(kind)
		c.SetPoints(dml.EMU(x1), dml.EMU(y1), dml.EMU(x2), dml.EMU(y2))
		if bind {
			c.Connect(a, startSite, b, endSite)
		} else {
			c.SetStartShape(a, startSite)
		}
		c.SetKind(ConnectorKind(int(kindSel+1) % 3))
		c.SetLineWidth(width)
		c.SetLineColor(dml.Color{Type: dml.ColorTypeRGB, RGB: dml.NewRGB(uint8(startSite), uint8(endSite), 0)})
		c.SetLineDash(dml.DashDashDot)
		_, _, _ = c.StartConnection()
		_, _, _ = c.EndConnection()

		fuzzReparsePptx(p)
	})
}

// FuzzPptxMasterLayoutSetters fuzzes the slide-master and layout write APIs:
// SetBackgroundFill on the master/layout, the master text-style level setters
// (font/size/bold/italic/color/bullet across in- and out-of-range levels), and
// the editable placeholders' SetPosition/SetSize. It saves and re-opens. No
// panic; a self-consistent read-back.
func FuzzPptxMasterLayoutSetters(f *testing.F) {
	f.Add(int8(0), "376092", int8(1), "Arial", 18.0, true, int64(100), int64(200))
	f.Add(int8(3), "", int8(-1), "", 0.0, false, int64(0), int64(0))
	f.Add(int8(1), "zzz", int8(20), "\x00bad", 1e9, true, int64(-5), int64(9999999999))
	f.Add(int8(2), "FFAABBCC", int8(8), "Font", -3.0, false, int64(914400), int64(914400))

	f.Fuzz(func(t *testing.T, fillSel int8, color string, level int8, font string, size float64, flag bool, px, py int64) {
		p := Create()
		p.AddSlide()
		masters := p.SlideMasters()
		if len(masters) == 0 {
			return
		}
		m := masters[0]
		fill := fuzzFill(uint8(fillSel), color)
		m.SetBackgroundFill(fill)

		lvl := int(level)
		for _, ts := range []*MasterTextStyle{m.TitleStyle(), m.BodyStyle(), m.OtherStyle()} {
			ts.SetLevelFont(lvl, font)
			ts.SetLevelFontSize(lvl, size)
			ts.SetLevelBold(lvl, flag)
			ts.SetLevelItalic(lvl, !flag)
			ts.SetLevelColor(lvl, dml.Color{Type: dml.ColorTypeRGB, RGB: dml.NewRGB(uint8(level), 0, uint8(fillSel))})
			ts.SetLevelBullet(lvl, BulletType(int(level)%5))
			ts.SetLevelBulletChar(lvl, font)
			_ = ts.Level(lvl)
		}

		for _, ep := range m.EditablePlaceholders() {
			ep.SetPosition(dml.EMU(px), dml.EMU(py))
			ep.SetSize(dml.EMU(px), dml.EMU(py))
			_, _, _ = ep.Position()
			_, _, _ = ep.Size()
		}
		for _, l := range m.Layouts() {
			l.SetBackgroundFill(fill)
			for _, ep := range l.EditablePlaceholders() {
				ep.SetPosition(dml.EMU(px), dml.EMU(py))
				ep.SetSize(dml.EMU(px), dml.EMU(py))
			}
		}

		fuzzReparsePptx(p)
	})
}

// diagramScaffoldDeck wraps a valid presentation seed with the parts, slide
// shape-tree frame, relationships, and content-type overrides PowerPoint writes
// for a SmartArt graphic, leaving a placeholder data part that the fuzzer
// replaces. It mirrors the deckWithSmartArt test helper but works on raw bytes.
func diagramScaffoldDeck(valid []byte) []byte {
	const slidePart = "ppt/slides/slide1.xml"
	const relsPart = "ppt/slides/_rels/slide1.xml.rels"
	const ctPart = "[Content_Types].xml"

	slide := fuzzseed.ZipEntry(valid, slidePart)
	rels := fuzzseed.ZipEntry(valid, relsPart)
	ct := fuzzseed.ZipEntry(valid, ctPart)
	if slide == nil || rels == nil || ct == nil {
		return nil
	}

	frame := []byte(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="10" name="Diagram 1"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="838200" y="365125"/><a:ext cx="7772400" cy="4351338"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:dm="rId10" r:lo="rId11" r:qs="rId12" r:cs="rId13"/></a:graphicData></a:graphic></p:graphicFrame>`)
	slide = bytes.Replace(slide, []byte("</p:spTree>"), append(frame, []byte("</p:spTree>")...), 1)

	addRels := []byte(`<Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="../diagrams/data1.xml"/>` +
		`<Relationship Id="rId11" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout" Target="../diagrams/layout1.xml"/>` +
		`<Relationship Id="rId12" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle" Target="../diagrams/quickStyle1.xml"/>` +
		`<Relationship Id="rId13" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors" Target="../diagrams/colors1.xml"/>`)
	rels = bytes.Replace(rels, []byte("</Relationships>"), append(addRels, []byte("</Relationships>")...), 1)

	addCT := []byte(`<Override PartName="/ppt/diagrams/data1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml"/>` +
		`<Override PartName="/ppt/diagrams/layout1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramLayout+xml"/>` +
		`<Override PartName="/ppt/diagrams/quickStyle1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramStyle+xml"/>` +
		`<Override PartName="/ppt/diagrams/colors1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramColors+xml"/>`)
	ct = bytes.Replace(ct, []byte("</Types>"), append(addCT, []byte("</Types>")...), 1)

	layout := `<?xml version="1.0"?><dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:test/layout"><dgm:layoutNode name="root"><dgm:alg type="lin"/><dgm:shape/><dgm:presOf/></dgm:layoutNode></dgm:layoutDef>`
	quick := `<?xml version="1.0"?><dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:test/qs"><dgm:styleLbl name="node0"><dgm:style/></dgm:styleLbl></dgm:styleDef>`
	colors := `<?xml version="1.0"?><dgm:colorsDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:test/colors"><dgm:styleLbl name="node0"><dgm:fillClrLst/></dgm:styleLbl></dgm:colorsDef>`

	return fuzzseed.EditZip(valid, [][2]string{
		{slidePart, string(slide)},
		{relsPart, string(rels)},
		{ctPart, string(ct)},
		{"ppt/diagrams/data1.xml", "<dgm:dataModel xmlns:dgm=\"http://schemas.openxmlformats.org/drawingml/2006/diagram\"/>"},
		{"ppt/diagrams/layout1.xml", layout},
		{"ppt/diagrams/quickStyle1.xml", quick},
		{"ppt/diagrams/colors1.xml", colors},
	})
}

// FuzzPptxSmartArtData feeds arbitrary bytes to the SmartArt diagram data part
// of an otherwise-valid deck, driving the dgm:dataModel parser through
// Slide.SmartArt and SmartArt.Nodes. On a successful parse it reads the node
// tree back and round-trips the deck. Any panic is a bug; parse errors are
// expected and fine.
func FuzzPptxSmartArtData(f *testing.F) {
	valid := buildValidPptxFuzzSeed(f)
	scaffold := diagramScaffoldDeck(valid)
	if scaffold == nil {
		f.Fatal("could not scaffold a SmartArt deck from the valid seed")
	}
	const dataPart = "ppt/diagrams/data1.xml"

	dm := `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dgm:ptLst><dgm:pt modelId="0" type="doc"/><dgm:pt modelId="1"><dgm:t><a:p><a:r><a:t>Alpha</a:t></a:r></a:p></dgm:t></dgm:pt></dgm:ptLst><dgm:cxnLst><dgm:cxn modelId="9" srcId="0" destId="1" srcOrd="0"/></dgm:cxnLst></dgm:dataModel>`
	f.Add([]byte(dm))
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add([]byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:ptLst/></dgm:dataModel>`))
	// A cycle in the parent-of connections (guarded by the parser's visited set).
	f.Add([]byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:ptLst><dgm:pt modelId="1"/><dgm:pt modelId="2"/></dgm:ptLst><dgm:cxnLst><dgm:cxn srcId="1" destId="2"/><dgm:cxn srcId="2" destId="1"/></dgm:cxnLst></dgm:dataModel>`))
	// Connections referencing points that do not exist.
	f.Add([]byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:ptLst><dgm:pt modelId="doc" type="doc"/></dgm:ptLst><dgm:cxnLst><dgm:cxn srcId="x" destId="y" srcOrd="99999999999"/></dgm:cxnLst></dgm:dataModel>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		wrapped := fuzzseed.ReplaceZipEntry(scaffold, dataPart, data)
		if wrapped == nil {
			t.Skip("scaffold unreadable")
		}
		p, err := OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if err != nil {
			return
		}
		defer func() { _ = p.Close() }()

		var walk func(nodes []*SmartArtNode)
		walk = func(nodes []*SmartArtNode) {
			for _, n := range nodes {
				if n == nil {
					continue
				}
				_ = n.Text
				walk(n.Children)
			}
		}
		for _, sa := range p.SmartArt() {
			_ = sa.DataPartName()
			walk(sa.Nodes())
		}
		out, err := p.SaveBytes()
		if err != nil {
			return
		}
		if p2, err := OpenReader(bytes.NewReader(out), int64(len(out))); err == nil {
			_ = p2.SmartArt()
			_ = p2.Close()
		}
	})
}
