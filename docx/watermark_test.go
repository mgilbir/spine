package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
)

// minimalTransparentPNGBytes is a 1x1 PNG used to exercise the image watermark
// path (reuses the package's fallback PNG).
var minimalTransparentPNGBytes = minimalTransparentPNG

// concatHeaderParts returns the concatenated uncompressed content of every
// /word/headerN.xml part in a saved .docx, so tests can inspect the emitted VML.
func concatHeaderParts(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	var sb strings.Builder
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "word/header") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		sb.Write(b)
	}
	return sb.String()
}

func TestSetTextWatermarkCreated(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("Body")
	if err := doc.SetTextWatermark("CONFIDENTIAL", WatermarkOptions{Diagonal: true}); err != nil {
		t.Fatalf("SetTextWatermark: %v", err)
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	wm := reopened.Watermark()
	if wm == nil {
		t.Fatal("expected a watermark, got nil")
	}
	if wm.Type != WatermarkText {
		t.Errorf("Type = %v, want WatermarkText", wm.Type)
	}
	if wm.Text != "CONFIDENTIAL" {
		t.Errorf("Text = %q, want %q", wm.Text, "CONFIDENTIAL")
	}
}

func TestSetTextWatermarkOnOpened(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = doc.Close() })

	if err := doc.SetTextWatermark("DRAFT", WatermarkOptions{Color: "FF0000"}); err != nil {
		t.Fatalf("SetTextWatermark: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	wm := reopened.Watermark()
	if wm == nil || wm.Type != WatermarkText || wm.Text != "DRAFT" {
		t.Fatalf("watermark = %+v, want text DRAFT", wm)
	}
	if !strings.Contains(concatHeaderParts(t, data), "#ff0000") {
		t.Error("expected the fill color to be present in the header VML")
	}
}

func TestSetImageWatermark(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("Body")
	if err := doc.SetImageWatermark(minimalTransparentPNGBytes, WatermarkOptions{}); err != nil {
		t.Fatalf("SetImageWatermark: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	wm := reopened.Watermark()
	if wm == nil || wm.Type != WatermarkImage {
		t.Fatalf("watermark = %+v, want image", wm)
	}
}

func TestRemoveWatermark(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("Body")
	if err := doc.SetTextWatermark("SECRET", WatermarkOptions{}); err != nil {
		t.Fatalf("SetTextWatermark: %v", err)
	}
	if !doc.RemoveWatermark() {
		t.Fatal("RemoveWatermark reported no removal")
	}
	if wm := doc.Watermark(); wm != nil {
		t.Fatalf("watermark still present after removal: %+v", wm)
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if wm := reopened.Watermark(); wm != nil {
		t.Fatalf("watermark survived save/reopen: %+v", wm)
	}
}

func TestSetTextWatermarkReplaces(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("Body")
	if err := doc.SetTextWatermark("FIRST", WatermarkOptions{}); err != nil {
		t.Fatalf("first SetTextWatermark: %v", err)
	}
	if err := doc.SetTextWatermark("SECOND", WatermarkOptions{}); err != nil {
		t.Fatalf("second SetTextWatermark: %v", err)
	}
	wm := doc.Watermark()
	if wm == nil || wm.Text != "SECOND" {
		t.Fatalf("watermark = %+v, want SECOND", wm)
	}
	// Only one watermark shape should remain across all headers.
	data, _ := doc.SaveBytes()
	if n := strings.Count(concatHeaderParts(t, data), "PowerPlusWaterMarkObject"); n != 1 {
		t.Errorf("watermark shape count = %d, want 1", n)
	}
}

// TestWatermarkByteIdentityUntouched verifies that reading the watermark of a
// document leaves every part byte-identical on a round-trip save.
func TestWatermarkByteIdentityUntouched(t *testing.T) {
	for _, path := range []string{"testdata/minimal.docx", "testdata/chart.docx", "testdata/svg_test.docx"} {
		doc, err := Open(path)
		if err != nil {
			t.Fatalf("Open %s: %v", path, err)
		}
		// Reading the watermark must not mark anything modified.
		_ = doc.Watermark()
		if len(doc.modifiedHdrFtrParts) != 0 {
			t.Errorf("%s: reading watermark marked %d parts modified", path, len(doc.modifiedHdrFtrParts))
		}
		tmp := filepath.Join(t.TempDir(), "rt.docx")
		if err := doc.Save(tmp); err != nil {
			_ = doc.Close()
			t.Fatalf("Save %s: %v", path, err)
		}
		_ = doc.Close()
		missing, extra, changed := testutil.CompareZipFiles(t, path, tmp)
		if len(missing)+len(extra)+len(changed) != 0 {
			t.Errorf("%s: not byte-identical (missing=%v extra=%v changed=%v)", path, missing, extra, changed)
		}
	}
}

// TestTextWatermarkDrawingMLEscapesCR verifies a carriage return in a DrawingML
// text watermark's w:t body is emitted as a &#xD; character reference rather
// than a raw CR. A raw CR in element content is normalized to a newline on
// reparse (XML §2.11), so only the character reference round-trips. Regression
// test for C349. The assertion targets the DrawingML choice's w:t specifically
// (the VML fallback carries the text in an attribute, which was already
// CR-escaped).
func TestTextWatermarkDrawingMLEscapesCR(t *testing.T) {
	doc := Create()
	if err := doc.SetTextWatermark("A\rB", WatermarkOptions{DrawingML: true}); err != nil {
		t.Fatalf("SetTextWatermark: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	headers := concatHeaderParts(t, data)
	if !strings.Contains(headers, `<w:t xml:space="preserve">A&#xD;B</w:t>`) {
		t.Errorf("DrawingML watermark w:t did not escape the CR as &#xD;; got %q", headers)
	}
}
