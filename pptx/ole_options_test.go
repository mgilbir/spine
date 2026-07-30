package pptx

import (
	"bytes"
	"image"
	"image/jpeg"
	"strings"
	"testing"
)

// oleTestPayload is a distinctive OLE payload so it is never confused with the
// preview image bytes.
var oleTestPayload = []byte("\xd0\xcf\x11\xe0OLE-OBJECT-PAYLOAD")

// olePreviewImage is a valid image that is NOT the placeholder AddOLEObject
// embeds by default, and deliberately NOT a PNG: the default preview content
// type is "image/png", so a JPEG preview is the only way to tell "the supplied
// content type was stored" from "the default happened to match".
func olePreviewImage() []byte {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 10, 8))
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// findMediaPart returns the /ppt/media part holding exactly data, or "".
func findMediaPart(p *Presentation, data []byte) (name, contentType string) {
	for n, part := range p.otherParts {
		if part == nil || !strings.HasPrefix(n, "/ppt/media/") {
			continue
		}
		if bytes.Equal(part.Data, data) {
			return n, part.ContentType
		}
	}
	return "", ""
}

// TestOLEObjectOptionsCombined applies every OLE option at once. Functional
// options fail by one writing another's field (a copy/pasted closure body), a
// mode that only works when it is the sole option, or a later default
// overwriting an explicit value — none of which a one-option-at-a-time test can
// see. Every option is therefore given a value distinguishable from both the
// default and from the other options' values.
func TestOLEObjectOptionsCombined(t *testing.T) {
	const (
		customCT   = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		previewCT  = "image/jpeg"
		customName = "Budget Workbook"
		progID     = "Excel.Sheet.12"
	)
	// Bounds deliberately differ from each other and from every default.
	const (
		bx, by = int64(111111), int64(222222)
		bw, bh = int64(3333333), int64(444444)
	)

	p := Create()
	s := p.AddSlide()
	preview := olePreviewImage()

	of, err := s.AddOLEObject(oleTestPayload, progID,
		WithOLEBounds(bx, by, bw, bh),
		WithOLEName(customName),
		WithOLEContentType(customCT),
		WithOLEPreviewImage(preview, previewCT),
		WithOLEShowAsIcon(true),
	)
	if err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}

	// WithOLEContentType must type the OBJECT part, not the preview part.
	part, ok := p.otherParts[of.partName]
	if !ok {
		t.Fatalf("OLE part %q was not stored", of.partName)
	}
	if part.ContentType != customCT {
		t.Errorf("OLE part content type = %q, want %q", part.ContentType, customCT)
	}
	if !bytes.Equal(part.Data, oleTestPayload) {
		t.Errorf("OLE part data = %q, want the payload", part.Data)
	}

	// WithOLEPreviewImage must embed the supplied bytes, typed with the
	// supplied image content type — not with the OLE content type.
	previewPart, previewPartCT := findMediaPart(p, preview)
	if previewPart == "" {
		t.Fatal("the supplied preview image was not embedded as a media part")
	}
	if previewPartCT != previewCT {
		t.Errorf("preview part content type = %q, want %q (the default is image/png, so this "+
			"fails when the option's content type is dropped)", previewPartCT, previewCT)
	}
	if previewPartCT == customCT {
		t.Error("WithOLEContentType leaked into the preview part's content type")
	}
	// The default placeholder must NOT also have been embedded.
	if n, _ := findMediaPart(p, minimalTransparentPNG); n != "" {
		t.Errorf("the default placeholder preview was embedded at %q despite WithOLEPreviewImage", n)
	}

	// WithOLEBounds and WithOLEName.
	if x, y := of.Position(); int64(x) != bx || int64(y) != by {
		t.Errorf("Position() = (%d,%d), want (%d,%d)", x, y, bx, by)
	}
	if w, h := of.Size(); int64(w) != bw || int64(h) != bh {
		t.Errorf("Size() = (%d,%d), want (%d,%d)", w, h, bw, bh)
	}
	if of.Name() != customName {
		t.Errorf("Name() = %q, want %q", of.Name(), customName)
	}

	// WithOLEShowAsIcon must reach the part.
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(slideXML, `showAsIcon="1"`) {
		t.Errorf("showAsIcon not emitted:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `progId="`+progID+`"`) {
		t.Errorf("progId not emitted:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `name="`+customName+`"`) {
		t.Errorf("the OLE name did not reach p:oleObj:\n%s", slideXML)
	}
	// The frame geometry written to the part must be the requested bounds.
	if !strings.Contains(slideXML, `imgW="3333333"`) || !strings.Contains(slideXML, `imgH="444444"`) {
		t.Errorf("OLE frame dimensions not emitted as 3333333/444444:\n%s", slideXML)
	}
}

// TestOLEObjectOptionDefaults pins the behaviour each option overrides, so the
// combined test above cannot pass merely because a default happened to equal
// the requested value.
func TestOLEObjectOptionDefaults(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	of, err := s.AddOLEObject(oleTestPayload, "Package")
	if err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	part := p.otherParts[of.partName]
	if part == nil {
		t.Fatalf("OLE part %q was not stored", of.partName)
	}
	if part.ContentType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Error("the default OLE content type already equals the value WithOLEContentType sets")
	}
	if n, _ := findMediaPart(p, minimalTransparentPNG); n == "" {
		t.Error("no default placeholder preview was embedded")
	}
	if n, _ := findMediaPart(p, olePreviewImage()); n != "" {
		t.Error("the default embedded the custom preview image")
	}
	if x, y := of.Position(); int64(x) != oleDefaultX || int64(y) != oleDefaultY {
		t.Errorf("default Position() = (%d,%d), want (%d,%d)", x, y, oleDefaultX, oleDefaultY)
	}
	if w, h := of.Size(); int64(w) != oleDefaultWidth || int64(h) != oleDefaultHeight {
		t.Errorf("default Size() = (%d,%d), want (%d,%d)", w, h, oleDefaultWidth, oleDefaultHeight)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), `showAsIcon=`) {
		t.Error("showAsIcon was emitted without WithOLEShowAsIcon")
	}

	// An empty preview falls back to the placeholder rather than embedding a
	// zero-length media part.
	p2 := Create()
	s2 := p2.AddSlide()
	if _, err := s2.AddOLEObject(oleTestPayload, "Package", WithOLEPreviewImage(nil, "")); err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	if n, _ := findMediaPart(p2, minimalTransparentPNG); n == "" {
		t.Error("an empty WithOLEPreviewImage did not fall back to the placeholder")
	}
	// Likewise an empty content type falls back to the generic OLE type.
	p3 := Create()
	s3 := p3.AddSlide()
	of3, err := s3.AddOLEObject(oleTestPayload, "Package", WithOLEContentType(""))
	if err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	if got := p3.otherParts[of3.partName].ContentType; got == "" {
		t.Error("an empty WithOLEContentType left the part untyped")
	}
}
