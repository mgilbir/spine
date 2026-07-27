package oxml

import (
	"regexp"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// This file holds Builder-path byte-identity coverage for the PresentationML
// model: the per-type *_RoundTrip tests elsewhere in the package go through the
// stdlib encoder and assert only that the output re-unmarshals, which cannot
// observe a dropped attribute, a duplicated namespace declaration or a deleted
// child. Everything here re-marshals through the production Builder and
// compares bytes.

const (
	nsPDecl   = ` xmlns:p="` + xmlb.NSPresentationML + `"`
	nsADecl   = ` xmlns:a="` + xmlb.NSDrawingML + `"`
	nsRDecl   = ` xmlns:r="` + xmlb.NSOfficeDocumentRels + `"`
	nsMCDecl  = ` xmlns:mc="` + xmlb.NSMarkupCompatibility + `"`
	nsP14Decl = ` xmlns:p14="` + xmlb.NSPowerPoint2010 + `"`
	nsP15Decl = ` xmlns:p15="` + xmlb.NSPowerPoint2012 + `"`
)

// marshalToString renders v through the production Builder as ns:localName.
func marshalToString(t *testing.T, ns, localName string, v any) string {
	t.Helper()
	b := xmlb.NewPresentationMLBuilder()
	b.SetCollapseEmptyElements(true)
	b.MarshalElement(ns, localName, v)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	return b.String()
}

// roundTripPML unmarshals src into dst (with the source registered so the
// capture kit is armed), re-marshals it through the Builder and returns the
// bytes.
func roundTripPML(t *testing.T, src, ns, localName string, dst any) string {
	t.Helper()
	if err := xmlb.UnmarshalWithSource([]byte(src), dst); err != nil {
		t.Fatalf("unmarshal %s: %v", localName, err)
	}
	return marshalToString(t, ns, localName, dst)
}

// declRE matches an xmlns declaration together with its leading space.
var declRE = regexp.MustCompile(` xmlns(:[A-Za-z0-9_.-]+)?="[^"]*"`)

// wantSameAsSource strips xmlns declarations. A test fragment must declare the
// prefixes it uses to parse standalone, while the Builder emits declarations
// where the enclosing part would have them — a placement difference that says
// nothing about the property under test. Everything else (attribute presence,
// value, order, child structure, empty-tag form) is compared byte for byte.
// Declaration placement and duplication have their own tests (C523, C529).
func wantSameAsSource(src string) string {
	return declRE.ReplaceAllString(src, "")
}

// assertRoundTrip re-marshals src through the Builder and requires the result
// to match src byte for byte modulo xmlns declaration placement.
func assertRoundTrip(t *testing.T, src, ns, localName string, dst any) {
	t.Helper()
	got := roundTripPML(t, src, ns, localName, dst)
	if want, gotN := wantSameAsSource(src), wantSameAsSource(got); gotN != want {
		t.Errorf("re-marshal is not byte-identical\n want: %s\n  got: %s", want, gotN)
	}
}

// --- C420: explicit-zero attributes on the always-remarshal paths ---------

// Every entry is a self-contained element whose source spells an XSD
// default-valued attribute explicitly. All of them sit on a path that is
// re-marshaled on every save (slide transitions and timing, master and layout
// transitions and timing, the shape tree, the comments parts), so dropping the
// attribute is a silent byte-drift — and for spokes/numSld/lvl a silent
// semantic change, since the schema default is not the Go zero value.
func TestC420_ExplicitDefaultAttributesSurviveRemarshal(t *testing.T) {
	cases := []struct {
		name  string
		local string
		src   string
		newV  func() any
	}{
		{"cut thruBlk=0", "cut",
			`<p:cut` + nsPDecl + ` thruBlk="0"/>`,
			func() any { return &OptionalBlackTransition{} }},
		{"fade thruBlk=false", "fade",
			`<p:fade` + nsPDecl + ` thruBlk="false"/>`,
			func() any { return &OptionalBlackTransition{} }},
		{"stSnd loop=0", "stSnd",
			`<p:stSnd` + nsPDecl + ` loop="0"/>`,
			func() any { return &TransitionStartSoundAction{} }},
		{"wheel spokes=0", "wheel",
			`<p:wheel` + nsPDecl + ` spokes="0"/>`,
			func() any { return &WheelTransition{} }},
		{"blinds dir=horz", "blinds",
			`<p:blinds` + nsPDecl + ` dir="horz"/>`,
			func() any { return &OrientationTransition{} }},
		{"push dir=l", "push",
			`<p:push` + nsPDecl + ` dir="l"/>`,
			func() any { return &SideDirectionTransition{} }},
		{"strips dir=lu", "strips",
			`<p:strips` + nsPDecl + ` dir="lu"/>`,
			func() any { return &CornerDirectionTransition{} }},
		{"cover dir=l", "cover",
			`<p:cover` + nsPDecl + ` dir="l"/>`,
			func() any { return &EightDirectionTransition{} }},
		{"split orient+dir", "split",
			`<p:split` + nsPDecl + ` orient="horz" dir="out"/>`,
			func() any { return &SplitTransition{} }},
		{"zoom dir=out", "zoom",
			`<p:zoom` + nsPDecl + ` dir="out"/>`,
			func() any { return &InOutTransition{} }},
		{"iterate backwards=0", "iterate",
			`<p:iterate` + nsPDecl + ` type="el" backwards="0"/>`,
			func() any { return &Iterate{} }},
		{"audio isNarration=0", "audio",
			`<p:audio` + nsPDecl + ` isNarration="0"/>`,
			func() any { return &Audio{} }},
		{"video fullScrn=0", "video",
			`<p:video` + nsPDecl + ` fullScrn="0"/>`,
			func() any { return &Video{} }},
		{"cMediaNode mute+numSld=0", "cMediaNode",
			`<p:cMediaNode` + nsPDecl + ` vol="50000" mute="0" numSld="0"/>`,
			func() any { return &CommonMediaNode{} }},
		{"bldDgm uiExpand+rev=0", "bldDgm",
			`<p:bldDgm` + nsPDecl + ` spid="4" grpId="0" uiExpand="0" rev="0"/>`,
			func() any { return &BuildDiagram{} }},
		{"bldOleChart uiExpand=0", "bldOleChart",
			`<p:bldOleChart` + nsPDecl + ` spid="5" grpId="0" uiExpand="0"/>`,
			func() any { return &BuildOleChart{} }},
		{"bldGraphic uiExpand=0", "bldGraphic",
			`<p:bldGraphic` + nsPDecl + ` spid="6" grpId="0" uiExpand="0"/>`,
			func() any { return &BuildGraphic{} }},
		{"a:bldDgm rev=0", "bldDgm",
			`<p:bldDgm` + nsPDecl + ` bld="one" rev="0"/>`,
			func() any { return &AnimationDgmBuildProperties{} }},
		{"oleChartEl lvl=0", "oleChartEl",
			`<p:oleChartEl` + nsPDecl + ` type="embed" lvl="0"/>`,
			func() any { return &OleChartElement{} }},
		{"tmpl lvl=0", "tmpl",
			`<p:tmpl` + nsPDecl + ` lvl="0"/>`,
			func() any { return &Template{} }},
		{"sp useBgFill=0", "sp",
			`<p:sp` + nsPDecl + nsADecl + ` useBgFill="0"><p:nvSpPr><p:cNvPr id="2" name="x"/>` +
				`<p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr/></p:sp>`,
			func() any { return &Shape{} }},
		{"nvPr isPhoto+userDrawn=0", "nvPr",
			`<p:nvPr` + nsPDecl + ` isPhoto="0" userDrawn="0"/>`,
			func() any { return &NvPr{} }},
		{"cmAuthor clrIdx+lastIdx=0", "cmAuthor",
			`<p:cmAuthor` + nsPDecl + ` id="1" name="A" initials="A" lastIdx="0" clrIdx="0"/>`,
			func() any { return &CommentAuthor{} }},
		{"cm attr order", "cm",
			`<p:cm` + nsPDecl + ` idx="1" authorId="2" dt="2024-01-01T00:00:00"><p:pos x="1" y="2"/></p:cm>`,
			func() any { return &Comment{} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertRoundTrip(t, tc.src, xmlb.NSPresentationML, tc.local, tc.newV())
		})
	}
}

// A modeled edit still wins over the captured value: the capture must not turn
// the attribute read-only.
func TestC420_ModeledEditOverridesCapturedValue(t *testing.T) {
	var tr OptionalBlackTransition
	if err := xmlb.UnmarshalWithSource([]byte(`<p:cut`+nsPDecl+` thruBlk="0"/>`), &tr); err != nil {
		t.Fatal(err)
	}
	tr.ThruBlk = true
	got := marshalToString(t, xmlb.NSPresentationML, "cut", &tr)
	if !strings.Contains(got, `thruBlk="1"`) {
		t.Errorf("modeled ThruBlk=true not emitted: %s", got)
	}
}

// The whole timing/transition subtree of a master or layout is re-marshaled on
// every save (pptx/presentation.go marshals them unconditionally), so the
// explicit-zero attributes must survive through the part root writer, not just
// through an isolated element.
func TestC420_MasterTimingAndTransitionByteIdentical(t *testing.T) {
	src := `<p:sldMaster` + nsPDecl + nsADecl + nsRDecl + `>` +
		`<p:cSld><p:spTree/></p:cSld>` +
		`<p:transition spd="fast" advClick="0"><p:fade thruBlk="0"/>` +
		`<p:sndAc><p:stSnd loop="0"><p:snd r:embed="rId9" name="s"/></p:stSnd></p:sndAc></p:transition>` +
		`<p:timing><p:tnLst><p:par><p:cTn id="1" dur="indefinite" nodeType="tmRoot">` +
		`<p:iterate type="el" backwards="0"><p:tmAbs val="100"/></p:iterate>` +
		`<p:childTnLst><p:audio isNarration="0"><p:cMediaNode mute="0" numSld="0">` +
		`<p:cTn id="2"/><p:tgtEl><p:sldTgt/></p:tgtEl></p:cMediaNode></p:audio>` +
		`<p:video fullScrn="0"><p:cMediaNode><p:cTn id="3"/><p:tgtEl><p:sldTgt/></p:tgtEl></p:cMediaNode></p:video>` +
		`</p:childTnLst></p:cTn></p:par></p:tnLst>` +
		`<p:bldLst><p:bldDgm spid="4" grpId="0" uiExpand="0" rev="0"/>` +
		`<p:bldOleChart spid="5" grpId="0" uiExpand="0"/>` +
		`<p:bldGraphic spid="6" grpId="0" uiExpand="0"><p:bldSub><a:bldDgm bld="one" rev="0"/></p:bldSub></p:bldGraphic>` +
		`</p:bldLst></p:timing>` +
		`</p:sldMaster>`

	var sm SlideMaster
	if err := xmlb.UnmarshalWithSource([]byte(src), &sm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b := xmlb.NewPresentationMLBuilder()
	b.SetCollapseEmptyElements(true)
	sm.MarshalRootToBuilder(b)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	if got := b.String(); got != src {
		t.Errorf("master timing/transition not byte-identical\n want: %s\n  got: %s", src, got)
	}
}

// --- C382 / C530: degenerate p:ext bodies ---------------------------------

func TestC382_DegenerateExtensionBodies(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		// C382: a self-closing or whitespace-only sectionLst ext used to abort
		// the whole Open with io.EOF from xml.Unmarshal.
		{"empty sectionLst ext", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURISectionLst + `"/></p:extLst>`},
		{"whitespace sectionLst ext", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURISectionLst + `">
  </p:ext></p:extLst>`},
		// C530: typed URIs whose ext carries no matching child must not
		// fabricate one.
		{"empty creationId ext", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURIPMLCreationId + `"/></p:extLst>`},
		{"empty media ext", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURIMedia + `"/></p:extLst>`},
		{"empty sldGuideLst ext", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURISldGuideLst + `"/></p:extLst>`},
		{"empty laserClr ext", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURILaserClr + `"/></p:extLst>`},
		{"wrong child for creationId uri", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURIPMLCreationId +
			`"><q:other xmlns:q="urn:q" val="7"/></p:ext></p:extLst>`},
		// C530: a typed ext with extra siblings keeps all of them.
		{"creationId with extra sibling", `<p:extLst` + nsPDecl + `><p:ext uri="` + xmlb.ExtURIPMLCreationId +
			`"><p14:creationId` + nsP14Decl + ` val="42"/><q:extra xmlns:q="urn:q"/></p:ext></p:extLst>`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var el ExtensionList
			if err := xmlb.UnmarshalWithSource([]byte(tc.src), &el); err != nil {
				t.Fatalf("parse must not fail: %v", err)
			}
			got := wantSameAsSource(marshalToString(t, xmlb.NSPresentationML, "extLst", &el))
			if want := wantSameAsSource(tc.src); got != want {
				t.Errorf("re-marshal is not byte-identical\n want: %s\n  got: %s", want, got)
			}
		})
	}
}

