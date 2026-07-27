package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C30/C36: a p:extLst inside a transition holds p:ext children (not a:ext).
// An unknown-URI extension, including xmlns declarations carried on the ext
// element, must round-trip byte-faithfully through the production Builder.
func TestTransition_ExtLst_UnknownExtRoundTrip(t *testing.T) {
	fragment := `<p:transition xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" spd="slow">` +
		`<p:fade/>` +
		`<p:extLst><p:ext uri="{AAAAAAAA-0000-0000-0000-000000000000}" xmlns:foo="urn:example:foo">` +
		`<foo:thing a="1"/>` +
		`</p:ext></p:extLst></p:transition>`

	var tr Transition
	if err := xml.Unmarshal([]byte(fragment), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tr.ExtLst == nil || len(tr.ExtLst.Ext) != 1 {
		t.Fatalf("p:ext not parsed: %+v", tr.ExtLst)
	}
	ext := tr.ExtLst.Ext[0]
	if ext.URI != "{AAAAAAAA-0000-0000-0000-000000000000}" {
		t.Errorf("URI = %q", ext.URI)
	}
	if !strings.Contains(string(ext.RawContent), "<foo:thing") {
		t.Errorf("RawContent = %q, want foo:thing captured", ext.RawContent)
	}
	if len(ext.InlineNSDecls) != 1 || ext.InlineNSDecls[0].Prefix != "foo" {
		t.Errorf("InlineNSDecls = %+v, want xmlns:foo captured", ext.InlineNSDecls)
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "transition", &tr)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	// The fragment's own xmlns:p declaration is captured and replayed
	// verbatim (in a real part the declaration lives on the part root).
	want := `<p:transition xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" spd="slow">` +
		`<p:fade/>` +
		`<p:extLst><p:ext uri="{AAAAAAAA-0000-0000-0000-000000000000}" xmlns:foo="urn:example:foo">` +
		`<foo:thing a="1"/>` +
		`</p:ext></p:extLst></p:transition>`
	if got := b.String(); got != want {
		t.Errorf("round-trip mismatch:\n got %s\nwant %s", got, want)
	}
}

// C30: same for a p:extLst inside p:timing (previously typed dml.ExtLst,
// whose Ext only matched a:ext, so p:ext extensions parsed empty and were
// deleted on every save).
func TestTiming_ExtLst_UnknownExtRoundTrip(t *testing.T) {
	fragment := `<p:timing xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:extLst mod="1"><p:ext uri="{BBBBBBBB-0000-0000-0000-000000000000}" xmlns:foo="urn:example:foo">` +
		`<foo:data val="7"/>` +
		`</p:ext></p:extLst></p:timing>`

	var tm Timing
	if err := xml.Unmarshal([]byte(fragment), &tm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tm.ExtLst == nil || len(tm.ExtLst.Ext) != 1 {
		t.Fatalf("p:ext not parsed: %+v", tm.ExtLst)
	}
	if tm.ExtLst.Mod == nil || !*tm.ExtLst.Mod {
		t.Errorf("mod attribute not captured: %+v", tm.ExtLst.Mod)
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "timing", &tm)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<p:timing>` +
		`<p:extLst mod="1"><p:ext uri="{BBBBBBBB-0000-0000-0000-000000000000}" xmlns:foo="urn:example:foo">` +
		`<foo:data val="7"/>` +
		`</p:ext></p:extLst></p:timing>`
	if got := b.String(); got != want {
		t.Errorf("round-trip mismatch:\n got %s\nwant %s", got, want)
	}
}

// Known-URI extensions keep their typed dispatch: a declaration the source
// carried on the p:ext element is replayed there, and the bare typed child
// stays bare (verbatim placement, no re-homing onto the child).
func TestExtension_KnownURI_TypedDispatchStillWorks(t *testing.T) {
	fragment := `<p:extLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:ext uri="{BB962C8B-B14F-4D97-AF65-F5344CB8AC3E}" xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
		`<p14:creationId val="3813894636"/>` +
		`</p:ext></p:extLst>`

	var el ExtensionList
	if err := xml.Unmarshal([]byte(fragment), &el); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(el.Ext) != 1 || el.Ext[0].CreationId == nil {
		t.Fatalf("typed creationId dispatch broken: %+v", el.Ext)
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "extLst", &el)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := b.String()
	want := `<p:ext uri="{BB962C8B-B14F-4D97-AF65-F5344CB8AC3E}" xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
		`<p14:creationId val="3813894636"/></p:ext>`
	if !strings.Contains(out, want) {
		t.Errorf("typed dispatch did not preserve the source's declaration placement: %s", out)
	}
}

// A programmatically built p14:media with a Link (externally linked media)
// emits its r:link attribute rather than dropping it (C355).
func TestExtension_P14Media_LinkEmitted(t *testing.T) {
	el := &ExtensionList{
		Ext: []Extension{{
			URI:   xmlb.ExtURIMedia,
			Media: &P14Media{Embed: "rId1", Link: "rId2"},
		}},
	}
	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "extLst", el)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := b.String()
	if !strings.Contains(out, `r:embed="rId1"`) {
		t.Errorf("p14:media dropped r:embed:\n%s", out)
	}
	if !strings.Contains(out, `r:link="rId2"`) {
		t.Errorf("p14:media dropped programmatic r:link:\n%s", out)
	}
}

// A p14:sectionLst extension parses into the typed Section model and, when
// unmodified, replays its source bytes verbatim (byte-identical round-trip).
func TestExtension_SectionLst_RoundTrip(t *testing.T) {
	sectionExt := `<p:ext uri="{521415D9-36F7-43E2-AB2F-B90AF26B5E84}">` +
		`<p14:sectionLst xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
		`<p14:section name="Default Section" id="{124617AE-E5F0-462F-B980-77B306D58FBF}">` +
		`<p14:sldIdLst><p14:sldId id="256"/><p14:sldId id="630"/></p14:sldIdLst>` +
		`</p14:section>` +
		`<p14:section name="Untitled Section" id="{05E1D2DE-88C5-4E6F-B731-0AEA3E39ACC8}">` +
		`<p14:sldIdLst/>` +
		`</p14:section>` +
		`</p14:sectionLst></p:ext>`
	fragment := `<p:extLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		sectionExt + `</p:extLst>`

	var el ExtensionList
	if err := xml.Unmarshal([]byte(fragment), &el); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(el.Ext) != 1 || el.Ext[0].SectionLst == nil {
		t.Fatalf("sectionLst not parsed into typed model: %+v", el.Ext)
	}
	sl := el.Ext[0].SectionLst
	if len(sl.Section) != 2 {
		t.Fatalf("got %d sections, want 2", len(sl.Section))
	}
	if sl.Section[0].Name != "Default Section" ||
		sl.Section[0].ID != "{124617AE-E5F0-462F-B980-77B306D58FBF}" {
		t.Errorf("section[0] = %+v", sl.Section[0])
	}
	if len(sl.Section[0].SldId) != 2 || sl.Section[0].SldId[0] != 256 || sl.Section[0].SldId[1] != 630 {
		t.Errorf("section[0].SldId = %v, want [256 630]", sl.Section[0].SldId)
	}
	if len(sl.Section[1].SldId) != 0 {
		t.Errorf("section[1].SldId = %v, want empty", sl.Section[1].SldId)
	}
	if sl.Dirty() {
		t.Errorf("freshly parsed section list should not be dirty")
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "extLst", &el)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	if got := b.String(); !strings.Contains(got, sectionExt) {
		t.Errorf("unmodified section list not replayed verbatim:\n got %s\nwant substring %s", got, sectionExt)
	}
}

// A mutated (or programmatically built) section list regenerates the canonical
// PowerPoint form, with an empty member list emitted self-closing.
func TestExtension_SectionLst_Regenerate(t *testing.T) {
	sl := &P14SectionLst{
		Section: []*P14Section{
			{Name: "Intro", ID: "{AAAAAAAA-0000-0000-0000-000000000001}", SldId: []uint32{256}},
			{Name: "Empty", ID: "{AAAAAAAA-0000-0000-0000-000000000002}"},
		},
	}
	sl.MarkDirty()
	el := &ExtensionList{Ext: []Extension{{URI: xmlb.ExtURISectionLst, SectionLst: sl}}}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "extLst", el)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<p:ext uri="{521415D9-36F7-43E2-AB2F-B90AF26B5E84}">` +
		`<p14:sectionLst xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
		`<p14:section name="Intro" id="{AAAAAAAA-0000-0000-0000-000000000001}">` +
		`<p14:sldIdLst><p14:sldId id="256"/></p14:sldIdLst></p14:section>` +
		`<p14:section name="Empty" id="{AAAAAAAA-0000-0000-0000-000000000002}">` +
		`<p14:sldIdLst/></p14:section>` +
		`</p14:sectionLst></p:ext>`
	if got := b.String(); !strings.Contains(got, want) {
		t.Errorf("regenerated section list mismatch:\n got %s\nwant substring %s", got, want)
	}
}

// Comment extension lists are p:ext extension lists too (C30).
func TestComment_ExtLst_RoundTrip(t *testing.T) {
	fragment := `<p:cm xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" authorId="0" idx="1">` +
		`<p:pos x="10" y="10"/>` +
		`<p:text>hello</p:text>` +
		`<p:extLst><p:ext uri="{CCCCCCCC-0000-0000-0000-000000000000}" xmlns:foo="urn:example:foo"><foo:meta/></p:ext></p:extLst>` +
		`</p:cm>`

	var cm Comment
	if err := xml.Unmarshal([]byte(fragment), &cm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cm.ExtLst == nil || len(cm.ExtLst.Ext) != 1 {
		t.Fatalf("comment p:ext not parsed: %+v", cm.ExtLst)
	}
	if !strings.Contains(string(cm.ExtLst.Ext[0].RawContent), "<foo:meta/>") {
		t.Errorf("RawContent = %q", cm.ExtLst.Ext[0].RawContent)
	}
}
