package schemavalid_test

import (
	"archive/zip"
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/schemavalid"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// Every part this library authors from nothing must satisfy the schema that
// describes it.
//
// Only authored documents are checked, never a round-tripped one: a corpus
// document carries its own producer's schema sins, and re-reporting those as
// spine failures would bury the signal. What a save reproduces is the corpus
// test's business; what a save *invents* is this one's.

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func newValidator(t *testing.T) *schemavalid.Validator {
	t.Helper()
	root := repoRoot(t)
	name, haveValidator := schemavalid.Available()
	haveSchemas := schemavalid.SchemasPresent(root)
	require := os.Getenv("SPINE_REQUIRE_SCHEMA") != ""

	// Both halves are external to the repository, and each is missing for its
	// own reason, so the skip says which. Set SPINE_REQUIRE_SCHEMA where both
	// are supposed to be present — a machine that has bought the standard — and
	// a missing one fails instead of quietly checking nothing. CI has neither:
	// the schemas are not redistributable, so it cannot.
	switch {
	case haveValidator && haveSchemas:
		t.Logf("validating with %s", name)
	case require:
		t.Fatalf("SPINE_REQUIRE_SCHEMA is set but validator=%v schemas=%v", haveValidator, haveSchemas)
	case !haveSchemas:
		t.Skipf("ISO 29500 schemas not present under %v; they are copyrighted and not "+
			"redistributable, so they are gitignored — see spec/README.md to obtain them", schemavalid.SchemaDirs)
	default:
		t.Skip("no schema validator installed: `apt-get install libxml2-utils` (xmllint), or python3-lxml")
	}
	v, err := schemavalid.New(root)
	if err != nil {
		t.Fatalf("preparing the schema set: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

// gap records a conformance failure this suite reports rather than asserts,
// keyed by part name, with the substring that identifies it and a written
// reason. A gap that stops occurring fails the test: the entry has to be
// removed in the same commit that fixes it, so the list cannot rot into a
// record of things that used to be broken.
type gap struct{ part, match, why string }

// validatePackage validates every XML part of a saved package that the schema
// set describes.
func validatePackage(t *testing.T, v *schemavalid.Validator, pkg []byte, gaps ...gap) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("saved package is not a zip: %v", err)
	}
	checked := 0
	hit := make([]bool, len(gaps))
	for _, zf := range zr.File {
		if !describedBySchema(zf.Name) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", zf.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", zf.Name, err)
		}
		// A part whose root namespace the schema set does not define is out of
		// scope rather than passing: it is logged so the exclusion stays
		// visible instead of quietly widening.
		if ns := schemavalid.RootNamespace(data); ns != "" && !v.Describes(ns) {
			t.Logf("no schema for %s (root namespace %s); not checked", zf.Name, ns)
			continue
		}
		checked++
		err = v.Validate(data)
		if err == nil {
			continue
		}
		if i := matchGap(gaps, zf.Name, err.Error()); i >= 0 {
			hit[i] = true
			t.Logf("known gap in %s: %s\n\t%v", zf.Name, gaps[i].why, err)
			continue
		}
		t.Errorf("%s does not conform to its schema:\n\t%v", zf.Name, err)
	}
	for i, g := range gaps {
		if !hit[i] {
			t.Errorf("the recorded gap in %s (%s) no longer occurs — delete the entry", g.part, g.match)
		}
	}
	if checked == 0 {
		t.Error("no parts were checked, so this assertion proves nothing")
	}
}

func matchGap(gaps []gap, part, msg string) int {
	for i, g := range gaps {
		if g.part == part && strings.Contains(msg, g.match) {
			return i
		}
	}
	return -1
}