// --- C523: extension leaves must not double-declare their namespace -------

func TestC523_ExtensionLeafDeclaredOnExtNotDuplicated(t *testing.T) {
	cases := []string{
		`<p:extLst` + nsPDecl + `><p:ext` + nsP14Decl + ` uri="` + xmlb.ExtURIShowMediaCtrls +
			`"><p14:showMediaCtrls val="1"/></p:ext></p:extLst>`,
		`<p:extLst` + nsPDecl + `><p:ext` + nsP14Decl + ` uri="` + xmlb.ExtURIDefaultImageDpi +
			`"><p14:defaultImageDpi val="220"/></p:ext></p:extLst>`,
		`<p:extLst` + nsPDecl + `><p:ext` + nsP14Decl + ` uri="` + xmlb.ExtURIDiscardImageEditData +
			`"><p14:discardImageEditData val="0"/></p:ext></p:extLst>`,
		`<p:extLst` + nsPDecl + `><p:ext` + nsP15Decl + ` uri="` + xmlb.ExtURIPresenceInfo +
			`"><p15:presenceInfo userId="u" providerId="AD"/></p:ext></p:extLst>`,
		`<p:extLst` + nsPDecl + `><p:ext` + nsP15Decl + ` uri="` + xmlb.ExtURIChartTrackingRefBased +
			`"><p15:chartTrackingRefBased val="1"/></p:ext></p:extLst>`,
		`<p:extLst` + nsPDecl + nsADecl + `><p:ext` + nsP14Decl + ` uri="` + xmlb.ExtURILaserClr +
			`"><p14:laserClr><a:srgbClr val="FF0000"/></p14:laserClr></p:ext></p:extLst>`,
	}
	for _, src := range cases {
		var el ExtensionList
		if err := xmlb.UnmarshalWithSource([]byte(src), &el); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		got := marshalToString(t, xmlb.NSPresentationML, "extLst", &el)
		for _, decl := range []string{
			`xmlns:p14="` + xmlb.NSPowerPoint2010 + `"`,
			`xmlns:p15="` + xmlb.NSPowerPoint2012 + `"`,
		} {
			if n := strings.Count(src, decl); n == 0 {
				continue
			}
			if n := strings.Count(got, decl); n != 1 {
				t.Errorf("declaration %s emitted %d times (source declared it once, on p:ext)\n  got: %s", decl, n, got)
			}
		}
		// The declaration must stay on p:ext, where the producer put it.
		if i, j := strings.Index(got, "xmlns:p1"), strings.Index(got, "><p1"); i >= 0 && j >= 0 && i > j {
			t.Errorf("declaration re-homed onto the child: %s", got)
		}
	}
}

