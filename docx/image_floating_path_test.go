package docx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempFile drops bytes into the test's temp dir and returns the path.
func writeTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Run.AddFloatingImage is the file-path sibling of AddFloatingImageFromBytes.
// It owns two decisions the bytes form does not: reading the file, and deriving
// the content type from the extension. Both fail quietly — a wrong content type
// produces a package Word repairs, and a swallowed read error produces an empty
// image part.
func TestAddFloatingImage_FromPath(t *testing.T) {
	anchor := Anchor{RelativeToPage: true, X: 21, Y: 34, BehindText: true}

	t.Run("png", func(t *testing.T) {
		path := writeTempFile(t, "logo.png", tinyPNG)
		doc := Create()
		img, err := doc.AddParagraph().AddRun().AddFloatingImage(path, anchor)
		if err != nil {
			t.Fatalf("AddFloatingImage: %v", err)
		}
		if !img.Floating() {
			t.Error("AddFloatingImage produced an inline image")
		}
		if got := img.ContentType(); got != "image/png" {
			t.Errorf("ContentType() = %q, want image/png (derived from the .png extension)", got)
		}
		if got := img.Data(); string(got) != string(tinyPNG) {
			t.Errorf("the stored image bytes (%d) are not the file's bytes (%d)", len(got), len(tinyPNG))
		}

		reopened := saveAndReopen(t, doc)
		imgs := reopened.Images()
		if len(imgs) != 1 {
			t.Fatalf("got %d images after reopen, want 1", len(imgs))
		}
		if !imgs[0].Floating() {
			t.Error("the image reopened as inline: the anchor was not written")
		}
		if string(imgs[0].Data()) != string(tinyPNG) {
			t.Error("the image bytes did not survive the round trip")
		}
		parts, _ := docParts(t, mustSaveBytes(t, doc))
		main := parts["word/document.xml"]
		if !strings.Contains(main, "<wp:anchor") {
			t.Errorf("no wp:anchor in the drawing:\n%s", main)
		}
		if strings.Contains(main, "<wp:inline") {
			t.Errorf("the floating image was written as wp:inline:\n%s", main)
		}
	})

	t.Run("svg gets a raster fallback", func(t *testing.T) {
		path := writeTempFile(t, "vector.svg", []byte(tinySVG))
		doc := Create()
		img, err := doc.AddParagraph().AddRun().AddFloatingImage(path, anchor)
		if err != nil {
			t.Fatalf("AddFloatingImage(svg): %v", err)
		}
		if !img.Floating() {
			t.Error("the floating SVG was placed inline")
		}
		// An SVG must ship with a raster fallback part, or readers that cannot
		// render SVG show nothing at all.
		_, names := docParts(t, mustSaveBytes(t, doc))
		var svgs, rasters int
		for _, n := range names {
			switch {
			case strings.HasPrefix(n, "word/media/") && strings.HasSuffix(n, ".svg"):
				svgs++
			case strings.HasPrefix(n, "word/media/"):
				rasters++
			}
		}
		if svgs != 1 {
			t.Errorf("got %d svg media parts, want 1 (%v)", svgs, names)
		}
		if rasters != 1 {
			t.Errorf("got %d raster fallback parts, want 1 (%v)", rasters, names)
		}
	})

	t.Run("errors", func(t *testing.T) {
		doc := Create()
		run := doc.AddParagraph().AddRun()

		if _, err := run.AddFloatingImage(filepath.Join(t.TempDir(), "nope.png"), anchor); err == nil {
			t.Error("AddFloatingImage on a missing file returned no error")
		}
		// An extension the package has no content type for must be refused
		// rather than written with a guessed one.
		bad := writeTempFile(t, "notes.txt", []byte("not an image"))
		if _, err := run.AddFloatingImage(bad, anchor); err == nil {
			t.Error("AddFloatingImage on an unsupported extension returned no error")
		}
		// A .png that is not a PNG must be refused by the raster validation.
		lying := writeTempFile(t, "lying.png", []byte("this is not a png"))
		if _, err := run.AddFloatingImage(lying, anchor); err == nil {
			t.Error("AddFloatingImage accepted a file whose bytes are not the format its extension claims")
		}
	})
}

