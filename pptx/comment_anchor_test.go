package pptx

import (
	"encoding/xml"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// modernThreadAnchoredXML is modernThreadXML with a pc:spMk in the anchor
// marker list, i.e. a comment attached to shape 5 rather than to the slide.
const modernThreadAnchoredXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p188:cmLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main"><p188:cm id="{2731B08D-2FF5-074E-A2EE-D3516BCF8095}" authorId="{7E013C82-7D75-1E69-2E91-2087A44DBE8C}" created="2025-01-30T11:09:28.599"><pc:sldMkLst xmlns:pc="http://schemas.microsoft.com/office/powerpoint/2013/main/command"><pc:docMk/><pc:sldMk cId="1836216607" sldId="256"/><pc:spMk id="5" creationId="{9AB1D2C3-0000-0000-0000-000000000000}"/></pc:sldMkLst><p188:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Anchored to a shape.</a:t></a:r></a:p></p188:txBody></p188:cm></p188:cmLst>`

// TestCommentIDAndAnchorShapeID covers Comment.ID for both comment mechanisms
// and Comment.AnchorShapeID for the anchored / unanchored split. Both are
// lookups that could plausibly return a constant: ID could return the author id
// and AnchorShapeID could report (0, true) for everything, neither of which a
// single happy-path call detects.
func TestCommentIDAndAnchorShapeID(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		p, err := Open("testdata/test.pptx")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		s := firstSlide(t, p)
		p.otherParts[legacyAuthorsPart] = &coxml.RawPart{ContentType: opc.ContentTypePresentationCommentAuthors, Data: []byte(legacyAuthorsXML)}
		injectSlidePart(p, s, "/ppt/comments/comment1.xml", opc.ContentTypePresentationComments, opc.RelTypeComments, legacyCommentXML)

		comments := s.Comments()
		if len(comments) != 1 {
			t.Fatalf("got %d comments, want 1", len(comments))
		}
		c := comments[0]
		// The legacy id is the comment's idx attribute, which is 1 in the
		// fixture — not the authorId (0) and not the empty string.
		if got := c.ID(); got != "1" {
			t.Errorf("legacy Comment.ID() = %q, want \"1\" (the p:cm idx)", got)
		}
		// Legacy comments have no shape anchor at all.
		if id, ok := c.AnchorShapeID(); ok || id != 0 {
			t.Errorf("legacy AnchorShapeID() = (%d, %v), want (0, false)", id, ok)
		}
	})

	t.Run("modern-slide-anchored", func(t *testing.T) {
		p, err := Open("testdata/test.pptx")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		s := firstSlide(t, p)
		p.otherParts[modernAuthorsPart] = &coxml.RawPart{ContentType: opc.ContentTypeAuthors, Data: []byte(modernAuthorsXML)}
		injectSlidePart(p, s, "/ppt/comments/modernComment1.xml", opc.ContentTypeModernComments, opc.RelTypeModernComments, modernThreadXML)

		comments := s.Comments()
		if len(comments) != 1 {
			t.Fatalf("got %d comments, want 1", len(comments))
		}
		top := comments[0]
		if got, want := top.ID(), "{1731B08D-2FF5-074E-A2EE-D3516BCF8095}"; got != want {
			t.Errorf("modern Comment.ID() = %q, want %q", got, want)
		}
		// A reply carries its own id, not its parent's.
		if len(top.Replies()) != 1 {
			t.Fatalf("got %d replies, want 1", len(top.Replies()))
		}
		reply := top.Replies()[0]
		if got, want := reply.ID(), "{C720E66C-5F4D-41AB-B8D6-6EAE84414019}"; got != want {
			t.Errorf("reply Comment.ID() = %q, want %q", got, want)
		}
		if reply.ID() == top.ID() {
			t.Error("the reply and its parent report the same ID")
		}
		// This thread's marker list has a sldMk but no spMk: the comment is
		// anchored to the slide, so AnchorShapeID must report a miss.
		if id, ok := top.AnchorShapeID(); ok || id != 0 {
			t.Errorf("slide-anchored AnchorShapeID() = (%d, %v), want (0, false)", id, ok)
		}
	})

	t.Run("modern-shape-anchored", func(t *testing.T) {
		p, err := Open("testdata/test.pptx")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		s := firstSlide(t, p)
		p.otherParts[modernAuthorsPart] = &coxml.RawPart{ContentType: opc.ContentTypeAuthors, Data: []byte(modernAuthorsXML)}
		injectSlidePart(p, s, "/ppt/comments/modernComment1.xml", opc.ContentTypeModernComments, opc.RelTypeModernComments, modernThreadAnchoredXML)

		comments := s.Comments()
		if len(comments) != 1 {
			t.Fatalf("got %d comments, want 1", len(comments))
		}
		c := comments[0]
		if got, want := c.ID(), "{2731B08D-2FF5-074E-A2EE-D3516BCF8095}"; got != want {
			t.Errorf("Comment.ID() = %q, want %q", got, want)
		}
		id, ok := c.AnchorShapeID()
		if !ok {
			t.Fatal("AnchorShapeID() reported no anchor for a comment carrying a pc:spMk")
		}
		// 5 is the spMk id; it must not be confused with the sldMk sldId (256)
		// or the cId (1836216607), both of which are also numeric attributes in
		// the same marker list.
		if id != 5 {
			t.Errorf("AnchorShapeID() = %d, want 5 (sldId is 256, cId is 1836216607)", id)
		}
	})
}

