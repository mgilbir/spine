// Package ccharvest holds the shared harvest primitives used by the Common
// Crawl tooling (tools/ccfetch and tools/ccrun): WARC range fetching, WARC
// record decoding, OPC/OOXML validation and real-type classification, the
// DNS-over-HTTPS blocklist gate for live refetches, and small digest/dedup
// helpers.
//
// It is stdlib-only and lives under internal/ so it is reachable only from the
// module's own tools and tests; it is deliberately NOT imported by the
// library packages (pptx/docx/xlsx/opc/common).
package ccharvest

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	// CCBaseURL is the public Common Crawl data host; WARC filenames from the
	// index are relative to it.
	CCBaseURL = "https://data.commoncrawl.org/"

	// UserAgent identifies the harvester to both the CDN and origin servers.
	UserAgent = "spine-corpus-fetch/1.0 (+github.com/mgilbir/spine)"
)

// DocTypes are the OOXML document types the corpus is built from.
var DocTypes = []string{"pptx", "xlsx", "docx"}

// IsDocType reports whether t is one of the recognized document types.
func IsDocType(t string) bool {
	for _, d := range DocTypes {
		if t == d {
			return true
		}
	}
	return false
}

// Digest returns the lowercase hex SHA-256 of payload — the corpus's content
// identity and dedup key.
func Digest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
