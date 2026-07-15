package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/internal/ccharvest"
)

// Kinds of reference, determined by which manifest a row came from.
const (
	kindWARC = "warc" // complete payload, read from the WARC archive by range
	kindLive = "live" // truncated payload, refetched from the origin URL
)

// Ref is one self-contained corpus reference: everything needed to fetch and
// test a single file, with no dependency on an external index. It is the unit
// the orchestrator passes to a worker (as JSON on stdin) and the unit recorded
// in the ledger and quarantine.
type Ref struct {
	Crawl    string `json:"crawl"`
	Type     string `json:"type"` // manifest document type: pptx/xlsx/docx
	Kind     string `json:"kind"` // kindWARC or kindLive
	URL      string `json:"url"`
	WARCFile string `json:"warc_filename"`
	Offset   int64  `json:"warc_record_offset"`
	Length   int64  `json:"warc_record_length"`
	Digest   string `json:"content_digest"`
}

// parseManifestName derives (type, kind) from a manifest file name:
// manifest-<type>.tsv is WARC-fetchable, manifest-<type>-truncated.tsv is
// live-refetched.
func parseManifestName(base string) (typ, kind string, err error) {
	name := strings.TrimSuffix(strings.TrimPrefix(base, "manifest-"), ".tsv")
	kind = kindWARC
	if strings.HasSuffix(name, "-truncated") {
		kind = kindLive
		name = strings.TrimSuffix(name, "-truncated")
	}
	if !ccharvest.IsDocType(name) {
		return "", "", fmt.Errorf("%s: cannot derive document type (want manifest-{pptx,xlsx,docx}[-truncated].tsv)", base)
	}
	return name, kind, nil
}

// resolveManifests expands directory arguments into manifest-*.tsv globs and
// returns a sorted, de-duplicated file list.
func resolveManifests(args []string) ([]string, error) {
	seen := map[string]bool{}
	var files []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			matches, err := filepath.Glob(filepath.Join(a, "manifest-*.tsv"))
			if err != nil {
				return nil, err
			}
			sort.Strings(matches)
			for _, m := range matches {
				add(m)
			}
		} else {
			add(a)
		}
	}
	return files, nil
}

// loadManifest reads one manifest file into refs. It accepts both the legacy
// 5-column layout and the 6-column multi-crawl layout (a leading crawl column).
func loadManifest(path, typ, kind string) ([]Ref, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fh.Close() }()

	var refs []Ref
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if lineNo == 1 || line == "" { // header / blank
			continue
		}
		fields := strings.Split(line, "\t")
		var crawl string
		switch len(fields) {
		case 6:
			crawl, fields = fields[0], fields[1:]
		case 5:
			// legacy single-crawl manifest, no crawl column
		default:
			return nil, fmt.Errorf("%s:%d: want 5 or 6 tab-separated fields, got %d", path, lineNo, len(fields))
		}
		off, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: bad offset: %w", path, lineNo, err)
		}
		length, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: bad length: %w", path, lineNo, err)
		}
		refs = append(refs, Ref{
			Crawl:    crawl,
			Type:     typ,
			Kind:     kind,
			URL:      fields[0],
			WARCFile: fields[1],
			Offset:   off,
			Length:   length,
			Digest:   fields[4],
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return refs, nil
}

// loadAllRefs reads every manifest file and returns the concatenated refs in a
// stable order (manifest file order, then row order), with duplicate digests
// removed (first occurrence wins). The returned count reports how many
// duplicate rows were dropped.
func loadAllRefs(files []string) (refs []Ref, dupes int, err error) {
	seen := map[string]bool{}
	for _, path := range files {
		typ, kind, err := parseManifestName(filepath.Base(path))
		if err != nil {
			return nil, 0, err
		}
		rows, err := loadManifest(path, typ, kind)
		if err != nil {
			return nil, 0, err
		}
		for _, r := range rows {
			if seen[r.Digest] {
				dupes++
				continue
			}
			seen[r.Digest] = true
			refs = append(refs, r)
		}
	}
	return refs, dupes, nil
}
