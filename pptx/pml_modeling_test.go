package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// slideModelExtLst is a p:extLst in the byte form the marshaler emits.
const slideModelExtLst = `<p:extLst><p:ext uri="{FFDD11EE-0000-0000-0000-000000000002}"` +
	` xmlns:foo="urn:example:foo"><foo:mark val="2"/></p:ext></p:extLst>`

// slideModelCustData and slideModelControls are cSld children in the byte
// form the raw-capture path re-emits (C33: custDataLst and ActiveX controls
// were deleted on every save).
const slideModelCustData = `<p:custDataLst><p:custData r:id="rId7"/></p:custDataLst>`
const slideModelControls = `<p:controls><p:control spid="_x0000_s1026" r:id="rId8" imgW="2540" imgH="2540"/></p:controls>`

// slideModelCxnSp is a connector carrying an extLst.
const slideModelCxnSp = `<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="30" name="Conn"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>` +
	`<p:spPr/>` + slideModelExtLst + `</p:cxnSp>`

// deckWithSlideModelingGaps builds a deck via the API and splices in every
// C33 construct: slide-level showMaster* attributes, cSld custDataLst and
// controls, sp useBgFill+extLst, a connector with extLst, and grpSpPr fills.
func deckWithSlideModelingGaps(t *testing.T) []byte {
	t.Helper()
	p := Create()
	slide := p.AddSlide()
	box := slide.AddTextBox()
	box.TextFrame().SetText("keeper")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		xml = bytes.Replace(xml,
			[]byte(`presentationml/2006/main">`),
			[]byte(`presentationml/2006/main" showMasterSp="0" showMasterPhAnim="0">`), 1)
		xml = bytes.Replace(xml, []byte(`<p:sp>`), []byte(`<p:sp useBgFill="1">`), 1)
		xml = bytes.Replace(xml, []byte(`</p:sp>`), []byte(slideModelExtLst+`</p:sp>`), 1)
		xml = bytes.Replace(xml, []byte(`</a:xfrm></p:grpSpPr>`),
			[]byte(`</a:xfrm><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:effectLst/></p:grpSpPr>`), 1)
		xml = bytes.Replace(xml, []byte(`</p:spTree>`),
			[]byte(slideModelCxnSp+`</p:spTree>`+slideModelCustData+slideModelControls), 1)
		return xml
	})
}

// C33: a zero-modification Open+Save keeps slide showMaster* attributes,
// custDataLst, ActiveX controls, useBgFill, shape/connector extensions, and
// group-shape-property fills, all byte-faithfully.
func TestZeroModSavePreservesSlideModelingGaps(t *testing.T) {
	deck := deckWithSlideModelingGaps(t)

	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	for _, want := range []string{
		`showMasterSp="0" showMasterPhAnim="0"><p:cSld>`,
		slideModelCustData,
		slideModelControls,
		`<p:sp useBgFill="1">`,
		slideModelExtLst,
		slideModelCxnSp,
		`</a:xfrm><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:effectLst/></p:grpSpPr>`,
		"keeper",
	} {
		if !strings.Contains(slideXML, want) {
			t.Errorf("slide1.xml lost %q:\n%s", want, slideXML)
		}
	}
}

// slideModelTiming is a p:timing subtree exercising the C34 shapes: graphicEl
// with a:chart and a:dgm (DrawingML namespace), progress as CT_TLAnimVariant,
// and bldSub with a:bldChart/a:bldDgm.
const slideModelTiming = `<p:timing>` +
	`<p:tnLst><p:par>` +
	`<p:cTn id="1" dur="indefinite" restart="never" nodeType="tmRoot">` +
	`<p:childTnLst>` +
	`<p:animEffect transition="in" filter="fade">` +
	`<p:cBhvr><p:cTn id="2" dur="500"/>` +
	`<p:tgtEl><p:spTgt spid="4"><p:graphicEl><a:chart seriesIdx="0" categoryIdx="2" bldStep="series"/></p:graphicEl></p:spTgt></p:tgtEl>` +
	`</p:cBhvr>` +
	`<p:progress><p:fltVal val="0.5"/></p:progress>` +
	`</p:animEffect>` +
	`<p:animEffect transition="out">` +
	`<p:cBhvr><p:cTn id="3"/>` +
	`<p:tgtEl><p:spTgt spid="5"><p:graphicEl><a:dgm id="{9B21C3C5-3B29-4004-8908-573A0F6BD9F5}" bldStep="bg"/></p:graphicEl></p:spTgt></p:tgtEl>` +
	`</p:cBhvr>` +
	`</p:animEffect>` +
	`</p:childTnLst>` +
	`</p:cTn>` +
	`</p:par></p:tnLst>` +
	`<p:bldLst>` +
	`<p:bldGraphic spid="4" grpId="0"><p:bldSub><a:bldChart bld="category" animBg="0"/></p:bldSub></p:bldGraphic>` +
	`<p:bldGraphic spid="5" grpId="1"><p:bldSub><a:bldDgm bld="lvlOne" rev="1"/></p:bldSub></p:bldGraphic>` +
	`</p:bldLst>` +
	`</p:timing>`

