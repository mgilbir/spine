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
