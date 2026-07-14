package docx

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// retypedDocument builds a minimal document and rewrites its main-part
// content-type override to the given WordprocessingML flavor.
func retypedDocument(t *testing.T, flavor string) []byte {
	t.Helper()
	d := Create()
	d.AddParagraphWithText("hello")
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	ct := string(readZipPart(t, data, "[Content_Types].xml"))
	if !strings.Contains(ct, opc.ContentTypeDocument) {
		t.Fatal("[Content_Types].xml has no document main-part override")
	}
	return rewriteZipEntry(t, data, "[Content_Types].xml",
		strings.Replace(ct, opc.ContentTypeDocument, flavor, 1))
}

// rewriteZipEntry replaces one entry of a zipped docx with new content.
func rewriteZipEntry(t *testing.T, data []byte, name, newData string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if f.Name == name {
			if _, err := w.Write([]byte(newData)); err != nil {
				t.Fatalf("write %s: %v", f.Name, err)
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			t.Fatalf("copy %s: %v", f.Name, err)
		}
		if err := rc.Close(); err != nil {
			t.Fatalf("close %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// TestOpenFlavorVariants opens each WordprocessingML main-part flavor, checks
// the reported flavor, and verifies a zero-modification save keeps the flavor
// (no silent retype to a regular document).
func TestOpenFlavorVariants(t *testing.T) {
	flavors := []string{
		opc.ContentTypeDocument,
		opc.ContentTypeDocumentTemplateMain,
		opc.ContentTypeDocumentMacroMain,
		opc.ContentTypeDocumentTemplateMacroMain,
	}
	for _, flavor := range flavors {
		t.Run(flavor, func(t *testing.T) {
			data := retypedDocument(t, flavor)
			d, err := OpenReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			defer func() { _ = d.Close() }()
			if got := d.Flavor(); got != flavor {
				t.Fatalf("Flavor() = %q, want %q", got, flavor)
			}

			saved, err := d.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			ct := string(readZipPart(t, saved, "[Content_Types].xml"))
			if !strings.Contains(ct, flavor) {
				t.Fatalf("saved [Content_Types].xml lost the %q flavor:\n%s", flavor, ct)
			}
			if flavor != opc.ContentTypeDocument && strings.Contains(ct, opc.ContentTypeDocument) {
				t.Fatalf("saved [Content_Types].xml retyped the main part to a regular document:\n%s", ct)
			}

			reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
			if err != nil {
				t.Fatalf("reopening saved package: %v", err)
			}
			defer func() { _ = reopened.Close() }()
			if got := reopened.Flavor(); got != flavor {
				t.Fatalf("reopened Flavor() = %q, want %q", got, flavor)
			}
		})
	}
}

// TestOpenRejectsUnknownMainPartContentType keeps the strict rejection for
// content types outside the WordprocessingML family.
func TestOpenRejectsUnknownMainPartContentType(t *testing.T) {
	data := retypedDocument(t, "application/x-not-a-document")
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrNotDOCX) {
		t.Fatalf("OpenReader error = %v, want ErrNotDOCX", err)
	}
}

// TestCreatedDocumentFlavor pins the default flavor for created documents.
func TestCreatedDocumentFlavor(t *testing.T) {
	if got := Create().Flavor(); got != opc.ContentTypeDocument {
		t.Fatalf("Create().Flavor() = %q, want %q", got, opc.ContentTypeDocument)
	}
}
