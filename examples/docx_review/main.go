// Example: Review a Word document with tracked changes and comments.
//
// This example plays the role of a reviewer who receives a document carrying
// tracked changes (insertions and deletions made with Word's Track Changes on)
// and reviewer comments, inspects them, then accepts every change to produce a
// clean, final copy.
//
// It runs in two phases:
//
//  1. Setup — spine has no public API for *authoring* tracked changes (they are
//     normally produced by Word), so we synthesize a realistic input document
//     by writing a minimal .docx whose word/document.xml already contains
//     w:ins/w:del revisions. A .docx is just a ZIP of XML parts, so this needs
//     nothing beyond the standard library. We then open that package with
//     spine and attach a couple of reviewer comments through the public API,
//     and save it as the "incoming" document.
//
//  2. Review — we reopen the incoming document and use the public review API:
//     Document.Revisions() to list every tracked change, Document.Comments()
//     to walk the comment threads, and Document.AcceptAllRevisions() to apply
//     the changes. Saving yields a clean document, which we reopen to confirm
//     no tracked changes remain and to print the resulting text.
//
// Run with: go run ./examples/docx_review
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/docx"
)

func main() {
	inputPath := "review_input.docx"
	cleanPath := "review_clean.docx"
	if len(os.Args) > 1 {
		inputPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		cleanPath = os.Args[2]
	}
	for _, p := range []string{inputPath, cleanPath} {
		if dir := filepath.Dir(p); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				log.Fatalf("Failed to create directory: %v", err)
			}
		}
	}

	// ── Phase 1: synthesize the incoming, under-review document ───────
	buildIncomingDocument(inputPath)
	fmt.Printf("Incoming document (tracked changes + comments) saved to: %s\n\n", inputPath)

	// ── Phase 2: run the review flow ─────────────────────────────────
	reviewDocument(inputPath, cleanPath)
}

// buildIncomingDocument writes a document that carries tracked changes and
// reviewer comments. The tracked changes are injected as raw XML (spine models
// them for round-trip fidelity but does not author them); the comments are
// added through spine's public API to show both halves of the review surface.
func buildIncomingDocument(path string) {
	// A minimal but valid word/document.xml with two paragraphs of tracked
	// changes: a deletion + insertion in the first, an inserted word in the
	// second. Each revision records an author and an ISO-8601 date, exactly as
	// Word writes them.
	const body = `<w:body>` +
		`<w:p>` +
		`<w:r><w:t xml:space="preserve">The migration completed </w:t></w:r>` +
		`<w:del w:id="1" w:author="Alice" w:date="2026-02-01T10:00:00Z">` +
		`<w:r><w:delText>last week</w:delText></w:r></w:del>` +
		`<w:ins w:id="2" w:author="Alice" w:date="2026-02-01T10:01:00Z">` +
		`<w:r><w:t>on schedule</w:t></w:r></w:ins>` +
		`<w:r><w:t>.</w:t></w:r>` +
		`</w:p>` +
		`<w:p>` +
		`<w:r><w:t xml:space="preserve">All </w:t></w:r>` +
		`<w:ins w:id="3" w:author="Bob" w:date="2026-02-02T09:30:00Z">` +
		`<w:r><w:t xml:space="preserve">seventeen </w:t></w:r></w:ins>` +
		`<w:r><w:t>services are healthy.</w:t></w:r>` +
		`</w:p>` +
		`<w:sectPr/>` +
		`</w:body>`

	raw := minimalDocx(body)

	// Open the synthesized package with spine so we can attach comments through
	// the public API. bytes.Reader satisfies io.ReaderAt.
	doc, err := docx.OpenReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		log.Fatalf("Failed to open synthesized document: %v", err)
	}

	// Attach a comment thread to the first paragraph: a reviewer question and
	// the author's reply. Paragraph.AddComment anchors over the whole paragraph.
	paras := doc.Paragraphs()
	c := paras[0].AddComment("Carol", "Can you confirm the exact completion date?")
	c.Reply("Alice", "Yes — it finished on the 1st, ahead of the plan.")

	// A second, standalone comment on the other paragraph.
	paras[1].AddComment("Carol", "Nice, the service count is worth calling out.")

	if err := doc.Save(path); err != nil {
		log.Fatalf("Failed to save incoming document: %v", err)
	}
}

// reviewDocument opens an incoming document, reports its tracked changes and
// comment threads, accepts all revisions, and writes a clean copy — then
// reopens the clean copy to confirm the changes were applied.
func reviewDocument(inputPath, cleanPath string) {
	doc, err := docx.Open(inputPath)
	if err != nil {
		log.Fatalf("Failed to open document under review: %v", err)
	}

	// ── Tracked changes ──────────────────────────────────────────────
	revs := doc.Revisions()
	fmt.Printf("Tracked changes: %d\n", len(revs))
	for _, r := range revs {
		fmt.Printf("  - %-9s by %-6s on %s: %q (editable=%v)\n",
			r.Type(), r.Author(), r.Date(), r.Text(), r.Editable())
	}

	// ── Comment threads ──────────────────────────────────────────────
	fmt.Printf("\nComment threads: %d\n", len(doc.Comments()))
	for _, c := range doc.Comments() {
		fmt.Printf("  - [%s] %q (resolved=%v)\n", c.Author(), c.Text(), c.Resolved())
		for _, reply := range c.Replies() {
			fmt.Printf("      ↳ [%s] %q\n", reply.Author(), reply.Text())
		}
	}

	// ── Accept every tracked change ──────────────────────────────────
	//
	// Insertions become normal text and deletions are removed; the comments are
	// left untouched. (RejectAllRevisions would instead discard insertions and
	// restore deletions.)
	if err := doc.AcceptAllRevisions(); err != nil {
		log.Fatalf("Failed to accept revisions: %v", err)
	}
	if err := doc.Save(cleanPath); err != nil {
		log.Fatalf("Failed to save clean document: %v", err)
	}
	fmt.Printf("\nClean document (all changes accepted) saved to: %s\n", cleanPath)

	// ── Verify the clean copy ────────────────────────────────────────
	clean, err := docx.Open(cleanPath)
	if err != nil {
		log.Fatalf("Failed to reopen clean document: %v", err)
	}
	if remaining := clean.Revisions(); len(remaining) != 0 {
		log.Fatalf("expected no tracked changes after accept, found %d", len(remaining))
	}
	fmt.Println("\nFinal text (revisions applied):")
	for _, p := range clean.Paragraphs() {
		if text := p.Text(); text != "" {
			fmt.Printf("  %s\n", text)
		}
	}
	fmt.Printf("\nReview complete: 0 tracked changes remain, %d comment(s) preserved.\n",
		len(clean.Comments()))
}

// minimalDocx wraps a word/document.xml body in the smallest valid Open
// Packaging Conventions ZIP: a content-types map, the package relationships
// that point at the main document, and the document part itself.
func minimalDocx(body string) []byte {
	const wns = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + wns + `>` + body + `</w:document>`,
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range parts {
		w, err := zw.Create(name)
		if err != nil {
			log.Fatalf("Failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			log.Fatalf("Failed to write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		log.Fatalf("Failed to finalize zip: %v", err)
	}
	return buf.Bytes()
}