// C34: chart/diagram animation targets and builds survive a zero-modification
// Open+Save byte-faithfully, with children in the DrawingML namespace.
func TestZeroModSavePreservesTimingAnimationModel(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	box := slide.AddTextBox()
	box.TextFrame().SetText("keeper")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	deck := rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte(`</p:sld>`), []byte(slideModelTiming+`</p:sld>`), 1)
	})

	opened, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := opened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if !strings.Contains(slideXML, slideModelTiming) {
		t.Errorf("timing subtree not preserved byte-faithfully:\n%s", slideXML)
	}
}

// presModelTextStyle is a defaultTextStyle exercising the C91 gaps: an
// srgbClr solid fill, schemeClr tint/shade transform children, line spacing,
// bullet color/char, and tab stops.
const presModelTextStyle = `<p:defaultTextStyle>` +
	`<a:defPPr><a:defRPr lang="en-US"/></a:defPPr>` +
	`<a:lvl1pPr marL="0" algn="l">` +
	`<a:lnSpc><a:spcPct val="150000"/></a:lnSpc>` +
	`<a:spcBef><a:spcPts val="600"/></a:spcBef>` +
	`<a:buClr><a:schemeClr val="accent1"><a:tint val="75000"/><a:shade val="25000"/></a:schemeClr></a:buClr>` +
	`<a:buChar char="-"/>` +
	`<a:tabLst><a:tab pos="914400" algn="l"/></a:tabLst>` +
	`<a:defRPr sz="1800"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:defRPr>` +
	`</a:lvl1pPr>` +
	`</p:defaultTextStyle>`

// C91: the presentation.xml writer no longer drops srgbClr solid fills,
// schemeClr tint/shade transforms, or bullet/spacing/tab paragraph children
// from a parsed defaultTextStyle (presentation.xml is regenerated on every
// save, so any writer gap is live data loss).
func TestDefaultTextStyleFullFidelityRoundTrip(t *testing.T) {
	deck := deckWithPresentationXMLRewrite(t, func(xml []byte) []byte {
		start := bytes.Index(xml, []byte(`<p:defaultTextStyle>`))
		end := bytes.Index(xml, []byte(`</p:defaultTextStyle>`))
		if start < 0 || end < 0 {
			t.Fatal("created deck has no defaultTextStyle")
		}
		var out []byte
		out = append(out, xml[:start]...)
		out = append(out, presModelTextStyle...)
		out = append(out, xml[end+len(`</p:defaultTextStyle>`):]...)
		return out
	})

	presXML := openAndResave(t, deck)
	if !strings.Contains(presXML, presModelTextStyle) {
		t.Errorf("defaultTextStyle not preserved byte-faithfully:\n%s", presXML)
	}
}

