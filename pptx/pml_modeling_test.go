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
