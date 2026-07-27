package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

// relsXML builds a package-relationships part carrying exactly rels.
func relsXML(rels ...string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n")
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, r := range rels {
		b.WriteString(r)
	}
	b.WriteString(`</Relationships>`)
	return []byte(b.String())
}

const officeDocRel = `<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>`

// zipEntries lists the entry names of a written package.
func zipEntries(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	names := make([]string, 0, len(zr.File))
	for _, zf := range zr.File {
		names = append(names, zf.Name)
	}
	return names
}

// --- C377: a metadata part must never outlive its relationship -------------

// TestCloseRefusesOrphanCorePropertiesAfterPreservedRels is the regression
// test for C377. writeCoreProperties wrote docProps/core.xml, registered its
// content type and appended a package relationship; writeRelationships then
// returned early because /_rels/.rels had been preserved, silently discarding
// that relationship. The package came out with core.xml present, a
// content-type override for it, and nothing pointing at it — reopening yielded
// Properties = nil.
func TestCloseRefusesOrphanCorePropertiesAfterPreservedRels(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WritePreservedPart(packageRelsPartName, ContentTypeRelationships, relsXML(officeDocRel)); err != nil {
		t.Fatalf("WritePreservedPart(.rels) error = %v", err)
	}
	// Only now are the properties set — the relationships part is already in
	// the zip stream and cannot be amended.
	w.Properties = &CoreProperties{Title: "orphan"}

	err := w.Close()
	if err == nil {
		t.Fatal("Close() returned nil; expected it to refuse writing an unreachable docProps/core.xml (C377)")
	}
	for _, want := range []string{"docProps/core.xml", packageRelsPartName} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Close() error = %v, want it to name %q", err, want)
		}
	}

	// Nothing must have been emitted for the part it refused to write: no zip
	// entry and no content-type override claiming a part that is not there.
	for _, name := range zipEntries(t, buf.Bytes()) {
		if strings.EqualFold(name, "docProps/core.xml") {
			t.Error("Close() refused the relationship but emitted the orphan part anyway")
		}
	}
	if ct := w.ContentTypes.GetContentType("/docProps/core.xml"); ct == ContentTypeCoreProps {
		t.Error("Close() registered a content-type override for a part it did not write")
	}
}

// TestPreservedRelsGainMetadataRelationship is the other half of C377: with
// the properties set before the relationships part is handed over, the missing
// relationship is injected into the preserved bytes and the saved package is
// readable.
func TestPreservedRelsGainMetadataRelationship(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Properties = &CoreProperties{Title: "reachable"}

	if err := w.WritePreservedPart(packageRelsPartName, ContentTypeRelationships, relsXML(officeDocRel)); err != nil {
		t.Fatalf("WritePreservedPart(.rels) error = %v", err)
	}
	if err := w.WritePart("/word/document.xml", ContentTypeDocument, []byte("<w/>")); err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if r.GetFile("/docProps/core.xml") == nil {
		t.Fatal("docProps/core.xml was not written")
	}
	if r.Properties == nil {
		t.Fatal("Properties = nil after reopening: the core relationship is missing (C377)")
	}
	if r.Properties.Title != "reachable" {
		t.Errorf("Properties.Title = %q, want %q", r.Properties.Title, "reachable")
	}
	if got := len(r.GetRelationshipsByType(RelTypeCore)); got != 1 {
		t.Errorf("core relationships = %d, want exactly 1", got)
	}
}

// TestPreservedRelsUnchangedWhenNothingIsNeeded pins the byte-identity side of
// the same path: a preserved relationships part that already covers everything
// Close will write goes out untouched.
func TestPreservedRelsUnchangedWhenNothingIsNeeded(t *testing.T) {
	source := relsXML(officeDocRel,
		`<Relationship Id="rId2" Type="`+RelTypeCore+`" Target="docProps/core.xml"/>`)

	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Properties = &CoreProperties{Title: "t"}
	if err := w.WritePreservedPart(packageRelsPartName, ContentTypeRelationships, source); err != nil {
		t.Fatalf("WritePreservedPart(.rels) error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	got, err := r.GetFile(packageRelsPartName).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll(.rels) error = %v", err)
	}
	if !bytes.Equal(got, source) {
		t.Errorf("preserved .rels changed:\n got %s\nwant %s", got, source)
	}
	if n := len(r.GetRelationshipsByType(RelTypeCore)); n != 1 {
		t.Errorf("core relationships = %d, want exactly 1 (no duplicate appended)", n)
	}
}

// --- C394: relationship ids must come from the exported slice --------------

