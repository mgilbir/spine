package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/opc"
)

// danglingRelCount returns the number of dangling-rel findings in a report.
func danglingRelCount(r validate.Report) int {
	n := 0
	for _, f := range r {
		if f.Code == validate.CodeDanglingRel {
			n++
		}
	}
	return n
}

func assertZipHasPrefix(t *testing.T, data []byte, prefix string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, prefix) {
			return
		}
	}
	t.Fatalf("no zip entry with prefix %q", prefix)
}

func TestAppendBodyContent(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("Dst first")
	dst.AddParagraphWithText("Dst second")

	src := Create()
	src.AddParagraphWithText("Src alpha")
	src.AddTable(2, 2)
	src.AddParagraphWithText("Src beta")

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}

	body := re.Body()
	for _, want := range []string{"Dst first", "Dst second", "Src alpha", "Src beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("merged body missing %q; got:\n%s", want, body)
		}
	}
	// Order: dst content precedes src content.
	if i, j := strings.Index(body, "Dst second"), strings.Index(body, "Src alpha"); i < 0 || j < 0 || i > j {
		t.Errorf("append order wrong: %q", body)
	}
	if len(re.Tables()) != 1 {
		t.Errorf("expected 1 table after append, got %d", len(re.Tables()))
	}
}

func TestAppendWithImage(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("Cover")

	src := Create()
	p := src.AddParagraph()
	r := p.AddRun()
	if _, err := r.AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("AddImageFromBytes: %v", err)
	}

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if len(dst.imageParts) != 1 {
		t.Fatalf("expected 1 image part carried, got %d", len(dst.imageParts))
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	assertZipHasPrefix(t, data, "word/media/")
}

func TestAppendWithNumbering(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("Intro")

	src := Create()
	def := src.Numbering().AddDefinition()
	def.SetLevel(0, NumberFormatDecimal, "%1.")
	list := def.ListStyle()
	src.AddParagraphWithText("Item one").SetListStyle(list, 0)

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	if !strings.Contains(re.Body(), "Item one") {
		t.Errorf("numbered item text missing")
	}
	assertZipHasPrefix(t, data, "word/numbering.xml")
}

// zipEntryText returns the concatenated text of every zip entry whose name
// starts with prefix.
func zipEntryText(t *testing.T, data []byte, prefix string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	var sb strings.Builder
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close() //nolint:errcheck
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		sb.Write(b)
	}
	return sb.String()
}

func TestAppendCarriesHeaderFooter(t *testing.T) {
	// Source has a first section (with a header and footer) ended by a section
	// break, then a second section. The paragraph-level section break carries
	// the header/footer references, so the parts must come across on append.
	src := Create()
	src.AddParagraphWithText("Section one body")
	src.AddHeader(HeaderDefault).AddParagraphWithText("SRC HEADER")
	src.AddFooter(FooterDefault).AddParagraphWithText("SRC FOOTER")
	src.AddSectionBreak()
	src.AddParagraphWithText("Section two body")

	dst := Create()
	dst.AddParagraphWithText("Dst intro")

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	assertZipHasPrefix(t, data, "word/header")
	assertZipHasPrefix(t, data, "word/footer")

	// The carried header/footer text survives.
	if !strings.Contains(zipEntryText(t, data, "word/header"), "SRC HEADER") {
		t.Errorf("carried header missing SRC HEADER text")
	}
	if !strings.Contains(zipEntryText(t, data, "word/footer"), "SRC FOOTER") {
		t.Errorf("carried footer missing SRC FOOTER text")
	}
}

func TestAppendRawNumbering(t *testing.T) {
	// Build a source with a numbered list, save, and reopen: the reopened source
	// keeps its numbering definitions as raw preserved XML (not the typed model),
	// which the append must still carry.
	seed := Create()
	seed.AddParagraphWithText("Seed intro")
	def := seed.Numbering().AddDefinition()
	def.SetLevel(0, NumberFormatDecimal, "%1.")
	list := def.ListStyle()
	seed.AddParagraphWithText("Raw item one").SetListStyle(list, 0)

	seedBytes, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	src, err := OpenReader(bytes.NewReader(seedBytes), int64(len(seedBytes)))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}

	dst := Create()
	dst.AddParagraphWithText("Dst intro")
	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	if !strings.Contains(re.Body(), "Raw item one") {
		t.Errorf("raw-numbered item text missing after append")
	}
	assertZipHasPrefix(t, data, "word/numbering.xml")
}

// TestAppendImportsChartRelationship builds a source document containing a chart
// (via the public API), saves and reopens it so the chart lives as a preserved
// package part, then appends it into a destination that carries its own image
// (so the source's chart rId could otherwise alias onto the destination's image
// rId). After the append the merged body's chart reference must resolve to an
// imported chart part rather than dangling or aliasing.
func TestAppendImportsChartRelationship(t *testing.T) {
	seed := Create()
	seed.AddParagraphWithText("Chart doc")
	c := chart.NewColumn().SetTitle("Sales").SetCategories([]string{"Q1", "Q2"})
	c.AddSeries("North", []float64{10, 20})
	if err := seed.AddChart(c, 5000000, 3000000); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	seedBytes, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	src, err := OpenReader(bytes.NewReader(seedBytes), int64(len(seedBytes)))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}

	// Destination with its own image, so it already occupies a relationship id.
	dst := Create()
	p := dst.AddParagraph()
	if _, err := p.AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("AddImageFromBytes: %v", err)
	}

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if n := danglingRelCount(dst.Validate()); n != 0 {
		t.Fatalf("after append: %d dangling-rel findings, want 0:\n%v", n, dst.Validate())
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if n := danglingRelCount(re.Validate()); n != 0 {
		t.Fatalf("reopened: %d dangling-rel findings, want 0:\n%v", n, re.Validate())
	}
	assertZipHasPrefix(t, data, "word/charts/")
	if got := len(re.Charts()); got != 1 {
		t.Fatalf("expected 1 resolvable chart after append, got %d", got)
	}
}

func TestAppendNil(t *testing.T) {
	dst := Create()
	dst.AddParagraphWithText("x")
	if err := dst.Append(nil); err != ErrNilDocument {
		t.Fatalf("err = %v, want ErrNilDocument", err)
	}
}
