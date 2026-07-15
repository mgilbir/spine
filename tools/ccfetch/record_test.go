package main

import (
	"os"
	"testing"
)

func TestLoadManifestParsesRows(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/manifest-docx.tsv"
	content := "url\twarc_filename\twarc_record_offset\twarc_record_length\tcontent_digest\n" +
		"https://example.com/a.docx\tcrawl-data/x.warc.gz\t123\t456\tABCDEF\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := loadManifest(path, "docx", sourceWARC)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.manifestType != "docx" || r.source != sourceWARC || r.url != "https://example.com/a.docx" ||
		r.warcFile != "crawl-data/x.warc.gz" || r.offset != 123 || r.length != 456 || r.digest != "ABCDEF" {
		t.Errorf("unexpected row: %+v", r)
	}
}

// TestLoadManifestMultiCrawl parses the 6-column multi-crawl manifest format,
// where a self-describing crawl id precedes the URL.
func TestLoadManifestMultiCrawl(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/manifest-pptx.tsv"
	content := "crawl\turl\twarc_filename\twarc_record_offset\twarc_record_length\tcontent_digest\n" +
		"CC-MAIN-2026-21\thttps://example.com/a.pptx\tcrawl-data/y.warc.gz\t7\t8\tZZ\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := loadManifest(path, "pptx", sourceWARC)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.crawl != "CC-MAIN-2026-21" || r.url != "https://example.com/a.pptx" ||
		r.warcFile != "crawl-data/y.warc.gz" || r.offset != 7 || r.length != 8 || r.digest != "ZZ" {
		t.Errorf("unexpected row: %+v", r)
	}
}

func TestParseManifestName(t *testing.T) {
	tests := []struct {
		base, typ, source string
		wantErr           bool
	}{
		{"manifest-pptx.tsv", "pptx", sourceWARC, false},
		{"manifest-docx.tsv", "docx", sourceWARC, false},
		{"manifest-xlsx-truncated.tsv", "xlsx", sourceLive, false},
		{"manifest-pptx-truncated.tsv", "pptx", sourceLive, false},
		{"manifest-other.tsv", "", "", true},
		{"manifest-truncated.tsv", "", "", true},
	}
	for _, tt := range tests {
		typ, source, err := parseManifestName(tt.base)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tt.base)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tt.base, err)
			continue
		}
		if typ != tt.typ || source != tt.source {
			t.Errorf("%s: got (%s, %s), want (%s, %s)", tt.base, typ, source, tt.typ, tt.source)
		}
	}
}