// Run.AddFloatingSVGImage is the only entry point that takes both a caller
// supplied raster fallback and an anchor. Anything that dropped the anchor
// (delegating to AddSVGImage) or dropped the fallback would still produce a
// loadable document.
func TestAddFloatingSVGImage(t *testing.T) {
	// A byte-distinct fallback: the package's own transparent placeholder is a
	// 1x1 PNG too, so passing tinyPNG could not tell "used the caller's bytes"
	// apart from "silently substituted the placeholder".
	wantFallback := taggedPNG("CALLER-FALLBACK")

	doc := Create()
	img, err := doc.AddParagraph().AddRun().AddFloatingSVGImage(
		[]byte(tinySVG), wantFallback, "image/png",
		Anchor{RelativeToPage: true, X: 12, Y: 24})
	if err != nil {
		t.Fatalf("AddFloatingSVGImage: %v", err)
	}
	if !img.Floating() {
		t.Error("AddFloatingSVGImage produced an inline image; the anchor was dropped")
	}

	parts, names := docParts(t, mustSaveBytes(t, doc))
	main := parts["word/document.xml"]
	if !strings.Contains(main, "<wp:anchor") {
		t.Errorf("no wp:anchor in the drawing:\n%s", main)
	}
	// The svgBlip extension is what tells Word the SVG is the real image and
	// the raster is only a fallback.
	if !strings.Contains(main, "svgBlip") {
		t.Errorf("the drawing carries no svgBlip extension:\n%s", main)
	}

	var svg, fallback string
	for _, n := range names {
		if !strings.HasPrefix(n, "word/media/") {
			continue
		}
		if strings.HasSuffix(n, ".svg") {
			svg = parts[n]
		} else {
			fallback = parts[n]
		}
	}
	if svg != tinySVG {
		t.Errorf("the stored svg part is not the supplied svg data:\n%s", svg)
	}
	// The caller's fallback must be used, not the package's transparent
	// placeholder.
	if fallback != string(wantFallback) {
		t.Error("the raster fallback part is not the caller-supplied fallback data; the placeholder was substituted")
	}

	reopened := saveAndReopen(t, doc)
	imgs := reopened.Images()
	if len(imgs) != 1 {
		t.Fatalf("got %d images after reopen, want 1", len(imgs))
	}
	if !imgs[0].Floating() {
		t.Error("the floating SVG reopened as inline")
	}
}

