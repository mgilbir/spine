package opc

import (
	"bytes"
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
// empty-string <Override ContentType=""/>.
func TestCreatePart_NoEmptyOverride(t *testing.T) {
	w := NewWriter(&bytes.Buffer{})
	if err := w.WritePart("/xl/media/thing.dat", "", []byte("x")); err != nil {
		t.Fatalf("WritePart: %v", err)
	}
	if ctype, ok := w.ContentTypes.Overrides["/xl/media/thing.dat"]; ok {
		t.Errorf("empty content type registered an override = %q", ctype)
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
