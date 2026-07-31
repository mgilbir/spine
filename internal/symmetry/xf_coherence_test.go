package symmetry_test

// Behavioural cross-format guards for the §5 "cross-format betrayal" class: a
// user who learns one format is misled by the next. crossformat_test.go records
// API *shapes*; this file pins the *behaviour* those shapes promise, because
// several of these findings were signature-identical and behaviour-divergent
// (bad image bytes, the impossible-state response) and a signature table cannot
// see that.

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// minimalPNG is a real 1x1 PNG: the add-time image validation (C441) reads the
// magic prefix, so tests may not stand in arbitrary bytes.
var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x60, 0x00, 0x02, 0x00,
	0x00, 0x05, 0x00, 0x01, 0xe2, 0x26, 0x05, 0x9b,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
	0xae, 0x42, 0x60, 0x82,
}

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="32"><rect width="64" height="32" fill="#f00"/></svg>`

// zipEntries returns every entry of a saved package keyed by name.
func zipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading saved package: %v", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		out[f.Name] = b
	}
	return out
}

// ---------------------------------------------------------------------------
// C386 — encrypted open exists in every format, not just docx.
// ---------------------------------------------------------------------------

// TestEncryptedOpenIsReachableInEveryFormat is the finding in one test: before
// it, xlsx.Open and pptx.Open both told the caller to open an encrypted file
// through opc, which returned an *opc.Reader that no public API could turn into
// a Workbook or a Presentation (openFromReader is unexported, and OpenReader
// takes an io.ReaderAt). The documented fallback — crypto.Decrypt — needs the
// two CFB streams, and the CFB parser is private to opc. Encryption is
// format-generic, so the gap was pure API. It is now closed by every format's
// ordinary open taking opc.WithPassword.
func TestEncryptedOpenIsReachableInEveryFormat(t *testing.T) {
	const password = "hunter2"

	t.Run("xlsx", func(t *testing.T) {
		wb := xlsx.Create()
		sheet, err := wb.AddSheet("Secret")
		if err != nil {
			t.Fatal(err)
		}
		cell, err := sheet.Cell("A1")
		if err != nil {
			t.Fatal(err)
		}
		cell.SetString("classified")

		var enc bytes.Buffer
		if err := wb.SaveEncryptedTo(&enc, password); err != nil {
			t.Fatalf("SaveEncryptedTo: %v", err)
		}
		// It really is an encrypted container: the plain reader refuses it.
		if _, err := xlsx.OpenReader(bytes.NewReader(enc.Bytes()), int64(enc.Len())); !errors.Is(err, opc.ErrEncrypted) {
			t.Fatalf("plain OpenReader on an encrypted workbook: got %v, want opc.ErrEncrypted", err)
		}

		got, err := xlsx.OpenReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()), opc.WithPassword(password))
		if err != nil {
			t.Fatalf("OpenReader with a password: %v", err)
		}
		s, err := got.SheetByName("Secret")
		if err != nil {
			t.Fatalf("SheetByName after encrypted open: %v", err)
		}
		c, err := s.Cell("A1")
		if err != nil {
			t.Fatal(err)
		}
		if c.String() != "classified" {
			t.Errorf("A1 = %q, want %q", c.String(), "classified")
		}
		if _, err := xlsx.OpenReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()), opc.WithPassword("wrong")); !errors.Is(err, crypto.ErrWrongPassword) {
			t.Errorf("wrong password: got %v, want crypto.ErrWrongPassword", err)
		}
	})

	t.Run("pptx", func(t *testing.T) {
		p := pptx.Create()
		slide := p.AddSlide()
		slide.AddTextBox().SetText("classified")

		var enc bytes.Buffer
		if err := p.SaveEncryptedTo(&enc, password); err != nil {
			t.Fatalf("SaveEncryptedTo: %v", err)
		}
		if _, err := pptx.OpenReader(bytes.NewReader(enc.Bytes()), int64(enc.Len())); !errors.Is(err, opc.ErrEncrypted) {
			t.Fatalf("plain OpenReader on an encrypted deck: got %v, want opc.ErrEncrypted", err)
		}

		got, err := pptx.OpenReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()), opc.WithPassword(password))
		if err != nil {
			t.Fatalf("OpenReader with a password: %v", err)
		}
		if n := len(got.Slides()); n != 1 {
			t.Fatalf("decrypted deck has %d slides, want 1", n)
		}
		if _, err := pptx.OpenReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()), opc.WithPassword("wrong")); !errors.Is(err, crypto.ErrWrongPassword) {
			t.Errorf("wrong password: got %v, want crypto.ErrWrongPassword", err)
		}
	})

	t.Run("docx", func(t *testing.T) {
		d := docx.Create()
		d.AddParagraphWithText("classified")

		var enc bytes.Buffer
		if err := d.SaveEncryptedTo(&enc, password); err != nil {
			t.Fatalf("SaveEncryptedTo: %v", err)
		}
		got, err := docx.OpenReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()), opc.WithPassword(password))
		if err != nil {
			t.Fatalf("OpenReader with a password: %v", err)
		}
		if n := len(got.Paragraphs()); n == 0 {
			t.Fatal("decrypted document has no paragraphs")
		}
	})
}

// TestEncryptedOpenOptionsAreReachableInEveryFormat pins the second half of
// C386: opc.ReaderOptions.AllowMissingDataIntegrity must be expressible from a
// format-level open, or the strictness knob added to common/crypto stays
// unreachable for anyone holding a .docx/.xlsx/.pptx. The strict default is
// what matters most, so that is what is asserted here.
func TestEncryptedOpenOptionsAreReachableInEveryFormat(t *testing.T) {
	var strict []opc.ReaderOption // no options beyond the password: integrity verification required
	for _, tc := range []struct {
		format string
		open   func(r io.ReaderAt, size int64, pw string, opts ...opc.ReaderOption) error
	}{
		{"docx", func(r io.ReaderAt, size int64, pw string, o ...opc.ReaderOption) error {
			_, err := docx.OpenReader(r, size, append([]opc.ReaderOption{opc.WithPassword(pw)}, o...)...)
			return err
		}},
		{"xlsx", func(r io.ReaderAt, size int64, pw string, o ...opc.ReaderOption) error {
			_, err := xlsx.OpenReader(r, size, append([]opc.ReaderOption{opc.WithPassword(pw)}, o...)...)
			return err
		}},
		{"pptx", func(r io.ReaderAt, size int64, pw string, o ...opc.ReaderOption) error {
			_, err := pptx.OpenReader(r, size, append([]opc.ReaderOption{opc.WithPassword(pw)}, o...)...)
			return err
		}},
	} {
		// A plain (unencrypted) zip is not a CFB container, so every format
		// reports the same failure through the same option-carrying entry point.
		junk := []byte("not a CFB container at all")
		if err := tc.open(bytes.NewReader(junk), int64(len(junk)), "pw", strict...); err == nil {
			t.Errorf("%s: the open accepted non-container bytes", tc.format)
		}
	}
}

// ---------------------------------------------------------------------------
// C387 — SVG auto-fallback in every format.
// ---------------------------------------------------------------------------

// TestSVGGetsARasterFallbackInEveryFormat: docx and xlsx already built the
// conformant dual-part structure (raster blip + asvg:svgBlip extension) from a
// single "here are SVG bytes" call. pptx emitted a bare <a:blip r:embed>
// pointing at the .svg part, which most PowerPoint versions render as nothing —
// and Validate passed. The machinery existed (Picture.SetSVGImageData); the
// discovery-path API bypassed it.
func TestSVGGetsARasterFallbackInEveryFormat(t *testing.T) {
	const svgExtURI = "{96DAC541-7B7A-43D3-8B79-37D633B846F1}"

	assertSVGPair := func(t *testing.T, format string, entries map[string][]byte, mediaDir string) {
		t.Helper()
		var svgPart, rasterPart string
		for name := range entries {
			if !strings.HasPrefix(name, mediaDir) {
				continue
			}
			switch {
			case strings.HasSuffix(name, ".svg"):
				svgPart = name
			case strings.HasSuffix(name, ".png"), strings.HasSuffix(name, ".jpeg"), strings.HasSuffix(name, ".jpg"):
				rasterPart = name
			}
		}
		if svgPart == "" {
			t.Errorf("%s: no .svg media part in the saved package", format)
		}
		if rasterPart == "" {
			t.Errorf("%s: SVG image saved with no raster fallback part — most viewers render nothing", format)
		}
		var body string
		for name, data := range entries {
			if strings.Contains(name, svgExtURI) {
				continue
			}
			if strings.Contains(string(data), svgExtURI) {
				body = string(data)
				break
			}
		}
		if body == "" {
			t.Fatalf("%s: no part carries the svgBlip extension %s", format, svgExtURI)
		}
		if !strings.Contains(body, "svgBlip") {
			t.Errorf("%s: extension present but no asvg:svgBlip element", format)
		}
	}

	t.Run("pptx", func(t *testing.T) {
		p := pptx.Create()
		s := p.AddSlide()
		pic, err := s.AddPictureFromBytes([]byte(testSVG), "image/svg+xml")
		if err != nil {
			t.Fatalf("AddPictureFromBytes(svg): %v", err)
		}
		// The frame is sized from the SVG root, not from the 1x1 fallback.
		if w, _ := pic.Size(); int64(w) != 64*9525 {
			t.Errorf("SVG picture width = %d EMU, want %d (64px at 96 DPI)", int64(w), 64*9525)
		}
		data, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}
		assertSVGPair(t, "pptx", zipEntries(t, data), "ppt/media/")
	})

	t.Run("docx", func(t *testing.T) {
		d := docx.Create()
		if _, err := d.AddParagraph().AddRun().AddImageFromBytes([]byte(testSVG), "image/svg+xml"); err != nil {
			t.Fatalf("AddImageFromBytes(svg): %v", err)
		}
		data, err := d.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}
		assertSVGPair(t, "docx", zipEntries(t, data), "word/media/")
	})

	t.Run("xlsx", func(t *testing.T) {
		wb := xlsx.Create()
		sheet, err := wb.AddSheet("Pics")
		if err != nil {
			t.Fatal(err)
		}
		if err := sheet.AddImage("A1", []byte(testSVG), xlsx.ImageOptions{}); err != nil {
			t.Fatalf("AddImage(svg): %v", err)
		}
		data, err := wb.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}
		assertSVGPair(t, "xlsx", zipEntries(t, data), "xl/media/")
	})
}

// ---------------------------------------------------------------------------
// C441 — bad image bytes fail at add time in every format.
// ---------------------------------------------------------------------------

// TestBadImageBytesFailAtAddTimeInEveryFormat: xlsx rejected garbage under an
// "image/png" content type; docx returned a nil error and saved a corrupt media
// part; pptx returned a nil error and saved, contradicting its own godoc
// ("Saving fails with an error when the data is empty or not a decodable
// image") — the save never failed either.
func TestBadImageBytesFailAtAddTimeInEveryFormat(t *testing.T) {
	garbage := []byte("this is definitely not an image")

	t.Run("docx", func(t *testing.T) {
		d := docx.Create()
		if _, err := d.AddParagraph().AddRun().AddImageFromBytes(garbage, "image/png"); err == nil {
			t.Fatal("AddImageFromBytes accepted non-image bytes under image/png")
		}
	})

	t.Run("pptx", func(t *testing.T) {
		p := pptx.Create()
		if _, err := p.AddSlide().AddPictureFromBytes(garbage, "image/png"); err == nil {
			t.Fatal("AddPictureFromBytes accepted non-image bytes under image/png")
		}
	})

	t.Run("xlsx", func(t *testing.T) {
		wb := xlsx.Create()
		sheet, err := wb.AddSheet("Pics")
		if err != nil {
			t.Fatal(err)
		}
		if err := sheet.AddImage("A1", garbage, xlsx.ImageOptions{}); err == nil {
			t.Fatal("AddImage accepted non-image bytes")
		}
	})

	// And the same real PNG is accepted by all three, so the rule rejects
	// garbage rather than everything.
	t.Run("valid PNG accepted everywhere", func(t *testing.T) {
		d := docx.Create()
		if _, err := d.AddParagraph().AddRun().AddImageFromBytes(minimalPNG, "image/png"); err != nil {
			t.Errorf("docx rejected a valid PNG: %v", err)
		}
		p := pptx.Create()
		if _, err := p.AddSlide().AddPictureFromBytes(minimalPNG, "image/png"); err != nil {
			t.Errorf("pptx rejected a valid PNG: %v", err)
		}
		wb := xlsx.Create()
		sheet, err := wb.AddSheet("Pics")
		if err != nil {
			t.Fatal(err)
		}
		if err := sheet.AddImage("A1", minimalPNG, xlsx.ImageOptions{}); err != nil {
			t.Errorf("xlsx rejected a valid PNG: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// C442 — the alt-text WRITE side is reachable, and spelled the same, everywhere.
// ---------------------------------------------------------------------------

func TestAltTextWriteIsReachableInEveryFormat(t *testing.T) {
	const alt = "A red square"

	t.Run("docx SetAltText", func(t *testing.T) {
		d := docx.Create()
		img, err := d.AddParagraph().AddRun().AddImageFromBytes(minimalPNG, "image/png")
		if err != nil {
			t.Fatal(err)
		}
		img.SetAltText(alt)
		if got := img.AltText(); got != alt {
			t.Errorf("AltText = %q, want %q", got, alt)
		}
	})

	t.Run("pptx SetAltText", func(t *testing.T) {
		p := pptx.Create()
		pic, err := p.AddSlide().AddPictureFromBytes(minimalPNG, "image/png")
		if err != nil {
			t.Fatal(err)
		}
		pic.SetAltText(alt)
		if got := pic.AltText(); got != alt {
			t.Errorf("AltText = %q, want %q", got, alt)
		}
	})

	// xlsx has no per-image handle (AddImage returns only an error and
	// xlsx.Image is a read view), so its settable path is the option field.
	// Before C442 there was none at all: alt text was readable and unwritable.
	t.Run("xlsx ImageOptions.AltText", func(t *testing.T) {
		f, ok := reflect.TypeOf(xlsx.ImageOptions{}).FieldByName("AltText")
		if !ok || f.Type.Kind() != reflect.String {
			t.Fatal("xlsx.ImageOptions has no AltText string field")
		}
		wb := xlsx.Create()
		sheet, err := wb.AddSheet("Pics")
		if err != nil {
			t.Fatal(err)
		}
		if err := sheet.AddImage("A1", minimalPNG, xlsx.ImageOptions{AltText: alt}); err != nil {
			t.Fatal(err)
		}
		imgs := sheet.Images()
		if len(imgs) != 1 {
			t.Fatalf("sheet has %d images, want 1", len(imgs))
		}
		if got := imgs[0].AltText(); got != alt {
			t.Errorf("Image.AltText = %q, want %q", got, alt)
		}
		data, err := wb.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for name, body := range zipEntries(t, data) {
			if strings.HasPrefix(name, "xl/drawings/") && strings.Contains(string(body), `descr="`+alt+`"`) {
				found = true
			}
		}
		if !found {
			t.Error("saved drawing carries no descr attribute for the image alt text")
		}
	})
}
