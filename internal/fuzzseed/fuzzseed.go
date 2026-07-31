// Package fuzzseed provides shared helpers for the fuzz targets in opc,
// pptx, docx, and xlsx: in-memory zip assembly, zip entry replacement, and
// optional seeding from the (gitignored) Common Crawl corpus when present.
package fuzzseed

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// BuildZip assembles an in-memory zip archive from name/body pairs, in
// order. Duplicate names are allowed: archive/zip writes them as separate
// entries, which is exactly the malformed shape some seeds want. Entries
// whose names archive/zip rejects are skipped so the remaining entries
// still form an archive.
func BuildZip(entries [][2]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		if err != nil {
			continue
		}
		_, _ = w.Write([]byte(e[1]))
	}
	_ = zw.Close()
	return buf.Bytes()
}

// ReplaceZipEntry rewrites orig with the named entry's content replaced by
// body, leaving every other entry byte-for-byte intact. It returns nil when
// orig is not a readable zip archive.
func ReplaceZipEntry(orig []byte, name string, body []byte) []byte {
	zr, err := zip.NewReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		content := body
		if f.Name != name {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				continue
			}
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			continue
		}
		_, _ = w.Write(content)
	}
	_ = zw.Close()
	return buf.Bytes()
}

// EditZip returns a copy of orig with each entry in edits applied: an entry
// whose name already exists is replaced in place, and one that does not is
// appended after the existing entries. Every other entry is copied verbatim.
// The edits are applied in slice order. It returns nil when orig is not a
// readable zip archive. It generalizes ReplaceZipEntry to add-or-replace so a
// scaffolded package (extra parts plus rewritten relationships and content
// types) can be assembled in one pass.
func EditZip(orig []byte, edits [][2]string) []byte {
	zr, err := zip.NewReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		return nil
	}
	replace := make(map[string]string, len(edits))
	order := make([]string, 0, len(edits))
	for _, e := range edits {
		if _, seen := replace[e[0]]; !seen {
			order = append(order, e[0])
		}
		replace[e[0]] = e[1]
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	written := make(map[string]bool, len(zr.File))
	for _, f := range zr.File {
		content, ok := replace[f.Name]
		if !ok {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				continue
			}
			content = string(data)
		}
		written[f.Name] = true
		w, err := zw.Create(f.Name)
		if err != nil {
			continue
		}
		_, _ = w.Write([]byte(content))
	}
	// Append entries whose names were not present in orig.
	for _, name := range order {
		if written[name] {
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			continue
		}
		_, _ = w.Write([]byte(replace[name]))
	}
	_ = zw.Close()
	return buf.Bytes()
}

// ZipEntry returns the content of the named entry in the archive, or nil.
func ZipEntry(orig []byte, name string) []byte {
	zr, err := zip.NewReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		return nil
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil
		}
		return data
	}
	return nil
}

// CorpusSeeds returns up to max real files of the given format ("docx",
// "pptx", or "xlsx"), smallest first, each at most maxSize bytes, from the
// gitignored Common Crawl corpus. The directory is resolved from the
// SPINE_FUZZ_CORPUS environment variable or testdata/corpus/cc at the
// repository root; when neither exists the result is empty, so tests and
// fuzz runs work without the corpus and no corpus bytes are ever committed.
func CorpusSeeds(format string, max int, maxSize int64) [][]byte {
	root := os.Getenv("SPINE_FUZZ_CORPUS")
	if root == "" {
		root = filepath.Join("..", "testdata", "corpus", "cc")
	}
	dir := filepath.Join(root, format)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type candidate struct {
		name string
		size int64
	}
	var files []candidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() > maxSize {
			continue
		}
		files = append(files, candidate{e.Name(), info.Size()})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].size != files[j].size {
			return files[i].size < files[j].size
		}
		return files[i].name < files[j].name
	})
	var seeds [][]byte
	for _, c := range files {
		if len(seeds) >= max {
			break
		}
		data, err := os.ReadFile(filepath.Join(dir, c.name))
		if err != nil {
			continue
		}
		seeds = append(seeds, data)
	}
	return seeds
}

// ---------------------------------------------------------------------------
// Namespace resolution
// ---------------------------------------------------------------------------

