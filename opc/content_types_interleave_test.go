package opc

import (
	"bytes"
	"testing"
)

// TestContentTypesInterleavedRoundTrip verifies that Marshal reproduces a
// source [Content_Types].xml byte-for-byte: interleaved Default/Override
// order, per-entry attribute order, and a prolog without standalone.
func TestContentTypesInterleavedRoundTrip(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Override PartName="/xl/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Override ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" PartName="/xl/workbook.xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`</Types>` + "\r\n")

	ct, err := UnmarshalContentTypes(src)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes: %v", err)
	}
	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("round trip drifted:\nsrc  %q\ngot  %q", src, out)
	}
}

// TestContentTypesNoDeclarationRoundTrip verifies a source without an XML
// declaration does not gain one.
func TestContentTypesNoDeclarationRoundTrip(t *testing.T) {
	src := []byte(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`</Types>`)

	ct, err := UnmarshalContentTypes(src)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes: %v", err)
	}
	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("round trip drifted:\nsrc  %q\ngot  %q", src, out)
	}
}

// TestContentTypesEntriesAddedAfterParse verifies post-parse registrations
// are appended after the replayed source entries.
func TestContentTypesEntriesAddedAfterParse(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Override PartName="/a.xml" ContentType="a"/>` +
		`<Default Extension="rels" ContentType="r"/>` +
		`</Types>`)

	ct, err := UnmarshalContentTypes(src)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes: %v", err)
	}
	ct.SetDefault("png", "image/png")
	ct.SetOverride("/b.xml", "b")
	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Override PartName="/a.xml" ContentType="a"/>` +
		`<Default Extension="rels" ContentType="r"/>` +
		`<Default Extension="png" ContentType="image/png"/>` +
		`<Override PartName="/b.xml" ContentType="b"/>` +
		`</Types>`
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}
