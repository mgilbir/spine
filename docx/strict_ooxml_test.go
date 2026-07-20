package docx

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// A valid ISO-Strict Word document (officeDocument relationship under the
// purl.oclc.org namespace) must be reported as opc.ErrStrictOOXML — a distinct,
// actionable signal that it is a genuine but unsupported-dialect Office file —
// not as the generic ErrNotDOCX. Found by Common Crawl validation (docx
// 0676b4).
func TestOpenReportsStrictOOXMLDistinctly(t *testing.T) {
	const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`</Types>`
	const rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" ` +
		`Type="http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument" ` +
		`Target="word/document.xml"/>` +
		`</Relationships>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels":         rootRels,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	_, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if !errors.Is(err, opc.ErrStrictOOXML) {
		t.Fatalf("OpenReader error = %v, want opc.ErrStrictOOXML", err)
	}
	if errors.Is(err, ErrNotDOCX) {
		t.Errorf("strict package misclassified as ErrNotDOCX")
	}
}