// describedBySchema reports whether the vendored schema set covers a part at
// all. The package layer (content types, relationships) and VML are vendored
// and therefore checked; a part excluded here is one no schema describes, not
// one allowed to be wrong.
func describedBySchema(name string) bool {
	if !strings.HasSuffix(name, ".xml") {
		return false
	}
	switch {
	case name == "docProps/core.xml":
		// Defined over Dublin Core, which is not vendored; see
		// schemavalid.skipSchemas.
		return false
	case strings.Contains(name, "/customXml/"):
		return false // arbitrary caller-supplied XML, by definition unmodeled
	}
	return true
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The documents the authored-output suites drive. They are package-level so the
// content-order check runs over exactly the same output as the schema check.
var docxCases = map[string]func(*testing.T) *docx.Document{
	"minimal": func(t *testing.T) *docx.Document {
		d := docx.Create()
		d.AddParagraph().SetText("hello")
		return d
	},
	"formatted text": func(t *testing.T) *docx.Document {
		d := docx.Create()
		p := d.AddParagraph()
		p.SetAlignment(docx.AlignmentCenter)
		r := p.AddRun()
		r.SetText("styled")
		r.SetBold(true)
		r.SetItalic(true)
		r.SetFontSize(14)
		r.SetColor("FF0000")
		d.AddParagraph().AddHyperlink("link", "https://example.com")
		return d
	},
	"table": func(t *testing.T) *docx.Document {
		d := docx.Create()
		tbl := d.AddTable(2, 3)
		rows := tbl.Rows()
		rows[0].Cells()[0].AddParagraph().SetText("a")
		rows[1].Cells()[2].AddParagraph().SetText("b")
		return d
	},
	"image": func(t *testing.T) *docx.Document {
		d := docx.Create()
		if _, err := d.AddParagraph().AddRun().AddImageFromBytes(testPNG(t), "image/png"); err != nil {
			t.Fatalf("AddImageFromBytes: %v", err)
		}
		return d
	},
	"text box": func(t *testing.T) *docx.Document {
		d := docx.Create()
		d.AddTextBox("boxed", docx.TextBoxOptions{})
		return d
	},
	"comment": func(t *testing.T) *docx.Document {
		d := docx.Create()
		p := d.AddParagraph()
		p.SetText("body")
		p.AddComment("Author", "a remark")
		return d
	},
	"headers and footers": func(t *testing.T) *docx.Document {
		d := docx.Create()
		d.AddParagraph().SetText("body")
		d.AddHeader(docx.HeaderDefault).AddParagraph().SetText("head")
		d.AddFooter(docx.FooterDefault).AddParagraph().SetText("foot")
		return d
	},
}

var xlsxCases = map[string]func(*testing.T) *xlsx.Workbook{
	"minimal": func(t *testing.T) *xlsx.Workbook {
		w := xlsx.Create()
		s, err := w.AddSheet("Data")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetCellValue("A1", "hello"); err != nil {
			t.Fatal(err)
		}
		return w
	},
	"typed values": func(t *testing.T) *xlsx.Workbook {
		w := xlsx.Create()
		s, err := w.AddSheet("Types")
		if err != nil {
			t.Fatal(err)
		}
		for ref, val := range map[string]any{
			"A1": "text", "A2": 42, "A3": 3.5, "A4": true,
		} {
			if err := s.SetCellValue(ref, val); err != nil {
				t.Fatal(err)
			}
		}
		return w
	},
	"column widths and sheet view": func(t *testing.T) *xlsx.Workbook {
		w := xlsx.Create()
		s, err := w.AddSheet("View")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetCellValue("A1", "wide"); err != nil {
			t.Fatal(err)
		}
		if err := s.SetColWidth(1, 24); err != nil {
			t.Fatal(err)
		}
		s.SetZoom(120)
		s.SetShowGridLines(false)
		s.SetTabColor("FF0000")
		return w
	},
	"multiple sheets": func(t *testing.T) *xlsx.Workbook {
		w := xlsx.Create()
		for _, n := range []string{"One", "Two", "Three"} {
			s, err := w.AddSheet(n)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetCellValue("A1", n); err != nil {
				t.Fatal(err)
			}
		}
		return w
	},
}

var pptxCases = map[string]func(*testing.T) *pptx.Presentation{
	"minimal": func(t *testing.T) *pptx.Presentation {
		p := pptx.Create()
		p.AddSlide()
		return p
	},
	"text box": func(t *testing.T) *pptx.Presentation {
		p := pptx.Create()
		s := p.AddSlide()
		tb := pptx.NewTextBox()
		tb.SetText("hello")
		if err := s.AddShape(tb); err != nil {
			t.Fatal(err)
		}
		return p
	},
	"speaker notes": func(t *testing.T) *pptx.Presentation {
		p := pptx.Create()
		s := p.AddSlide()
		if err := s.SetNotes("remember to breathe"); err != nil {
			t.Fatalf("SetNotes: %v", err)
		}
		return p
	},
	"image": func(t *testing.T) *pptx.Presentation {
		p := pptx.Create()
		s := p.AddSlide()
		if _, err := s.AddPictureFromBytes(testPNG(t), "image/png"); err != nil {
			t.Fatalf("AddPictureFromBytes: %v", err)
		}
		return p
	},
	"widescreen with several slides": func(t *testing.T) *pptx.Presentation {
		p := pptx.CreateWidescreen()
		for i := 0; i < 3; i++ {
			s := p.AddSlide()
			tb := pptx.NewTextBox()
			tb.SetText("slide")
			if err := s.AddShape(tb); err != nil {
				t.Fatal(err)
			}
		}
		return p
	},
}

// authoredPackages saves every case, keyed by format and name.
func authoredPackages(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for name, build := range docxCases {
		data, err := build(t).SaveBytes()
		if err != nil {
			t.Fatalf("docx %s: SaveBytes: %v", name, err)
		}
		out["docx/"+name] = data
	}
	for name, build := range xlsxCases {
		w := build(t)
		data, err := w.SaveBytes()
		if err != nil {
			t.Fatalf("xlsx %s: SaveBytes: %v", name, err)
		}
		_ = w.Close()
		out["xlsx/"+name] = data
	}
	for name, build := range pptxCases {
		data, err := build(t).SaveBytes()
		if err != nil {
			t.Fatalf("pptx %s: SaveBytes: %v", name, err)
		}
		out["pptx/"+name] = data
	}
	return out
}

func TestAuthoredPartsConformToSchema(t *testing.T) {
	v := newValidator(t)
	for name, pkg := range authoredPackages(t) {
		t.Run(name, func(t *testing.T) { validatePackage(t, v, pkg) })
	}
}
