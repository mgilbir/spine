package oxml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// rawOddityHolder mirrors how a raw element is really reached: the declaration
// it relies on is on an ancestor, so the element itself carries none.
type rawOddityHolder struct {
	Odd CT_RawElement `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 oddity"`
}

// replayRawOddity re-emits the parsed element inside a w:document carrying the
// standard WordprocessingML declarations — which bind the markup-compatibility
// namespace to mc, and to nothing else.
func replayRawOddity(t *testing.T, src string) string {
	t.Helper()
	var h rawOddityHolder
	if err := xmlb.UnmarshalWithSource([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b := xmlb.NewWordprocessingMLBuilder()
	b.StartElementWithNS(xmlb.NSWordprocessingML, "document", xmlb.WordprocessingMLNamespaces())
	h.Odd.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "oddity")
	b.EndElement(xmlb.NSWordprocessingML, "document")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v\n%s", err, b.String())
	}
	return b.String()
}

// C375's shape reached through the verbatim raw-element replay. undeclaredNSDecl
// asked only whether the element's *namespace* was registered, so a raw element
// whose source prefix is an alias of a registered URI was replayed under that
// alias with no declaration carried: Word 2007 binds the markup-compatibility
// namespace to both mc and ve, and the builder registers only mc. The literal
// path writes the name verbatim, so the result was an undeclared prefix that
// Builder.Finish accepted.
func TestRawElement_AliasedPrefixCarriesItsDeclaration(t *testing.T) {
	out := replayRawOddity(t, `<w:document xmlns:w="`+xmlb.NSWordprocessingML+`"`+
		` xmlns:ve="`+xmlb.NSMarkupCompatibility+`"><ve:oddity ve:note="x"/></w:document>`)

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if !strings.Contains(out, `<ve:oddity xmlns:ve="`+xmlb.NSMarkupCompatibility+`"`) {
		t.Errorf("aliased raw element did not carry its own declaration:\n%s", out)
	}
}

// The ordinary case must not gain a declaration it did not need: a raw element
// written under a prefix the destination root already declares keeps its bytes.
func TestRawElement_InScopePrefixGainsNoDeclaration(t *testing.T) {
	out := replayRawOddity(t, `<w:document xmlns:w="`+xmlb.NSWordprocessingML+`"`+
		` xmlns:mc="`+xmlb.NSMarkupCompatibility+`"><mc:oddity mc:note="x"/></w:document>`)

	if err := testutil.NamespaceWellFormed(out); err != nil {
		t.Errorf("%v:\n%s", err, out)
	}
	if strings.Count(out, `xmlns:mc="`+xmlb.NSMarkupCompatibility+`"`) != 1 {
		t.Errorf("raw element under an already-declared prefix gained a redundant declaration:\n%s", out)
	}
	if !strings.Contains(out, `<mc:oddity mc:note="x"/>`) {
		t.Errorf("raw element did not replay verbatim:\n%s", out)
	}
}
