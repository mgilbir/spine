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