// A programmatically built extension still declares its namespace inline, since
// the re-marshaled part root does not carry p14/p15.
func TestC523_ProgrammaticExtensionDeclaresNamespaceInline(t *testing.T) {
	el := &ExtensionList{Ext: []Extension{{
		URI:            xmlb.ExtURIShowMediaCtrls,
		ShowMediaCtrls: &P14ShowMediaCtrls{Val: "1"},
	}}}
	got := marshalToString(t, xmlb.NSPresentationML, "extLst", el)
	if !strings.Contains(got, `xmlns:p14="`+xmlb.NSPowerPoint2010+`"`) {
		t.Errorf("programmatic p14 leaf lost its inline declaration: %s", got)
	}
}

// --- C524: a directly-populated TimeNodeList must not marshal empty -------

func TestC524_DirectlyPopulatedTimeNodeListMarshalsItsNodes(t *testing.T) {
	tnl := &TimeNodeList{
		Par: []*ParallelTimeNode{{CTn: &CommonTimeNode{Id: 1, NodeType: "tmRoot"}}},
		Set: []*Set{{CBhvr: &CommonBehavior{Additive: "repl"}}},
	}
	got := marshalToString(t, xmlb.NSPresentationML, "tnLst", tnl)
	if strings.Contains(got, "<p:tnLst/>") || strings.Contains(got, "<p:tnLst></p:tnLst>") {
		t.Fatalf("directly-populated list marshaled empty (nodes silently dropped): %s", got)
	}
	for _, want := range []string{"<p:par>", `id="1"`, "<p:set>", `additive="repl"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}

	// An empty list still marshals empty.
	if got := marshalToString(t, xmlb.NSPresentationML, "tnLst", &TimeNodeList{}); got != "<p:tnLst/>" {
		t.Errorf("empty list = %q, want <p:tnLst/>", got)
	}
}

// The parsed order still wins where it exists (the fallback must not reorder a
// list that recorded its source sequence).
func TestC524_ParsedOrderStillWins(t *testing.T) {
	src := `<p:tnLst` + nsPDecl + `><p:set><p:cBhvr additive="repl"/></p:set>` +
		`<p:par><p:cTn id="1"/></p:par></p:tnLst>`
	var tnl TimeNodeList
	assertRoundTrip(t, src, xmlb.NSPresentationML, "tnLst", &tnl)
}

// --- C526: default-TRUE presProps/viewProps booleans ----------------------

func TestC526_DefaultTrueBooleansSurviveExplicitZero(t *testing.T) {
	cases := []struct {
		local string
		src   string
		newV  func() any
	}{
		{"htmlPubPr", `<p:htmlPubPr` + nsPDecl + ` showSpeakerNotes="0"/>`,
			func() any { return &HtmlPublishProperties{} }},
		{"webPr", `<p:webPr` + nsPDecl + ` resizeGraphics="0" organizeInFolders="0" useLongFilenames="0"/>`,
			func() any { return &WebProperties{} }},
		{"showPr", `<p:showPr` + nsPDecl + ` showAnimation="0" useTimings="0"/>`,
			func() any { return &ShowProperties{} }},
		{"browse", `<p:browse` + nsPDecl + ` showScrollbar="0"/>`,
			func() any { return &ShowInfoBrowse{} }},
		{"viewPr", `<p:viewPr` + nsPDecl + ` showComments="0"/>`,
			func() any { return &ViewProperties{} }},
		{"normalViewPr", `<p:normalViewPr` + nsPDecl + ` showOutlineIcons="0"/>`,
			func() any { return &NormalViewProperties{} }},
		{"restoredLeft", `<p:restoredLeft` + nsPDecl + ` sz="15000" autoAdjust="0"/>`,
			func() any { return &NormalViewPortion{} }},
		{"cSldViewPr", `<p:cSldViewPr` + nsPDecl + ` snapToGrid="0"/>`,
			func() any { return &CommonSlideViewProperties{} }},
		{"sorterViewPr", `<p:sorterViewPr` + nsPDecl + ` showFormatting="0"/>`,
			func() any { return &SorterViewProperties{} }},
	}
	for _, tc := range cases {
		t.Run(tc.local, func(t *testing.T) {
			got := wantSameAsSource(roundTripPML(t, tc.src, xmlb.NSPresentationML, tc.local, tc.newV()))
			if want := wantSameAsSource(tc.src); got != want {
				t.Errorf("explicit \"0\" on a default-TRUE attribute was deleted\n want: %s\n  got: %s", want, got)
			}
		})
	}
}

// --- C527: p:text coverage in its real context ----------------------------

// p:text is CT_Comment's xsd:string child; it has no standalone Go type, so it
// is exercised here rather than through a struct that existed only to fill a
// slot in the spec type map.
func TestComment_TextChildRoundTripsThroughBuilder(t *testing.T) {
	src := `<p:cm` + nsPDecl + ` authorId="1" dt="2024-01-01T00:00:00" idx="2">` +
		`<p:pos x="100" y="200"/><p:text>Add diagram to clarify.</p:text></p:cm>`
	var cm Comment
	assertRoundTrip(t, src, xmlb.NSPresentationML, "cm", &cm)
	if cm.Text != "Add diagram to clarify." {
		t.Errorf("Comment.Text = %q", cm.Text)
	}
}

// --- C528: unknown root-level children ------------------------------------

func TestC528_UnknownRootChildrenPreserved(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"unknown between typed children",
			`<p:sld` + nsPDecl + `><p:cSld><p:spTree/></p:cSld>` +
				`<q:future xmlns:q="urn:q" a="1"/>` +
				`<p:clrMapOvr/></p:sld>`},
		{"unknown first",
			`<p:sld` + nsPDecl + `><q:future xmlns:q="urn:q"/><p:cSld><p:spTree/></p:cSld></p:sld>`},
		{"two unknowns in a row",
			`<p:sld` + nsPDecl + `><p:cSld><p:spTree/></p:cSld>` +
				`<q:one xmlns:q="urn:q"/><q:two xmlns:q="urn:q"/></p:sld>`},
		{"AlternateContent after an unknown child keeps its position",
			`<p:sld` + nsPDecl + nsMCDecl + `><p:cSld><p:spTree/></p:cSld>` +
				`<q:future xmlns:q="urn:q"/>` +
				`<mc:AlternateContent><mc:Choice Requires="p14"` + nsP14Decl + `><p:transition spd="slow"/></mc:Choice>` +
				`<mc:Fallback><p:transition spd="slow"/></mc:Fallback></mc:AlternateContent>` +
				`</p:sld>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sld Slide
			if err := xmlb.UnmarshalWithSource([]byte(tc.src), &sld); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			b := xmlb.NewPresentationMLBuilder()
			b.SetCollapseEmptyElements(true)
			sld.MarshalRootToBuilder(b)
			if err := b.Finish(); err != nil {
				t.Fatalf("builder: %v", err)
			}
			if got := b.String(); got != tc.src {
				t.Errorf("unknown root child not replayed in place\n want: %s\n  got: %s", tc.src, got)
			}
		})
	}
}