// TestCloseDoesNotMintCollidingRelationshipIDs is the regression test for
// C394: nextRelID was a private counter that could not see a Relationships
// slice the caller assigned — as opc's own writeSignedPackage does — so Close
// handed out rIds already in use.
func TestCloseDoesNotMintCollidingRelationshipIDs(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Relationships = []*Relationship{{
		ID:         "rId1",
		Type:       RelTypeOfficeDocument,
		Target:     "word/document.xml",
		TargetMode: TargetModeInternal,
	}}
	w.Properties = &CoreProperties{Title: "t"}
	w.ExtendedProperties = &ExtendedProperties{Application: "spine"}
	w.CustomProperties = &CustomProperties{}
	if err := w.CustomProperties.Set("k", "v"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := w.WritePart("/word/document.xml", ContentTypeDocument, []byte("<w/>")); err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	seen := map[string]string{}
	for _, rel := range r.Relationships {
		if prev, dup := seen[rel.ID]; dup {
			t.Errorf("relationship id %s is bound twice: %s and %s (C394)", rel.ID, prev, rel.Target)
		}
		seen[rel.ID] = rel.Target
	}
	if len(r.Relationships) != 4 {
		t.Errorf("relationships = %d, want 4 (document, core, app, custom)", len(r.Relationships))
	}
	if r.Properties == nil || r.ExtendedProperties == nil || r.CustomProperties == nil {
		t.Error("a metadata part is unreachable after save")
	}
}

// TestCloseDoesNotDuplicateCallerSuppliedMetadataRelationship is the second
// half of C394: a caller-supplied relationship of the same type must suppress
// the one Close would otherwise append.
func TestCloseDoesNotDuplicateCallerSuppliedMetadataRelationship(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Relationships = []*Relationship{{
		ID:         "rId7",
		Type:       RelTypeCore,
		Target:     "/docProps/core.xml",
		TargetMode: TargetModeInternal,
	}}
	w.Properties = &CoreProperties{Title: "t"}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	r, err := NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if n := len(r.GetRelationshipsByType(RelTypeCore)); n != 1 {
		t.Errorf("core relationships = %d, want exactly 1 (C394)", n)
	}
}

// --- C447: the metadata writers must register their parts ------------------

// TestMetadataPartsAreRegistered is the regression test for C447: the core and
// extended writers did not record their parts in w.parts while the custom one
// did, so the four registries Close reconciles disagreed about what had been
// written.
func TestMetadataPartsAreRegistered(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Properties = &CoreProperties{Title: "t"}
	w.ExtendedProperties = &ExtendedProperties{Application: "spine"}
	w.CustomProperties = &CustomProperties{}
	if err := w.CustomProperties.Set("k", "v"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	for _, part := range []string{
		"/docprops/core.xml", "/docprops/app.xml", "/docprops/custom.xml",
		"/_rels/.rels", "/[content_types].xml",
	} {
		if !w.parts[part] {
			t.Errorf("Close() wrote %s without registering it in w.parts (C447)", part)
		}
	}

	// And every part is emitted exactly once.
	counts := map[string]int{}
	for _, name := range zipEntries(t, buf.Bytes()) {
		counts[strings.ToLower(name)]++
	}
	for name, n := range counts {
		if n != 1 {
			t.Errorf("zip entry %q written %d times", name, n)
		}
	}
}

// --- C392: WriteRawFile must not emit traversal names ----------------------

// TestWriteRawFileRejectsTraversal is the regression test for C392.
func TestWriteRawFileRejectsTraversal(t *testing.T) {
	hostile := []string{
		"../../etc/evil.conf",
		"/../escape.xml",
		"a/../../b.xml",
		"a/./b.xml",
		"a//b.xml",
		"",
		"dir/",
	}
	for _, name := range hostile {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		if err := w.WriteRawFile(name, []byte("x")); err == nil {
			t.Errorf("WriteRawFile(%q) = nil, want it rejected (C392)", name)
		} else if !errors.Is(err, ErrInvalidPartName) {
			t.Errorf("WriteRawFile(%q) error = %v, want ErrInvalidPartName", name, err)
		}
		// Nothing hostile may have reached the archive.
		_ = w.Abort()
		for _, entry := range zipEntries(t, buf.Bytes()) {
			if strings.Contains(entry, "..") {
				t.Errorf("WriteRawFile(%q) emitted zip entry %q", name, entry)
			}
		}
	}

	// The escape hatch keeps working for what its godoc says it is for.
	for _, name := range []string{"[Content_Types].xml", "/[trash]/0000.dat", "word/weird name.xml", "docProps/core.xml"} {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		if err := w.WriteRawFile(name, []byte("x")); err != nil {
			t.Errorf("WriteRawFile(%q) error = %v, want it accepted", name, err)
		}
		_ = w.Abort()
	}
}

// --- C450: the closed check must precede the empty-slice shortcut ----------

// TestWritePartRelationshipsClosedCheckFirst is the regression test for C450.
func TestWritePartRelationshipsClosedCheckFirst(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := w.WritePartRelationships("/word/document.xml", nil); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("WritePartRelationships(closed, empty) = %v, want ErrPackageClosed (C450)", err)
	}
	if err := w.WritePartRelationships("/word/document.xml", []*Relationship{{ID: "rId1", Type: RelTypeImage, Target: "media/i.png"}}); !errors.Is(err, ErrPackageClosed) {
		t.Errorf("WritePartRelationships(closed, non-empty) = %v, want ErrPackageClosed", err)
	}
}
