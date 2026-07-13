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
