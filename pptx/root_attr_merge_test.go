package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// C247: root-attribute replay must merge modeled writes rather than replaying
// the captured source list verbatim. On an opened deck, root setters such as
// SetEmbedTrueTypeFonts and SlideLayout.SetName were silent no-ops because the
// marshal path discarded the modeled attributes.

func TestRootAttrMerge_EmbedTrueTypeFontsOnOpenedDeck(t *testing.T) {
	// An opened deck carries OriginalRootAttrs, so the merged root writer is
	// exercised (a created deck would take the programmatic branch).
	p := openBytes(t, savedDeck(t))
	p.SetEmbedTrueTypeFonts(true)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	pres := string(zipPart(t, data, "ppt/presentation.xml"))
	if !strings.Contains(pres, `embedTrueTypeFonts="1"`) {
		t.Errorf("SetEmbedTrueTypeFonts(true) not reflected in saved root:\n%s", pres)
	}
	if got := openBytes(t, data); !got.EmbedTrueTypeFonts() {
		t.Error("reopened deck does not report embedTrueTypeFonts")
	}

	// On a baseline whose source never carried the flag, setting it false must
	// not inject the attribute: the merge appends only a modeled value, and the
	// modeled list omits embedTrueTypeFonts when it is not enabled.
	p2 := openBytes(t, savedDeck(t))
	p2.SetEmbedTrueTypeFonts(false)
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if pres2 := string(zipPart(t, data2, "ppt/presentation.xml")); strings.Contains(pres2, "embedTrueTypeFonts") {
		t.Errorf("clearing added a spurious embedTrueTypeFonts attribute:\n%s", pres2)
	}
}

func TestRootAttrMerge_LayoutSetNameOnOpenedDeck(t *testing.T) {
	p := openBytes(t, savedDeck(t))
	layouts := p.SlideLayouts()
	if len(layouts) == 0 {
		t.Fatal("no layouts in deck")
	}
	layouts[0].SetName("Renamed")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// The renamed matchingName appears in the layout part and survives reopen.
	var found string
	reopened := openBytes(t, data)
	for _, l := range reopened.SlideLayouts() {
		if l.Name() == "Renamed" {
			found = l.Name()
			break
		}
	}
	if found != "Renamed" {
		t.Errorf("layout matchingName not renamed after reopen; layouts=%v", layoutNames(reopened))
	}
}

func layoutNames(p *Presentation) []string {
	var names []string
	for _, l := range p.SlideLayouts() {
		names = append(names, l.Name())
	}
	return names
}

// The merge must be a no-op when nothing was modeled-modified: an opened deck
// re-saved twice yields byte-identical OPC parts (critical for the cctest
// byte-identity contract).
func TestRootAttrMerge_UnchangedRootByteIdentical(t *testing.T) {
	base := savedDeck(t)

	p1 := openBytes(t, base)
	data1, err := p1.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p2 := openBytes(t, data1)
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	for _, part := range []string{"ppt/presentation.xml", "ppt/slideLayouts/slideLayout1.xml", "ppt/slideMasters/slideMaster1.xml"} {
		a := zipPart(t, data1, part)
		b := zipPart(t, data2, part)
		if !bytes.Equal(a, b) {
			t.Errorf("part %q not byte-identical across re-save (merge is not a no-op)\nfirst:\n%s\nsecond:\n%s", part, a, b)
		}
	}
}