// tableModelGF is a table graphic frame exercising the corpus class F2 gaps:
// graphicFrameLocks with the full attribute set, and a tcPr with vert /
// anchorCtr / horzOverflow attributes plus lnTlToBr / lnBlToTr / headers
// children — all of which were previously dropped by the typed model.
const tableModelGF = `<p:graphicFrame>` +
	`<p:nvGraphicFramePr><p:cNvPr id="20" name="Table 19"/>` +
	`<p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1" noSelect="1" noChangeAspect="1" noMove="1" noResize="1"/></p:cNvGraphicFramePr>` +
	`<p:nvPr/></p:nvGraphicFramePr>` +
	`<p:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></p:xfrm>` +
	`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">` +
	`<a:tbl><a:tblPr rtl="1" firstRow="1" bandRow="1"><a:tableStyleId>{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}</a:tableStyleId></a:tblPr>` +
	`<a:tblGrid><a:gridCol w="1000"/></a:tblGrid>` +
	`<a:tr h="370840"><a:tc>` +
	`<a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>cell</a:t></a:r></a:p></a:txBody>` +
	`<a:tcPr marL="0" vert="vert270" anchor="ctr" anchorCtr="1" horzOverflow="overflow">` +
	`<a:lnTlToBr w="12700"><a:solidFill><a:srgbClr val="FF00FF"/></a:solidFill></a:lnTlToBr>` +
	`<a:lnBlToTr w="12700"><a:noFill/></a:lnBlToTr>` +
	`<a:headers><a:header>hdr-a</a:header><a:header>hdr-b</a:header></a:headers>` +
	`</a:tcPr>` +
	`</a:tc></a:tr></a:tbl>` +
	`</a:graphicData></a:graphic></p:graphicFrame>`

// nvPrCustData is a shape nvPr carrying a customer-data tags reference
// (p:custDataLst), previously deleted because NvPr had no field for it.
const nvPrCustData = `<p:nvPr><p:custDataLst><p:custData r:id="rId7"/><p:tags r:id="rId8"/></p:custDataLst></p:nvPr>`

// hlinkWithExtLst is an a:hlinkClick carrying an extLst (ahyp:hlinkClr),
// previously collapsed to a self-closing hlinkClick.
const hlinkWithExtLst = `<a:hlinkClick r:id="rId9">` +
	`<a:extLst><a:ext uri="{A12FA001-9CCC-4127-97F1-B205AA2FDE22}">` +
	`<ahyp:hlinkClr xmlns:ahyp="http://schemas.microsoft.com/office/drawing/2018/hyperlinkcolor" val="tx"/>` +
	`</a:ext></a:extLst></a:hlinkClick>`

// Corpus class F2: typed pml/dml structs were missing XSD fields and dropped
// real attributes and children on a zero-modification save: tcPr overflow and
// diagonal borders, table-cell headers, graphicFrameLocks beyond noGrp, nvPr
// custDataLst, and hlinkClick extLst.
func TestZeroModSavePreservesModelGapFields(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	slide.AddTextBox().TextFrame().SetText("keeper")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	deck := rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		xml = bytes.Replace(xml, []byte(`<p:nvPr/>`), []byte(nvPrCustData), 1)
		if i := bytes.Index(xml, []byte(`name="TextBox`)); i >= 0 {
			// Give the textbox cNvPr a hyperlink with an extLst.
			end := bytes.Index(xml[i:], []byte(`/>`))
			xml = append(xml[:i+end], append([]byte(`>`+hlinkWithExtLst+`</p:cNvPr>`), xml[i+end+2:]...)...)
		}
		return bytes.Replace(xml, []byte(`</p:spTree>`), []byte(tableModelGF+`</p:spTree>`), 1)
	})

	opened, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := opened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	for _, want := range []string{
		`<a:graphicFrameLocks noGrp="1" noSelect="1" noChangeAspect="1" noMove="1" noResize="1"/>`,
		`<a:tblPr rtl="1" firstRow="1" bandRow="1">`,
		`<a:tcPr marL="0" vert="vert270" anchor="ctr" anchorCtr="1" horzOverflow="overflow">`,
		`<a:lnTlToBr w="12700"><a:solidFill><a:srgbClr val="FF00FF"/></a:solidFill></a:lnTlToBr>`,
		`<a:lnBlToTr w="12700"><a:noFill/></a:lnBlToTr>`,
		`<a:headers><a:header>hdr-a</a:header><a:header>hdr-b</a:header></a:headers>`,
		nvPrCustData,
		hlinkWithExtLst,
		"keeper",
	} {
		if !strings.Contains(slideXML, want) {
			t.Errorf("slide1.xml lost %q:\n%s", want, slideXML)
		}
	}
}
