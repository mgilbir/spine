// Package fuzzseed provides shared helpers for the fuzz targets in opc,
// pptx, docx, and xlsx: in-memory zip assembly, zip entry replacement, and
// optional seeding from the (gitignored) Common Crawl corpus when present.
package fuzzseed

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
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