// TestScanShapeMarkID exercises the raw-marker scan directly, including the
// shapes of malformed input a file can contain. The scan must key on the spMk
// element name and its id attribute; matching any element, or any numeric
// attribute, is the failure this table describes.
func TestScanShapeMarkID(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		wantID uint32
		wantOK bool
	}{
		{"spMk with id", `<pc:spMk xmlns:pc="u" id="42"/>`, 42, true},
		{"spMk nested in sldMkLst", `<pc:sldMkLst xmlns:pc="u"><pc:docMk/><pc:sldMk cId="99" sldId="256"/><pc:spMk id="7"/></pc:sldMkLst>`, 7, true},
		{"sldMk only", `<pc:sldMkLst xmlns:pc="u"><pc:sldMk cId="99" sldId="256"/></pc:sldMkLst>`, 0, false},
		{"spMk without id", `<pc:spMk xmlns:pc="u" creationId="{X}"/>`, 0, false},
		{"no markers", `<pc:docMk xmlns:pc="u"/>`, 0, false},
		{"empty", ``, 0, false},
		{"malformed xml", `<pc:spMk id="3"`, 0, false},
		{"non-numeric id", `<pc:spMk xmlns:pc="u" id="abc"/>`, 0, false},
		{"negative id", `<pc:spMk xmlns:pc="u" id="-1"/>`, 0, false},
		{"overflowing id", `<pc:spMk xmlns:pc="u" id="99999999999999"/>`, 0, false},
		{"first spMk wins", `<pc:sldMkLst xmlns:pc="u"><pc:spMk id="11"/><pc:spMk id="22"/></pc:sldMkLst>`, 11, true},
		// The element name must match exactly: a similarly-named marker is not
		// a shape anchor.
		{"grpSpMk is not spMk", `<pc:grpSpMk xmlns:pc="u" id="9"/>`, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := scanShapeMarkID([]byte(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("scanShapeMarkID(%q) ok = %v, want %v (id %d)", tc.raw, ok, tc.wantOK, id)
			}
			if id != tc.wantID {
				t.Errorf("scanShapeMarkID(%q) = %d, want %d", tc.raw, id, tc.wantID)
			}
		})
	}
}

// FuzzScanShapeMarkID feeds arbitrary bytes to the anchor-marker scan, which
// reads raw children of a comment part straight from an untrusted file. The
// invariants are that it never panics, that it is deterministic, and that a
// positive result implies the input really does contain an spMk element
// carrying an id attribute — so a scan that stopped checking the element name
// or the attribute name is caught rather than merely "not crashing".
func FuzzScanShapeMarkID(f *testing.F) {
	f.Add([]byte(`<pc:spMk xmlns:pc="u" id="42"/>`))
	f.Add([]byte(`<pc:sldMkLst xmlns:pc="u"><pc:docMk/><pc:sldMk cId="1" sldId="256"/><pc:spMk id="7"/></pc:sldMkLst>`))
	f.Add([]byte(`<pc:spMk id="-1"/>`))
	f.Add([]byte(`<pc:spMk id="99999999999999"/>`))
	f.Add([]byte(`<pc:grpSpMk id="9"/>`))
	f.Add([]byte(``))
	f.Add([]byte(`<`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		id, ok := scanShapeMarkID(raw)

		// Deterministic: the same bytes must always yield the same answer.
		if id2, ok2 := scanShapeMarkID(raw); id2 != id || ok2 != ok {
			t.Fatalf("scanShapeMarkID is not deterministic: (%d,%v) then (%d,%v)", id, ok, id2, ok2)
		}

		if !ok {
			if id != 0 {
				t.Fatalf("scanShapeMarkID reported id %d alongside ok=false", id)
			}
			return
		}

		// A hit implies a well-formed-enough document containing an spMk start
		// element with an id attribute. Re-scan independently rather than
		// trusting a substring check.
		if !hasSpMkWithID(raw) {
			t.Fatalf("scanShapeMarkID returned (%d, true) for input with no spMk/@id: %q", id, raw)
		}
	})
}

// hasSpMkWithID reports whether raw contains an spMk start element carrying an
// id attribute, decoded independently of the function under test.
func hasSpMkWithID(raw []byte) bool {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "spMk" {
			continue
		}
		for _, at := range se.Attr {
			if at.Name.Local == "id" {
				return true
			}
		}
	}
}
