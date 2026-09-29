package pptx

import (
	"bytes"
	"errors"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// A slide child named ":" is well-formed XML that Go reads (as the local name
// ":") and PowerPoint does not. The contract is the one docx already keeps for
// such names: the deck opens, an untouched slide passes through byte for byte,
// and a save that has to rewrite the slide refuses with ErrUnwritableName. It
// used to rewrite the child as "<ns1::/>" and save a slide nothing could read
// back (FuzzPptxSlidePart, nightly of 2026-09-29).
func TestSlideChildWithNonQNameRefusesAnEditedSave(t *testing.T) {
	const part = "ppt/slides/slide1.xml"
	p := Create()
	p.AddSlide()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	deck := rewriteZipPart(t, data, part, func(b []byte) []byte {
		return bytes.Replace(b, []byte("</p:spTree>"), []byte("<:/></p:spTree>"), 1)
	})
	src := zipPart(t, deck, part)
	if !bytes.Contains(src, []byte("<:/>")) {
		t.Fatal("fixture rewrite did not apply")
	}

	open := func() *Presentation {
		t.Helper()
		p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
		if err != nil {
			t.Fatalf("a deck with a non-QName slide child no longer opens: %v", err)
		}
		return p
	}

	// Untouched: the slide's bytes are the source's.
	out, err := open().SaveBytes()
	if err != nil {
		t.Fatalf("an untouched deck no longer saves: %v", err)
	}
	if got := zipPart(t, out, part); !bytes.Equal(got, src) {
		t.Errorf("untouched slide changed on save\nwant: %s\n got: %s", src, got)
	}

	// Edited: the slide must be rewritten, and the name cannot be.
	edited := open()
	edited.Slides()[0].AddTextBox().TextFrame().SetText("edit")
	if _, err := edited.SaveBytes(); !errors.Is(err, xmlb.ErrUnwritableName) {
		t.Fatalf("edited save = %v, want ErrUnwritableName", err)
	}
}