// Document.SetSectionImageWatermark scopes an image watermark to one section's
// headers. Its whole reason to exist, next to SetImageWatermark, is that it must
// *not* stamp the other sections, and its media relationship has to be
// registered in each header part's own rels rather than the main part's.
func TestSetSectionImageWatermark(t *testing.T) {
	doc := Create()
	doc.AddParagraph().SetText("first section")
	doc.AddSectionBreak()
	doc.AddParagraph().SetText("second section")

	secs := doc.Sections()
	if len(secs) != 2 {
		t.Fatalf("got %d sections, want 2", len(secs))
	}
	// Give the *other* section a watermark of its own first, so both sections
	// own a header part. Without that, only one header exists and "the image
	// watermark landed in exactly one header" would hold no matter how badly
	// the section scoping worked.
	if err := doc.SetSectionTextWatermark(secs[1], "OTHER-SECTION", WatermarkOptions{}); err != nil {
		t.Fatalf("SetSectionTextWatermark: %v", err)
	}
	if err := doc.SetSectionImageWatermark(secs[0], tinyPNG, WatermarkOptions{}); err != nil {
		t.Fatalf("SetSectionImageWatermark: %v", err)
	}

	parts, names := docParts(t, mustSaveBytes(t, doc))

	var headers, stamped, textOnly []string
	for _, n := range names {
		if !strings.HasPrefix(n, "word/header") {
			continue
		}
		headers = append(headers, n)
		switch {
		case strings.Contains(parts[n], "WordPictureWatermark"):
			stamped = append(stamped, n)
		case strings.Contains(parts[n], "PowerPlusWaterMarkObject"):
			textOnly = append(textOnly, n)
		}
	}
	if len(headers) != 2 {
		t.Fatalf("got %d header parts (%v), want 2 — the fixture cannot show a section leak", len(headers), headers)
	}
	if len(stamped) != 1 {
		t.Fatalf("the image watermark appears in %d header parts (%v), want exactly 1 — the section scope leaked", len(stamped), stamped)
	}
	// The other section's header must be untouched: still its text watermark,
	// and no image watermark stamped over it.
	if len(textOnly) != 1 {
		t.Errorf("the other section's text watermark was replaced: %v", textOnly)
	}
	if len(textOnly) == 1 && strings.Contains(parts[textOnly[0]], "WordPictureWatermark") {
		t.Errorf("%s carries both watermarks", textOnly[0])
	}

	// The image relationship must live in that header's own .rels: an r:id
	// resolved against document.xml.rels is a dangling reference from the
	// header's point of view.
	relsName := strings.Replace(stamped[0], "word/", "word/_rels/", 1) + ".rels"
	rels, ok := parts[relsName]
	if !ok {
		t.Fatalf("the stamped header has no relationship part %s (%v)", relsName, names)
	}
	if !strings.Contains(rels, "/image") {
		t.Errorf("%s carries no image relationship:\n%s", relsName, rels)
	}
	var media int
	for _, n := range names {
		if strings.HasPrefix(n, "word/media/") {
			media++
		}
	}
	if media != 1 {
		t.Errorf("got %d media parts, want 1", media)
	}

	// It survives a round trip, still scoped to the one header.
	roundTripped, _ := docParts(t, mustSaveBytes(t, saveAndReopen(t, doc)))
	if !strings.Contains(roundTripped[stamped[0]], "WordPictureWatermark") {
		t.Errorf("the image watermark did not survive a save/reopen in %s", stamped[0])
	}
	if len(textOnly) == 1 && strings.Contains(roundTripped[textOnly[0]], "WordPictureWatermark") {
		t.Errorf("after the round trip %s also carries the image watermark", textOnly[0])
	}
}

// TestSetSectionImageWatermark_ReadsBack checks the single-section case where
// Document.Watermark is unambiguous, so the classifier is exercised too.
func TestSetSectionImageWatermark_ReadsBack(t *testing.T) {
	doc := Create()
	doc.AddParagraph().SetText("only section")
	if err := doc.SetSectionImageWatermark(doc.DefaultSection(), tinyPNG, WatermarkOptions{}); err != nil {
		t.Fatalf("SetSectionImageWatermark: %v", err)
	}
	wm := saveAndReopen(t, doc).Watermark()
	if wm == nil {
		t.Fatal("Watermark() found nothing after the round trip")
	}
	if wm.Type != WatermarkImage {
		t.Errorf("Watermark().Type = %v, want WatermarkImage (%v)", wm.Type, WatermarkImage)
	}
}

func TestSetSectionImageWatermark_Errors(t *testing.T) {
	doc := Create()
	doc.AddParagraph().SetText("x")
	sec := doc.DefaultSection()

	if err := doc.SetSectionImageWatermark(nil, tinyPNG, WatermarkOptions{}); err == nil {
		t.Error("SetSectionImageWatermark(nil section) returned no error")
	}
	if err := doc.SetSectionImageWatermark(sec, nil, WatermarkOptions{}); err == nil {
		t.Error("SetSectionImageWatermark with no image data returned no error")
	}
	if err := doc.SetSectionImageWatermark(sec, []byte("not an image"), WatermarkOptions{}); err == nil {
		t.Error("SetSectionImageWatermark with undecodable image data returned no error")
	}
}
