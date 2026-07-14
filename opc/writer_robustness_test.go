package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// C44: writing the same part's relationships twice must be rejected rather than
// emitting two zip entries with the same name.
func TestWritePartRelationships_RejectsDuplicate(t *testing.T) {
	w := NewWriter(&bytes.Buffer{})
	rels := []*Relationship{{ID: "rId1", Type: RelTypeOfficeDocument, Target: "x.xml", TargetMode: TargetModeInternal}}

	if err := w.WritePartRelationships("/x/part.xml", rels); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := w.WritePartRelationships("/x/part.xml", rels); err != ErrDuplicatePart {
		t.Errorf("second write = %v, want ErrDuplicatePart", err)
	}
}

// C45: a part written with an empty content type must not register an
// empty-string <Override ContentType=""/>; it relies on the Default mapping
// covering its extension instead.
func TestCreatePart_NoEmptyOverride(t *testing.T) {
	w := NewWriter(&bytes.Buffer{})
	if err := w.WritePart("/xl/media/thing.png", "", []byte("x")); err != nil {
		t.Fatalf("WritePart: %v", err)
	}
	if ctype, ok := w.ContentTypes.Overrides["/xl/media/thing.png"]; ok {
		t.Errorf("empty content type registered an override = %q", ctype)
	}
}

// C185: a part with an empty content type whose extension no Default mapping
// covers would be silently absent from [Content_Types].xml, producing an
// OPC-invalid package; it must be rejected at part-creation time instead.
func TestCreatePart_EmptyContentTypeNoDefault(t *testing.T) {
	w := NewWriter(&bytes.Buffer{})

	if err := w.WritePart("/xl/media/thing.dat", "", []byte("x")); !errors.Is(err, ErrInvalidContentType) {
		t.Errorf("WritePart with uncovered empty content type = %v, want ErrInvalidContentType", err)
	}

	// After registering a Default for the extension, the same part is fine.
	w.ContentTypes.SetDefault("dat", "application/octet-stream")
	if err := w.WritePart("/xl/media/thing.dat", "", []byte("x")); err != nil {
		t.Errorf("WritePart with covering default = %v, want nil", err)
	}
}

// A part preserved verbatim from a source package is exempt from the
// missing-content-type check: real-world packages carry junk entries (e.g.
// /[trash]/0000.dat) with no [Content_Types].xml entry, and round-tripping
// them must preserve that status exactly instead of failing the save.
func TestWritePreservedPart_EmptyContentTypeNoDefault(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WritePreservedPart("/[trash]/0000.dat", "", []byte("x")); err != nil {
		t.Fatalf("WritePreservedPart with uncovered empty content type = %v, want nil", err)
	}

	// No override may be registered for it either: the source package had no
	// entry, so the output must not grow one.
	if ctype, ok := w.ContentTypes.Overrides["/[trash]/0000.dat"]; ok {
		t.Errorf("preserved part with empty content type registered an override = %q", ctype)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("reopen zip: %v", err)
	}
	found := false
	for _, f := range zr.File {
		if f.Name == "[trash]/0000.dat" {
			found = true
		}
	}
	if !found {
		t.Error("preserved part missing from output zip")
	}
}

// A preserved part with a non-empty content type behaves exactly like
// WritePart: the type is registered when not already covered.
func TestWritePreservedPart_NonEmptyContentType(t *testing.T) {
	w := NewWriter(&bytes.Buffer{})
	if err := w.WritePreservedPart("/xl/custom.bin", "application/x-custom", []byte("x")); err != nil {
		t.Fatalf("WritePreservedPart: %v", err)
	}
	if got := w.ContentTypes.GetContentType("/xl/custom.bin"); got != "application/x-custom" {
		t.Errorf("content type = %q, want %q", got, "application/x-custom")
	}
}

