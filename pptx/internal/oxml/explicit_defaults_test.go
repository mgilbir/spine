package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C29: advTm has no XSD default — an explicit advTm="0" (advance immediately)
// must survive re-marshal instead of being deleted.
func TestTransition_ExplicitAdvTmZeroPreserved(t *testing.T) {
	src := `<p:transition xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" advClick="0" advTm="0"><p:fade/></p:transition>`
	var tr Transition
	if err := xml.Unmarshal([]byte(src), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tr.AdvTm == nil || *tr.AdvTm != 0 {
		t.Fatalf("AdvTm = %v, want explicit 0", tr.AdvTm)
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "transition", &tr)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	if got := b.String(); !strings.Contains(got, `advTm="0"`) {
		t.Errorf("explicit advTm=0 deleted: %s", got)
	}
}

// C224: display/autoRev/afterEffect/nodePh have no XSD default on p:cTn — an
// explicit "0" must round-trip on the always-remarshaled timing path.
func TestCommonTimeNode_ExplicitFalseAttrsPreserved(t *testing.T) {
	src := `<p:cTn xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"` +
		` id="4" autoRev="0" display="0" afterEffect="0" nodePh="0" nodeType="clickEffect"/>`
	var ctn CommonTimeNode
	if err := xml.Unmarshal([]byte(src), &ctn); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ctn.Display == nil || *ctn.Display {
		t.Fatalf("Display = %v, want explicit false", ctn.Display)
	}

	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(xmlb.NSPresentationML, "cTn", &ctn)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := b.String()
	for _, attr := range []string{`autoRev="0"`, `display="0"`, `afterEffect="0"`, `nodePh="0"`} {
		if !strings.Contains(out, attr) {
			t.Errorf("explicit %s deleted: %s", attr, out)
		}
	}
}
