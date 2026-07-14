package opc

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

const fuzzContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/part.xml" ContentType="application/xml"/></Types>`

const fuzzRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="part.xml"/></Relationships>`

// fuzzExerciseReader opens the bytes as an OPC package and, on success,
// exercises the surface a consumer would hit: part listing, bounded
// per-part reads, and relationship resolution. Any panic is a bug; errors
// are expected and fine.
func fuzzExerciseReader(data []byte) {
	// Tighter-than-default decompression caps keep bomb-shaped inputs cheap
	// during fuzzing; NewReader forwards zero options to this same function,
	// so the code path under test is identical.
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
}

// FuzzNewReader throws arbitrary bytes at the OPC zip reader.
func FuzzNewReader(f *testing.F) {
	minimal := fuzzseed.BuildZip([][2]string{
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
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", "definitely not xml \x00\x01"},
		{"part.xml", "<root/>"},
	}))
	// No [Content_Types].xml.
	f.Add(fuzzseed.BuildZip([][2]string{{"part.xml", "<root/>"}}))
	// Duplicate part names, including case variants.
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"part.xml", "<a/>"},
		{"part.xml", "<b/>"},
		{"PART.XML", "<c/>"},
	}))
	// Path traversal and absolute part names.
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"../escape.xml", "<a/>"},
		{"/abs/part.xml", "<a/>"},
		{"a//b.xml", "<a/>"},
	}))
	// Hostile relationships: duplicate IDs, empty and traversal targets, an
	// unparsable external URI.
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="t" Target="../../.."/><Relationship Id="rId1" Type="t" Target=""/><Relationship Id="rId2" Type="t" TargetMode="External" Target="http://["/></Relationships>`},
	}))
	// Deeply nested XML in a part.
	nested := strings.Repeat("<n>", 300) + "x" + strings.Repeat("</n>", 300)
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", fuzzContentTypesXML},
		{"_rels/.rels", fuzzRootRelsXML},
		{"part.xml", nested},
	}))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzExerciseReader(data)
	})
}

// FuzzOpcMetadataXML feeds the fuzz bytes to the package-metadata XML
// parsers directly: once as [Content_Types].xml and once as _rels/.rels
// inside otherwise-valid archives, so the fuzzer does not have to invent
// whole zip files to reach them.
func FuzzOpcMetadataXML(f *testing.F) {
	f.Add([]byte(fuzzContentTypesXML))
	f.Add([]byte(fuzzRootRelsXML))
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add([]byte(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="" ContentType=""/><Override PartName="part.xml" ContentType=""/><Override PartName="/part.xml"`))
	f.Add([]byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship/><Relationship Id="" Type="" Target=""/><Relationship Id="rId1" TargetMode="Nonsense" Target="/"/></Relationships>`))
	f.Add([]byte(strings.Repeat("<a>", 300) + strings.Repeat("</a>", 300)))

	f.Fuzz(func(t *testing.T, data []byte) {
		s := string(data)
		fuzzExerciseReader(fuzzseed.BuildZip([][2]string{
			{"[Content_Types].xml", s},
			{"_rels/.rels", fuzzRootRelsXML},
			{"part.xml", "<root/>"},
		}))
		fuzzExerciseReader(fuzzseed.BuildZip([][2]string{
			{"[Content_Types].xml", fuzzContentTypesXML},
			{"_rels/.rels", s},
			{"part.xml", "<root/>"},
		}))
	})
}