// UnresolvedElementNamespaces returns the qualified names in data whose prefix
// no declaration in scope binds, at most limit of them.
//
// Go's decoder is deliberately lenient about this: an undeclared prefix comes
// back with Name.Space set to the literal prefix rather than to a URI, and
// decoding carries on. That leniency is why a part can be emitted with an
// unbound prefix and still round-trip through this library while PowerPoint or
// Word calls the file damaged and the library's own accessors read the content
// back as empty — so the test is whether the space is a URI.
//
// Element and attribute names are both examined. Attributes matter as much as
// elements and are sometimes the only evidence: docx.Paragraph.AddHyperlink
// wrote r:id into a document whose root declared only w:, which no element name
// records. Attributes are also the names most likely to have been replayed
// verbatim from a malformed source, so AssertEmittedNamespacesResolve decides
// blame by comparing against the package the save was built from rather than by
// narrowing what is checked here.
func UnresolvedElementNamespaces(data []byte, limit int) []string {
	// Which spaces count as resolved is decided from the document's own
	// declarations rather than from the shape of the string. Testing for "://"
	// looks equivalent and is not: VML and the Office extensions are URNs
	// (urn:schemas-microsoft-com:vml), so that test reports every correct
	// watermark, signature line and OLE object as broken.
	declared := map[string]bool{
		"":    true,
		"xml": true,
		// The reserved prefixes are bound by the XML spec itself and are never
		// declared, so xml:space="preserve" must not read as unresolved.
		"http://www.w3.org/XML/1998/namespace": true,
		"http://www.w3.org/2000/xmlns/":        true,
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range se.Attr {
			// xmlns:p="URI" arrives as Space "xmlns"; a default xmlns="URI" as
			// Local "xmlns" with no space.
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				declared[a.Value] = true
			}
		}
	}

	// An undeclared prefix decodes with Name.Space set to the prefix itself,
	// which is never one of the declared URIs — including when the declaration
	// exists but is out of scope, which is the shape a sibling-scoped inline
	// declaration produces.
	dec = xml.NewDecoder(bytes.NewReader(data))
	var bad []string
	for len(bad) < limit {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !declared[se.Name.Space] {
			bad = append(bad, se.Name.Space+":"+se.Name.Local)
		}
		for _, a := range se.Attr {
			if len(bad) >= limit {
				break
			}
			// An xmlns declaration is itself in the reserved xmlns space, and
			// an unprefixed attribute is in no namespace at all.
			if a.Name.Space == "xmlns" || a.Name.Space == "" {
				continue
			}
			if !declared[a.Name.Space] {
				bad = append(bad, a.Name.Space+":"+a.Name.Local+" (attribute of "+se.Name.Local+")")
			}
		}
	}
	return bad
}

// AssertEmittedNamespacesResolve fails tb for every XML part in the saved
// package out that names an element in a namespace nothing declares.
//
// A part is held to that standard only when the package it was built from does
// not already break it: a part preserved verbatim from a source that carried
// unbound prefixes must keep them, byte for byte, and blaming the writer for
// what it was handed would make this unfalsifiable. A part with no counterpart
// in the input was written from nothing and is always the writer's own.
//
// It exists because this defect class was invisible by construction. The
// library's part roots replay the namespace declarations captured from their
// source, and every name written under them — by the reflection marshaler, by a
// hand-written MarshalToBuilder, or spliced in as preserved bytes — assumed
// those declarations covered it. A source that declared less than the writer
// used produced a part no reader could resolve, with no error on either side:
// docx.Paragraph.AddHyperlink wrote r:id into a document whose root declared
// only w:, and pptx.Slide.SetNotes wrote its text as a:t into a notes slide
// whose root declared only p:. Both are ordinary calls over valid input.
func AssertEmittedNamespacesResolve(tb testing.TB, in, out []byte) {
	tb.Helper()
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		return // not a package; the caller's own oracles cover that
	}
	for _, zf := range zr.File {
		if !strings.HasSuffix(zf.Name, ".xml") && !strings.HasSuffix(zf.Name, ".rels") {
			continue
		}
		emitted := ZipEntry(out, zf.Name)
		if emitted == nil {
			continue
		}
		bad := UnresolvedElementNamespaces(emitted, 4)
		if len(bad) == 0 {
			continue
		}
		if source := ZipEntry(in, zf.Name); source != nil && len(UnresolvedElementNamespaces(source, 1)) > 0 {
			// The source part was already unresolvable; preserving it is not
			// this library emitting an unbound prefix.
			continue
		}
		tb.Errorf("saved %s names %v in a namespace nothing declares; "+
			"Office reports such a part as damaged and this library reads it back empty",
			zf.Name, bad)
	}
}
