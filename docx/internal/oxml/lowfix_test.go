package oxml

import (
	"math"
	"strconv"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C509: the two models of w:headerReference/w:footerReference emitted its
// attributes in opposite orders — CT_HdrFtrRef writes Word's w:type-first form,
// CT_HeaderReference wrote r:id first — so which order a programmatically built
// reference got depended on which type happened to hold it.
func TestHeaderReferenceAttrOrderMatchesHdrFtrRef(t *testing.T) {
	render := func(marshal func(*xmlb.Builder)) string {
		b := xmlb.NewWordprocessingMLBuilder()
		b.StartElementWithNS(xmlb.NSWordprocessingML, "sectPr", xmlb.WordprocessingMLNamespaces())
		marshal(b)
		b.EndElement(xmlb.NSWordprocessingML, "sectPr")
		if err := b.Finish(); err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return b.String()
	}

	ref := &CT_HdrFtrRef{Type: "default", RID: "rId4"}
	standalone := &CT_HeaderReference{Type: "default", RID: "rId4"}

	gotRef := render(func(b *xmlb.Builder) {
		ref.marshalTo(b, xmlb.NSWordprocessingML, "headerReference")
	})
	gotStandalone := render(func(b *xmlb.Builder) {
		standalone.MarshalToBuilder(b, xmlb.NSWordprocessingML, "headerReference")
	})

	if !strings.Contains(gotRef, `<w:headerReference w:type="default" r:id="rId4"`) {
		t.Fatalf("CT_HdrFtrRef no longer emits Word's order: %s", gotRef)
	}
	if gotStandalone != gotRef {
		t.Errorf("the two models of w:headerReference disagree:\n CT_HdrFtrRef:       %s\n CT_HeaderReference: %s",
			gotRef, gotStandalone)
	}
}

// C511: atoiOK fed every id allocator, so it had to reject anything it could
// not read faithfully. The hand-rolled loop accepted a bare "-" as 0 and
// silently wrapped past the int range, either of which hands the allocator a
// bogus maximum and lets the next id collide.
func TestAtoiOKRejectsMalformedAndOverflowing(t *testing.T) {
	overflow := strconv.FormatUint(math.MaxUint64, 10) + "0"
	bad := []string{"", "-", "+", "1-", "-1-", " 1", "1 ", "0x10", "１", overflow, "-" + overflow}
	for _, s := range bad {
		if v, ok := atoiOK(s); ok {
			t.Errorf("atoiOK(%q) = (%d, true), want rejected", s, v)
		}
	}
	good := map[string]int{"0": 0, "7": 7, "-1": -1, "-0": 0, "2147483647": 2147483647}
	for s, want := range good {
		v, ok := atoiOK(s)
		if !ok || v != want {
			t.Errorf("atoiOK(%q) = (%d, %v), want (%d, true)", s, v, ok, want)
		}
	}
}
