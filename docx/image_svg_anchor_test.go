package docx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipReader(data []byte) (*zip.Reader, error) {
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

const tinySVG = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="red"/></svg>`

// docPart returns a named part from a saved document, and also the full set of
// part names.
func docParts(t *testing.T, data []byte) (map[string]string, []string) {
	t.Helper()
	zr, err := zipReader(data)
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	parts := map[string]string{}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		_ = rc.Close()
		parts[f.Name] = b.String()
	}
	return parts, names
}

func TestAddSVGImageInline(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()
	img, err := r.AddImageFromBytes([]byte(tinySVG), "image/svg+xml")
	if err != nil {
		t.Fatalf("AddImageFromBytes svg: %v", err)
	}
	img.SetSize(72, 72)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, names := docParts(t, data)

	// Two media parts: the raster fallback and the SVG.
	hasSVG, hasPNG := false, false
	for _, n := range names {
		if strings.HasPrefix(n, "word/media/") && strings.HasSuffix(n, ".svg") {
			hasSVG = true
		}
		if strings.HasPrefix(n, "word/media/") && strings.HasSuffix(n, ".png") {
			hasPNG = true
		}
	}
	if !hasSVG || !hasPNG {
		t.Fatalf("expected both svg and png media parts: svg=%v png=%v names=%v", hasSVG, hasPNG, names)
	}

	// The drawing must carry the svgBlip extension referencing the SVG, with
	// a real raster r:embed as the primary.
	body := parts["word/document.xml"]
	if !strings.Contains(body, "asvg:svgBlip") {
		t.Errorf("document.xml missing svgBlip extension:\n%s", body)
	}
	if !strings.Contains(body, svgBlipExtURI) {
		t.Error("svgBlip extension URI missing")
	}
	// The content type for svg must be declared (default extension or override).
	ct := parts["[Content_Types].xml"]
	if !strings.Contains(ct, "svg") {
		t.Errorf("[Content_Types].xml does not declare svg:\n%s", ct)
	}

	if _, err := zipReader(data); err != nil {
		t.Fatalf("reopen zip: %v", err)
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen doc: %v", err)
	}
}

func TestAddSVGImageWithFallback(t *testing.T) {
	png := minimalPNG()
	doc := Create()
	r := doc.AddParagraph().AddRun()
	if _, err := r.AddSVGImage([]byte(tinySVG), png, "image/png"); err != nil {
		t.Fatalf("AddSVGImage: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	// The fallback bytes we supplied must be the raster part (not the
	// transparent default).
	found := false
	for name, content := range parts {
		if strings.HasPrefix(name, "word/media/") && strings.HasSuffix(name, ".png") {
			if content == string(png) {
				found = true
			}
		}
	}
	if !found {
		t.Error("caller-supplied raster fallback not embedded")
	}

	// An SVG fallback content type must be rejected (it is not a raster).
	if _, err := r.AddSVGImage([]byte(tinySVG), []byte(tinySVG), "image/svg+xml"); err == nil {
		t.Error("expected error for svg fallback content type")
	}
}

func TestAddFloatingImage(t *testing.T) {
	doc := Create()
	r := doc.AddParagraph().AddRun()
	img, err := r.AddFloatingImageFromBytes(minimalPNG(), "image/png", Anchor{
		RelativeToPage: true,
		X:              72,
		Y:              144,
		BehindText:     true,
	})
	if err != nil {
		t.Fatalf("AddFloatingImageFromBytes: %v", err)
	}
	img.SetSize(100, 50)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	body := parts["word/document.xml"]

	for _, want := range []string{
		"<wp:anchor",
		`behindDoc="1"`,
		`relativeFrom="page"`,
		"<wp:wrapNone/>",
		// 72pt = 914400 EMU, 144pt = 1828800 EMU
		"<wp:posOffset>914400</wp:posOffset>",
		"<wp:posOffset>1828800</wp:posOffset>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("floating drawing missing %q:\n%s", want, body)
		}
	}
	// Required attributes must all be present (audit C146: don't omit them).
	for _, attr := range []string{"distT=", "distB=", "distL=", "distR=", "simplePos=", "locked=", "layoutInCell=", "allowOverlap="} {
		if !strings.Contains(body, attr) {
			t.Errorf("wp:anchor missing required attr %q", attr)
		}
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

func TestFloatingImageInFront(t *testing.T) {
	doc := Create()
	r := doc.AddParagraph().AddRun()
	if _, err := r.AddFloatingImageFromBytes(minimalPNG(), "image/png", Anchor{}); err != nil {
		t.Fatalf("AddFloatingImageFromBytes: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	body := parts["word/document.xml"]
	if !strings.Contains(body, `behindDoc="0"`) {
		t.Error("default anchor should be in front of text (behindDoc=0)")
	}
	if !strings.Contains(body, `relativeFrom="column"`) || !strings.Contains(body, `relativeFrom="paragraph"`) {
		t.Errorf("default anchor should be column/paragraph relative:\n%s", body)
	}
}

// TestSVGImageOnOpenedDocument: adding an SVG to an opened document must write
// both parts and declare the content type (the opened-doc save path).
func TestSVGImageOnOpenedDocument(t *testing.T) {
	base := Create()
	base.AddParagraphWithText("hello")
	baseData, err := base.SaveBytes()
	if err != nil {
		t.Fatalf("base SaveBytes: %v", err)
	}

	doc, err := OpenReader(bytes.NewReader(baseData), int64(len(baseData)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	r := doc.AddParagraph().AddRun()
	if _, err := r.AddImageFromBytes([]byte(tinySVG), "image/svg+xml"); err != nil {
		t.Fatalf("AddImageFromBytes svg on opened: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, names := docParts(t, data)
	hasSVGMedia := false
	for _, n := range names {
		if strings.HasPrefix(n, "word/media/") && strings.HasSuffix(n, ".svg") {
			hasSVGMedia = true
		}
	}
	if !hasSVGMedia {
		t.Fatalf("svg media part not written on opened doc: %v", names)
	}
	if !strings.Contains(parts["[Content_Types].xml"], "svg") {
		t.Errorf("opened-doc content types missing svg declaration:\n%s", parts["[Content_Types].xml"])
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}