// C207: adding a package-level relationship after Close must fail — the
// package-level .rels part has already been written, so the relationship
// could never be persisted and its r:id would silently dangle.
func TestAddRelationship_AfterClose(t *testing.T) {
	w := NewWriter(&bytes.Buffer{})
	if err := w.WritePart("/test.xml", "application/xml", []byte("<test/>")); err != nil {
		t.Fatalf("WritePart: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "test.xml", TargetModeInternal); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("AddRelationship after Close = %v, want ErrPackageClosed", err)
	}
}

// C206: WriteRawFile must not emit an absolute-path zip entry when given a
// leading-slash name; many zip consumers reject such entries.
func TestWriteRawFile_TrimsLeadingSlash(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteRawFile("/leading/slash.xml", []byte("<x/>")); err != nil {
		t.Fatalf("WriteRawFile: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if strings.HasPrefix(f.Name, "/") {
			t.Errorf("zip entry has absolute path: %q", f.Name)
		}
	}
	found := false
	for _, n := range names {
		if n == "leading/slash.xml" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected entry %q, got %v", "leading/slash.xml", names)
	}
}

// C49: a source file with a duplicate <Default> re-emits it only once.
func TestContentTypes_DedupDefault(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Types xmlns="` + ContentTypesNamespace + `">` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`</Types>`
	ct, err := UnmarshalContentTypes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := ct.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(out), `Extension="xml"`); n != 1 {
		t.Errorf("duplicate Default re-emitted %d times, want 1:\n%s", n, out)
	}
}

// C50: override lookup is case-insensitive, matching OPC part-name semantics.
func TestContentTypes_CaseInsensitiveOverride(t *testing.T) {
	ct := NewContentTypes()
	ct.SetOverride("/xl/Worksheets/Sheet1.xml", ContentTypeWorksheet)
	if got := ct.GetContentType("/xl/worksheets/sheet1.xml"); got != ContentTypeWorksheet {
		t.Errorf("case-insensitive override lookup = %q, want %q", got, ContentTypeWorksheet)
	}
}

// failingWriter fails every write, simulating a full or broken disk.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

// C54: a failure during Close's metadata writes must still surface as an
// error (with the underlying zip writer closed rather than the stream
// abandoned mid-entry), and the writer must stay closed afterwards.
func TestClose_MetadataWriteFailureStillClosesZip(t *testing.T) {
	w := NewWriter(failingWriter{})
	w.Properties = &CoreProperties{Title: "T"}

	err := w.Close()
	if err == nil {
		t.Fatal("Close() with failing writer returned nil error")
	}
	if errors.Is(err, ErrPackageClosed) {
		t.Fatalf("Close() error = %v, want the underlying write failure", err)
	}
	if !strings.Contains(err.Error(), "simulated write failure") {
		t.Errorf("Close() error = %v, want it to carry the underlying write failure", err)
	}

	// The writer is closed despite the failure.
	if err := w.Close(); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("second Close() error = %v, want ErrPackageClosed", err)
	}
	if _, err := w.CreatePart("/x.xml", "application/xml", CompressionDeflate); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("CreatePart after failed Close error = %v, want ErrPackageClosed", err)
	}
}

// C54: Abort discards the package without emitting metadata, marks the writer
// closed, and makes every subsequent call fail with ErrPackageClosed.
func TestAbort_DiscardsWithoutMetadata(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WritePart("/test/part.xml", "application/xml", []byte("<root/>")); err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := w.Abort(); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}

	// No metadata entries were emitted.
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("reading aborted output: %v", err)
	}
	for _, f := range zr.File {
		switch f.Name {
		case "[Content_Types].xml", "_rels/.rels", "docProps/core.xml", "docProps/app.xml":
			t.Errorf("Abort() emitted metadata entry %s", f.Name)
		}
	}

	// Subsequent calls error.
	if _, err := w.CreatePart("/x.xml", "application/xml", CompressionDeflate); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("CreatePart after Abort: error = %v, want ErrPackageClosed", err)
	}
	if err := w.Close(); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("Close after Abort: error = %v, want ErrPackageClosed", err)
	}
	if err := w.Abort(); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("second Abort: error = %v, want ErrPackageClosed", err)
	}
}

// C123: the writer returned by CreatePart is invalidated by the next part
// write; writing through it must error rather than silently interleave bytes
// into the following entry.
func TestCreatePart_InvalidatedWriterErrors(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	first, err := w.CreatePart("/a.xml", "application/xml", CompressionDeflate)
	if err != nil {
		t.Fatalf("CreatePart() error = %v", err)
	}
	if _, err := first.Write([]byte("<a/>")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := w.CreatePart("/b.xml", "application/xml", CompressionDeflate); err != nil {
		t.Fatalf("CreatePart() error = %v", err)
	}
	if _, err := first.Write([]byte("stale")); err == nil {
		t.Error("Write to invalidated part writer succeeded, want error")
	}
}

// rawCT is a minimal source-formatted [Content_Types].xml used by the C46
// tests; the unusual lowercase encoding and lack of a standalone attribute
// make prolog preservation observable.
const rawCT = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`</Types>`

// readCTEntry returns the [Content_Types].xml bytes from a finished package.
func readCTEntry(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == "[Content_Types].xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("opening CT entry: %v", err)
			}
			defer func() { _ = rc.Close() }()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatalf("reading CT entry: %v", err)
			}
			return buf.Bytes()
		}
	}
	t.Fatal("package has no [Content_Types].xml")
	return nil
}

// C46: a part created after a raw [Content_Types].xml write must have its
// content type merged into the emitted file instead of silently vanishing.
func TestWriteRawFile_ContentTypes_RawThenCreate(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.ContentTypes = &ContentTypes{
		Defaults:  map[string]string{"rels": ContentTypeRelationships, "xml": "application/xml"},
		Overrides: map[string]string{},
	}
	if err := w.WriteRawFile("[Content_Types].xml", []byte(rawCT)); err != nil {
		t.Fatalf("WriteRawFile() error = %v", err)
	}
	// A part with an extension-less name needs an Override entry.
	if err := w.WritePart("/word/document.xml", ContentTypeDocument, []byte("<w/>")); err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	// And a part with a new extension needs a Default entry.
	w.ContentTypes.SetDefault("png", ContentTypePNG)
	if err := w.WritePart("/media/image1.png", "", []byte("fakepng")); err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	got := string(readCTEntry(t, buf.Bytes()))
	// The raw formatting survives...
	if !strings.HasPrefix(got, `<?xml version="1.0" encoding="utf-8"?>`) {
		t.Errorf("raw prolog not preserved:\n%s", got)
	}
	if !strings.Contains(got, `<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`) {
		t.Errorf("raw Default entry not preserved:\n%s", got)
	}
	// ...and the late registrations are merged in.
	if !strings.Contains(got, `PartName="/word/document.xml"`) {
		t.Errorf("override for part created after raw CT write missing:\n%s", got)
	}
	if !strings.Contains(got, `Extension="png"`) {
		t.Errorf("default registered after raw CT write missing:\n%s", got)
	}

	// The result must resolve every part when reopened.
	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if ct := r.ContentTypes.GetContentType("/word/document.xml"); ct != ContentTypeDocument {
		t.Errorf("reopened content type for /word/document.xml = %q, want %q", ct, ContentTypeDocument)
	}
	if ct := r.ContentTypes.GetContentType("/media/image1.png"); ct != ContentTypePNG {
		t.Errorf("reopened content type for /media/image1.png = %q, want %q", ct, ContentTypePNG)
	}
}

// C46: when the raw [Content_Types].xml is written after all parts (the
// consumer included every entry itself), the raw bytes win verbatim.
func TestWriteRawFile_ContentTypes_CreateThenRaw(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.ContentTypes = &ContentTypes{
		Defaults:  map[string]string{"rels": ContentTypeRelationships, "xml": "application/xml"},
		Overrides: map[string]string{},
	}
	if err := w.WritePart("/data/part.xml", "application/xml", []byte("<d/>")); err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := w.WriteRawFile("[Content_Types].xml", []byte(rawCT)); err != nil {
		t.Fatalf("WriteRawFile() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := string(readCTEntry(t, buf.Bytes())); got != rawCT {
		t.Errorf("raw CT not preserved verbatim:\ngot  %q\nwant %q", got, rawCT)
	}
}

// C46: registrations after the raw write that the raw bytes already cover do
// not force a re-marshal; the raw bytes still win verbatim.
func TestWriteRawFile_ContentTypes_CoveredRegistrationKeepsRaw(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.ContentTypes = &ContentTypes{
		Defaults:  map[string]string{"rels": ContentTypeRelationships},
		Overrides: map[string]string{},
	}
	if err := w.WriteRawFile("[Content_Types].xml", []byte(rawCT)); err != nil {
		t.Fatalf("WriteRawFile() error = %v", err)
	}
	// The raw bytes already carry Default xml=application/xml; registering the
	// same mapping afterwards must not perturb the verbatim output.
	w.ContentTypes.SetDefault("xml", "application/xml")
	if err := w.WritePart("/data/part.xml", "", []byte("<d/>")); err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := string(readCTEntry(t, buf.Bytes())); got != rawCT {
		t.Errorf("raw CT not preserved verbatim:\ngot  %q\nwant %q", got, rawCT)
	}
}
