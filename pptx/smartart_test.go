package pptx

import (
	"bytes"
	"testing"
)

// diagramDataPart is a SmartArt data part (dgm:dataModel): a doc root with two
// top-level nodes "Alpha" and "Beta", where "Alpha" has a child "Alpha-1".
const diagramDataPart = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><dgm:ptLst><dgm:pt modelId="0" type="doc"><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></dgm:t></dgm:pt><dgm:pt modelId="1"><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Alpha</a:t></a:r></a:p></dgm:t></dgm:pt><dgm:pt modelId="2"><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Beta</a:t></a:r></a:p></dgm:t></dgm:pt><dgm:pt modelId="3"><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Alpha-1</a:t></a:r></a:p></dgm:t></dgm:pt></dgm:ptLst><dgm:cxnLst><dgm:cxn modelId="10" srcId="0" destId="1" srcOrd="0" destOrd="0"/><dgm:cxn modelId="11" srcId="0" destId="2" srcOrd="1" destOrd="0"/><dgm:cxn modelId="12" srcId="1" destId="3" srcOrd="0" destOrd="0"/></dgm:cxnLst><dgm:whole/></dgm:dataModel>`

// deckWithSmartArt returns a saved one-slide deck whose slide carries a
// p:graphicFrame referencing four diagram parts (data/layout/quickStyle/colors)
// via dgm:relIds, plus the parts, slide relationships, and content-type
// overrides that PowerPoint writes for a SmartArt graphic.
func deckWithSmartArt(t *testing.T) []byte {
	t.Helper()
	deck := savedDeck(t)
	deck = addZipParts(t, deck, map[string][]byte{
		"ppt/diagrams/data1.xml":       []byte(diagramDataPart),
		"ppt/diagrams/layout1.xml":     []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:test/layout"><dgm:title val=""/><dgm:desc val=""/><dgm:catLst/><dgm:sampData/><dgm:styleData/><dgm:clrData/><dgm:layoutNode name="root"><dgm:alg type="lin"/><dgm:shape/><dgm:presOf/></dgm:layoutNode></dgm:layoutDef>`),
		"ppt/diagrams/quickStyle1.xml": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:test/qs"><dgm:title val=""/><dgm:desc val=""/><dgm:catLst/><dgm:scene3d/><dgm:styleLbl name="node0"><dgm:scene3d/><dgm:sp3d/><dgm:txPr/><dgm:style/></dgm:styleLbl></dgm:styleDef>`),
		"ppt/diagrams/colors1.xml":     []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><dgm:colorsDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:test/colors"><dgm:title val=""/><dgm:desc val=""/><dgm:catLst/><dgm:styleLbl name="node0"><dgm:fillClrLst/><dgm:linClrLst/><dgm:effectClrLst/><dgm:txLinClrLst/><dgm:txFillClrLst/><dgm:txEffectClrLst/></dgm:styleLbl></dgm:colorsDef>`),
	})
	// Reference the diagram from the slide's shape tree.
	deck = rewriteZipPart(t, deck, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		frame := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="10" name="Diagram 1"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="838200" y="365125"/><a:ext cx="7772400" cy="4351338"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:dm="rId10" r:lo="rId11" r:qs="rId12" r:cs="rId13"/></a:graphicData></a:graphic></p:graphicFrame>`
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(frame+"</p:spTree>"), 1)
	})
	deck = rewriteZipPart(t, deck, "ppt/slides/_rels/slide1.xml.rels", func(rels []byte) []byte {
		add := `<Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="../diagrams/data1.xml"/>` +
			`<Relationship Id="rId11" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout" Target="../diagrams/layout1.xml"/>` +
			`<Relationship Id="rId12" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle" Target="../diagrams/quickStyle1.xml"/>` +
			`<Relationship Id="rId13" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors" Target="../diagrams/colors1.xml"/>`
		return bytes.Replace(rels, []byte("</Relationships>"), []byte(add+"</Relationships>"), 1)
	})
	deck = rewriteZipPart(t, deck, "[Content_Types].xml", func(ct []byte) []byte {
		add := `<Override PartName="/ppt/diagrams/data1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml"/>` +
			`<Override PartName="/ppt/diagrams/layout1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramLayout+xml"/>` +
			`<Override PartName="/ppt/diagrams/quickStyle1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramStyle+xml"/>` +
			`<Override PartName="/ppt/diagrams/colors1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramColors+xml"/>`
		return bytes.Replace(ct, []byte("</Types>"), []byte(add+"</Types>"), 1)
	})
	return deck
}

func TestSlideSmartArtRead(t *testing.T) {
	p := openBytes(t, deckWithSmartArt(t))
	slides := p.Slides()
	if len(slides) != 1 {
		t.Fatalf("Slides() = %d, want 1", len(slides))
	}
	arts := slides[0].SmartArt()
	if len(arts) != 1 {
		t.Fatalf("SmartArt() = %d, want 1", len(arts))
	}
	sa := arts[0]
	if sa.DataPartName() != "/ppt/diagrams/data1.xml" {
		t.Errorf("DataPartName() = %q, want /ppt/diagrams/data1.xml", sa.DataPartName())
	}
	nodes := sa.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("top-level nodes = %d, want 2", len(nodes))
	}
	if nodes[0].Text != "Alpha" || nodes[1].Text != "Beta" {
		t.Fatalf("top-level texts = [%q %q], want [Alpha Beta]", nodes[0].Text, nodes[1].Text)
	}
	if len(nodes[0].Children) != 1 || nodes[0].Children[0].Text != "Alpha-1" {
		t.Fatalf("Alpha children = %+v, want [Alpha-1]", nodes[0].Children)
	}

	// Presentation-level accessor sees the same graphic.
	if got := len(p.SmartArt()); got != 1 {
		t.Fatalf("Presentation.SmartArt() = %d, want 1", got)
	}
}

// TestSlideSmartArtPreserved verifies the diagram parts survive a round trip
// byte-for-byte (they are preserved verbatim, not regenerated).
func TestSlideSmartArtPreserved(t *testing.T) {
	src := deckWithSmartArt(t)
	p := openBytes(t, src)
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"ppt/diagrams/data1.xml",
		"ppt/diagrams/layout1.xml",
		"ppt/diagrams/quickStyle1.xml",
		"ppt/diagrams/colors1.xml",
	} {
		before := zipPart(t, src, name)
		after := zipPart(t, out, name)
		if !bytes.Equal(before, after) {
			t.Errorf("%s not preserved byte-for-byte:\n before: %s\n after:  %s", name, before, after)
		}
	}

	// The re-opened deck still reads the SmartArt.
	if got := len(openBytes(t, out).SmartArt()); got != 1 {
		t.Fatalf("SmartArt() after round trip = %d, want 1", got)
	}
}

// TestSlideSmartArtNoneWhenNoDiagram confirms plain shapes yield no SmartArt.
func TestSlideSmartArtNoneWhenNoDiagram(t *testing.T) {
	p := openBytes(t, savedDeck(t))
	if got := p.Slides()[0].SmartArt(); got != nil {
		t.Fatalf("SmartArt() on plain slide = %v, want nil", got)
	}
}
