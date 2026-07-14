package opc

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// fuzzZip assembles an in-memory zip archive from name/body pairs, in order.
// Duplicate names are allowed: archive/zip writes them as separate entries,
// which is exactly the malformed shape some seeds want.
func fuzzZip(entries [][2]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		if err != nil {
			// Entry names like `c:\...` can be rejected; skip them so the
			// remaining entries still form an archive.
			continue
		}
		_, _ = w.Write([]byte(e[1]))
	}
	_ = zw.Close()
	return buf.Bytes()
}

const fuzzContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/part.xml" ContentType="application/xml"/></Types>`

const fuzzRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="part.xml"/></Relationships>`

// FuzzNewReader throws arbitrary bytes at the OPC zip reader and, when a
// package opens, exercises the surface a consumer would hit: part listing,
// per-part reads, and relationship resolution. Any panic is a bug; errors
// are expected and fine.
func FuzzNewReader(f *testing.F) {
	minimal := fuzzZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"_rels/.rels", fuzzRootRelsXML},
		{"part.xml", `<?xml version="1.0"?><root><child a="1">text</child></root>`},
	})

	f.Add(minimal)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	// Empty archive: just an end-of-central-directory record.
	f.Add([]byte("PK\x05\x06" + strings.Repeat("\x00", 18)))
	// Truncated central directory.
	f.Add(minimal[:len(minimal)-10])
	// One corrupted byte in the middle of the archive.
	corrupt := append([]byte(nil), minimal...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// [Content_Types].xml that is not XML at all.
	f.Add(fuzzZip([][2]string{
		{"[Content_Types].xml", "definitely not xml \x00\x01"},
		{"part.xml", "<root/>"},
	}))
	// No [Content_Types].xml.
	f.Add(fuzzZip([][2]string{{"part.xml", "<root/>"}}))
	// Duplicate part names, including case variants.
	f.Add(fuzzZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"part.xml", "<a/>"},
		{"part.xml", "<b/>"},
		{"PART.XML", "<c/>"},
	}))
	// Path traversal and absolute part names.
	f.Add(fuzzZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"../escape.xml", "<a/>"},
		{"/abs/part.xml", "<a/>"},
		{"a//b.xml", "<a/>"},
	}))
	// Hostile relationships: duplicate IDs, empty and traversal targets, an
	// unparsable external URI.
	f.Add(fuzzZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="t" Target="../../.."/><Relationship Id="rId1" Type="t" Target=""/><Relationship Id="rId2" Type="t" TargetMode="External" Target="http://["/></Relationships>`},
	}))
	// Deeply nested XML in a part.
	nested := strings.Repeat("<n>", 300) + "x" + strings.Repeat("</n>", 300)
	f.Add(fuzzZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"_rels/.rels", fuzzRootRelsXML},
		{"part.xml", nested},
	}))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Tighter-than-default decompression caps keep bomb-shaped inputs
		// cheap during fuzzing; NewReader forwards zero options to this same
		// function, so the code path under test is identical.
		opts := ReaderOptions{
			MaxDecompressedPartSize:    16 << 20,
			MaxDecompressedPackageSize: 64 << 20,
		}
		r, err := NewReaderWithOptions(bytes.NewReader(data), int64(len(data)), opts)
		if err != nil {
			return
		}

		for i, file := range r.Files {
			if i >= 64 {
				break
			}
			if _, err := file.ReadAll(); err != nil {
				continue
			}
			rels, err := r.GetPartRelationships(file.Name)
			if err != nil {
				continue
			}
			for _, rel := range rels {
				_ = rel.IsExternal()
			}
		}
		for _, rel := range r.Relationships {
			_ = rel.IsExternal()
		}
		_ = r.GetRelationshipsByType(RelTypeOfficeDocument)
		if r.ContentTypes != nil {
			_ = r.ContentTypes.Clone()
		}
	})
}
