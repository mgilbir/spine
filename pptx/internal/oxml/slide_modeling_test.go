package oxml

import (
	"encoding/xml"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// roundTripSlideBytes unmarshals a full p:sld document and re-marshals it
// through the production Builder, requiring byte-identical output.
func roundTripSlideBytes(t *testing.T, src string) {
	t.Helper()
	var sld Slide
	if err := xml.Unmarshal([]byte(src), &sld); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b := xmlb.NewPresentationMLBuilder()
	sld.MarshalRootToBuilder(b)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	if got := b.String(); got != src {
		t.Errorf("round-trip drift:\n got: %s\nwant: %s", got, src)
	}
}

const sldOpen = `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
	` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
	` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

// modelingExtLst is a p:extLst in the byte form ExtensionList emits.
const modelingExtLst = `<p:extLst><p:ext uri="{FFDD11EE-0000-0000-0000-000000000001}"` +
	` xmlns:foo="urn:example:foo"><foo:mark val="1"/></p:ext></p:extLst>`

// C33: slide showMasterSp/showMasterPhAnim attributes, cSld custDataLst and
// controls (ActiveX), sp useBgFill+extLst, pic/cxnSp extLst, and grpSpPr
// fills/effects all round-trip byte-faithfully.
func TestSlideModelingGaps_RoundTrip(t *testing.T) {
	src := sldOpen + ` showMasterSp="0" showMasterPhAnim="0">` +
		`<p:cSld>` +
		`<p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr bwMode="auto">` +
		`<a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm>` +
		`<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>` +
		`<a:effectLst/>` +
		`</p:grpSpPr>` +
		`<p:sp useBgFill="1">` +
		`<p:nvSpPr><p:cNvPr id="2" name="Box"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>` +
		`<p:spPr/>` +
		modelingExtLst +
		`</p:sp>` +
		`<p:pic>` +
		`<p:nvPicPr><p:cNvPr id="3" name="Pic"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>` +
		`<p:blipFill><a:blip r:embed="rId3"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>` +
		`<p:spPr/>` +
		modelingExtLst +
		`</p:pic>` +
		`<p:cxnSp>` +
		`<p:nvCxnSpPr><p:cNvPr id="4" name="Conn"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>` +
		`<p:spPr/>` +
		modelingExtLst +
		`</p:cxnSp>` +
		`</p:spTree>` +
		`<p:custDataLst><p:custData r:id="rId7"/><p:tags r:id="rId8"/></p:custDataLst>` +
		`<p:controls><p:control spid="_x0000_s1026" r:id="rId9" imgW="2540" imgH="2540"/></p:controls>` +
		`</p:cSld>` +
		`</p:sld>`
	roundTripSlideBytes(t, src)
}

// C33: a group shape's grpSpPr keeps its fill group, effects, scene3d, and
// extLst instead of being reduced to bwMode+xfrm.
func TestGroupShapePropertiesFills_RoundTrip(t *testing.T) {
	src := sldOpen + `>` +
		`<p:cSld><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr/>` +
		`<p:grpSp>` +
		`<p:nvGrpSpPr><p:cNvPr id="10" name="Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr>` +
		`<a:gradFill><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="accent1"><a:tint val="50000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="1"/></a:gradFill>` +
		`<a:effectLst><a:outerShdw blurRad="40000" dist="20000" dir="5400000"><a:srgbClr val="000000"><a:alpha val="38000"/></a:srgbClr></a:outerShdw></a:effectLst>` +
		`<a:scene3d><a:camera prst="orthographicFront"/><a:lightRig rig="threePt" dir="t"/></a:scene3d>` +
		`</p:grpSpPr>` +
		`</p:grpSp>` +
		`</p:spTree></p:cSld>` +
		`</p:sld>`
	roundTripSlideBytes(t, src)
}
