package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// idEntryExt is a p:extLst in the byte form the marshaler emits, used inside
// sldMasterId/sldId entries and photoAlbum.
const idEntryExt = `<p:extLst><p:ext uri="{DDDDDDDD-0000-0000-0000-000000000000}" xmlns:foo="urn:example:foo"><foo:mark val="1"/></p:ext></p:extLst>`

// deckWithPresentationXMLRewrite builds a single-slide deck and rewrites
// ppt/presentation.xml.
func deckWithPresentationXMLRewrite(t *testing.T, rewrite func([]byte) []byte) []byte {
	t.Helper()
	p := Create()
	p.AddSlide()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/presentation.xml", rewrite)
}

func openAndResave(t *testing.T, deck []byte) string {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return string(zipPart(t, saved, "ppt/presentation.xml"))
}

// C225: the optional extLst child of sldMasterId and sldId entries survives
// the presentation.xml regeneration every save performs.
func TestSldIdEntryExtLstRoundTrip(t *testing.T) {
	deck := deckWithPresentationXMLRewrite(t, func(xml []byte) []byte {
		xml = bytes.Replace(xml,
			[]byte(`<p:sldMasterId id="2147483648" r:id="rId1"/>`),
			[]byte(`<p:sldMasterId id="2147483648" r:id="rId1">`+idEntryExt+`</p:sldMasterId>`), 1)
		return bytes.Replace(xml,
			[]byte(`<p:sldId id="256" r:id="rId2"/>`),
			[]byte(`<p:sldId id="256" r:id="rId2">`+idEntryExt+`</p:sldId>`), 1)
	})

	presXML := openAndResave(t, deck)
	if !strings.Contains(presXML, `<p:sldMasterId id="2147483648" r:id="rId1">`+idEntryExt+`</p:sldMasterId>`) {
		t.Errorf("sldMasterId extLst lost:\n%s", presXML)
	}
	if !strings.Contains(presXML, `<p:sldId id="256" r:id="rId2">`+idEntryExt+`</p:sldId>`) {
		t.Errorf("sldId extLst lost:\n%s", presXML)
	}
}

// C225: photoAlbum keeps its attributes and extLst child across a save.
func TestPhotoAlbumExtLstRoundTrip(t *testing.T) {
	album := `<p:photoAlbum bw="1" layout="2pic">` + idEntryExt + `</p:photoAlbum>`
	deck := deckWithPresentationXMLRewrite(t, func(xml []byte) []byte {
		return bytes.Replace(xml, []byte(`<p:defaultTextStyle>`),
			[]byte(album+`<p:defaultTextStyle>`), 1)
	})

	presXML := openAndResave(t, deck)
	if !strings.Contains(presXML, album) {
		t.Errorf("photoAlbum extLst lost:\n%s", presXML)
	}
}

// C4: a custom show keeps its XSD-required sldLst (and extLst) instead of
// re-emitting a schema-invalid empty <p:custShow name id/>.
func TestCustomShowRoundTrip(t *testing.T) {
	custShow := `<p:custShowLst><p:custShow name="Demo" id="0">` +
		`<p:sldLst><p:sld r:id="rId2"/></p:sldLst>` +
		idEntryExt +
		`</p:custShow></p:custShowLst>`
	deck := deckWithPresentationXMLRewrite(t, func(xml []byte) []byte {
		return bytes.Replace(xml, []byte(`<p:defaultTextStyle>`),
			[]byte(custShow+`<p:defaultTextStyle>`), 1)
	})

	presXML := openAndResave(t, deck)
	if !strings.Contains(presXML, custShow) {
		t.Errorf("custShow sldLst/extLst lost:\n%s", presXML)
	}
	if strings.Contains(presXML, `<p:custShow name="Demo" id="0"/>`) {
		t.Errorf("schema-invalid empty custShow emitted:\n%s", presXML)
	}
}

// twoSlideDeckWithIDs builds a two-slide deck and rewrites the id attribute
// of its two p:sldId entries (256 and 257) to the given attribute text, which
// may be empty to drop the attribute altogether.
func twoSlideDeckWithIDs(t *testing.T, first, second string) []byte {
	t.Helper()
	p := Create()
	p.AddSlide()
	p.AddSlide()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/presentation.xml", func(xml []byte) []byte {
		xml = bytes.Replace(xml, []byte(`<p:sldId id="256" `), []byte(`<p:sldId `+first+` `), 1)
		return bytes.Replace(xml, []byte(`<p:sldId id="257" `), []byte(`<p:sldId `+second+` `), 1)
	})
}

// assertDistinctSlideIDs fails when two p:sldId entries in presXML share an id,
// and returns the ids in order.
func assertDistinctSlideIDs(t *testing.T, presXML []byte) []string {
	t.Helper()
	refs, err := scanIDRefs(presXML, "sldId")
	if err != nil {
		t.Fatalf("saved presentation.xml does not parse: %v", err)
	}
	var ids []string
	seen := map[string]bool{}
	for _, r := range refs {
		if seen[r.ID] {
			t.Errorf("slide id %q appears twice in the saved sldIdLst:\n%s", r.ID, presXML)
		}
		seen[r.ID] = true
		ids = append(ids, r.ID)
	}
	return ids
}

// An sldId with no usable id loaded as id 0 and saved as id="0", colliding with
// a real id 0 or with another such entry. Found by FuzzPptxPresentationXML in
// the nightly of 2026-09-26.
func TestSldIdWithoutUsableIDGetsFreshID(t *testing.T) {
	cases := []struct{ name, first, second string }{
		{"missing beside an explicit 0", `id="0"`, ``},
		{"missing twice", ``, ``},
		{"not a number beside an explicit 0", `id="0"`, `id="abc"`},
		{"out of range beside an explicit 0", `id="0"`, `id="4294967296"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			presXML := openAndResave(t, twoSlideDeckWithIDs(t, tc.first, tc.second))
			ids := assertDistinctSlideIDs(t, []byte(presXML))
			if len(ids) != 2 {
				t.Fatalf("saved sldIdLst has %d entries, want 2:\n%s", len(ids), presXML)
			}
			if tc.first == `id="0"` && ids[0] != "0" {
				t.Errorf("explicit id 0 was renumbered to %s", ids[0])
			}
		})
	}
}

// A deck holding the largest slide id wrapped the next-id counter to 0, so the
// next AddSlide reused whatever low id the deck already had.
func TestAddSlideAfterMaxSlideIDDoesNotReuseAnID(t *testing.T) {
	// The low id must lie where the wrapped counter restarts, or reusing it goes
	// unnoticed.
	deck := twoSlideDeckWithIDs(t, `id="0"`, `id="4294967295"`)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	p.AddSlide()
	p.AddSlide()
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if ids := assertDistinctSlideIDs(t, zipPart(t, saved, "ppt/presentation.xml")); len(ids) != 4 {
		t.Fatalf("saved sldIdLst has %d entries, want 4: %v", len(ids), ids)
	}
}