// --- C529: raw children keep their namespace binding and empty-tag form ----

func TestC529_RawChildKeepsRootDeclaredPrefix(t *testing.T) {
	// ink: is declared on the part root, not on the raw child. Re-emission must
	// keep the element in that namespace — it used to fall through to an
	// unprefixed <trace>, i.e. a different namespace entirely.
	src := `<p:spTree` + nsPDecl + ` xmlns:ink="urn:ink"><p:nvGrpSpPr/><p:grpSpPr/>` +
		`<ink:trace id="7">data</ink:trace></p:spTree>`
	var st ShapeTree
	if err := xmlb.UnmarshalWithSource([]byte(src), &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := marshalToString(t, xmlb.NSPresentationML, "spTree", &st)
	if strings.Contains(got, "<trace") {
		t.Fatalf("raw child lost its namespace prefix (moved namespaces): %s", got)
	}
	if !strings.Contains(got, ":trace") || !strings.Contains(got, `"urn:ink"`) {
		t.Errorf("raw child namespace not bound in output: %s", got)
	}
}

func TestC529_RawChildKeepsExpandedEmptyTag(t *testing.T) {
	src := `<p:spTree` + nsPDecl + `><p:nvGrpSpPr/><p:grpSpPr/>` +
		`<p:contentPart r:id="rId4" xmlns:r="` + xmlb.NSOfficeDocumentRels + `"></p:contentPart></p:spTree>`
	var st ShapeTree
	if err := xmlb.UnmarshalWithSource([]byte(src), &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := marshalToString(t, xmlb.NSPresentationML, "spTree", &st)
	if !strings.Contains(got, `></p:contentPart>`) {
		t.Errorf("expanded empty raw child rewritten to a self-closing tag: %s", got)
	}
}

// --- C531: xsd:double lexical forms ---------------------------------------

func TestC531_AnimVariantFloatKeepsLexicalForm(t *testing.T) {
	for _, lex := range []string{"1.0", "1E3", "0.500", "-0.0", "1e-7"} {
		src := `<p:fltVal` + nsPDecl + ` val="` + lex + `"/>`
		var f AnimVariantFloat
		assertRoundTrip(t, src, xmlb.NSPresentationML, "fltVal", &f)
	}
	// A programmatic value renders from the number.
	got := marshalToString(t, xmlb.NSPresentationML, "fltVal", &AnimVariantFloat{Val: 2.5})
	if got != `<p:fltVal val="2.5"/>` {
		t.Errorf("programmatic fltVal = %s", got)
	}
}

// --- C525: modern comment / author regeneration ---------------------------

func TestC525_ModernCommentPreservesEmptyReplyLstAndAuthorAttrs(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p188:cmLst` + nsADecl + nsRDecl + ` xmlns:p188="` + xmlb.NSPowerPointComment2018 + `">` +
		`<p188:cm id="{A}" authorId="{B}" created="2024-01-01T00:00:00.000">` +
		`<p188:replyLst/>` +
		`<p188:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>hi</a:t></a:r></a:p></p188:txBody>` +
		`</p188:cm></p188:cmLst>`
	part, err := ParseModernCommentPart([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(part.Marshal())
	if !strings.Contains(got, "replyLst") {
		t.Errorf("empty <p188:replyLst/> deleted on re-marshal: %s", got)
	}
}

func TestC525_ModernAuthorDoesNotInventEmptyAttributes(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p188:authorLst` + nsADecl + nsRDecl + ` xmlns:p188="` + xmlb.NSPowerPointComment2018 + `">` +
		`<p188:author id="{A}" name="Ada"/>` +
		`</p188:authorLst>`
	lst, err := ParseModernAuthorList([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(lst.Marshal())
	for _, unwanted := range []string{`initials=""`, `userId=""`, `providerId=""`} {
		if strings.Contains(got, unwanted) {
			t.Errorf("re-marshal invented %s: %s", unwanted, got)
		}
	}
	if !strings.Contains(got, `id="{A}"`) || !strings.Contains(got, `name="Ada"`) {
		t.Errorf("modeled author attributes lost: %s", got)
	}
}

func TestC525_ModernAuthorKeepsSourceAttributeOrder(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p188:authorLst` + nsADecl + nsRDecl + ` xmlns:p188="` + xmlb.NSPowerPointComment2018 + `">` +
		`<p188:author name="Ada" initials="AL" id="{A}" providerId="AD" userId="u@x"/>` +
		`</p188:authorLst>`
	lst, err := ParseModernAuthorList([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := string(lst.Marshal())
	if !strings.Contains(got, `name="Ada" initials="AL" id="{A}" providerId="AD" userId="u@x"`) {
		t.Errorf("author attribute order not preserved: %s", got)
	}
}

// A library-created author (no capture) still takes the canonical form.
func TestC525_ProgrammaticAuthorKeepsCanonicalForm(t *testing.T) {
	lst := &ModernAuthorList{Authors: []*ModernAuthor{{ID: "{A}", Name: "Ada"}}}
	got := string(lst.Marshal())
	if !strings.Contains(got, `initials=""`) {
		t.Errorf("programmatic author lost the canonical attribute set: %s", got)
	}
}
