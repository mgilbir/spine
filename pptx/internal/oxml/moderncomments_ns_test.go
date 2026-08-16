package oxml

import (
	"strings"
	"testing"
)

// A comment part whose root binds "a" to something that is not DrawingML. Every
// prefix it uses is declared, so it is well-formed and namespace-valid; the
// point is only that "a" does not mean here what it means in a
// PowerPoint-authored file. The thread's txBody is preserved as raw bytes.
const foreignPrefixThreadXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<p188:cmLst xmlns:a="http://example.invalid/not-drawingml" ` +
	`xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main">` +
	`<p188:cm id="{1}" authorId="{2}" created="2025-01-30T11:09:28.599">` +
	`<p188:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>body</a:t></a:r></a:p></p188:txBody>` +
	`</p188:cm></p188:cmLst>`

// TestRewriteKeepsTheSourceMeaningOfAPrefix is the regression for the namespace
// re-homing this marshaler used to perform.
//
// The part's children are preserved as raw bytes, and raw bytes carry a prefix,
// not a namespace. Writing a fixed xmlns:a="…drawingml…" on the regenerated root
// therefore moved the whole preserved body from the namespace it was authored in
// into DrawingML — the bytes untouched, their meaning replaced. A document can
// reach that state innocently, and a hostile one can reach it on purpose: markup
// that is inert foreign-namespace junk to a reader or a scanner becomes live
// DrawingML once someone opens the file, edits a comment and saves.
func TestRewriteKeepsTheSourceMeaningOfAPrefix(t *testing.T) {
	part, err := ParseModernCommentPart([]byte(foreignPrefixThreadXML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	part.Comment.Status = "resolved" // any edit forces the rewrite

	out, err := part.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)

	// The source's binding for "a" must survive: the preserved body still means
	// what it meant.
	if !strings.Contains(got, `xmlns:a="http://example.invalid/not-drawingml"`) {
		t.Errorf("the source's binding for a was dropped:\n%s", got)
	}
	if strings.Contains(got, `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`) {
		t.Errorf("a was rebound to DrawingML, re-homing the preserved body:\n%s", got)
	}
	// And the body really is still there, unmodified.
	if !strings.Contains(got, `<a:t>body</a:t>`) {
		t.Errorf("preserved body lost:\n%s", got)
	}
	// The edit landed.
	if !strings.Contains(got, `status="resolved"`) {
		t.Errorf("the edit that forced the rewrite is missing:\n%s", got)
	}
}

// A namespace this marshaler needs but the source never declared has to be bound
// to a prefix the source left free, rather than to the canonical one when the
// source has already used that spelling for something else.
func TestAddedDeclarationDoesNotStealATakenPrefix(t *testing.T) {
	part, err := ParseModernCommentPart([]byte(foreignPrefixThreadXML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// A library-created reply: its body is synthesized, so DrawingML must be
	// written — and the source has taken "a" for something else.
	part.Comment.Replies = append(part.Comment.Replies, &ModernReply{
		ID: "{9}", AuthorID: "{2}", Created: "2025-01-30T12:00:00.000", BodyText: "added",
	})

	out, err := part.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)

	if strings.Contains(got, `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`) {
		t.Fatalf("DrawingML stole the prefix the source bound elsewhere:\n%s", got)
	}
	if !strings.Contains(got, "http://schemas.openxmlformats.org/drawingml/2006/main") {
		t.Fatalf("the synthesized body's namespace was never declared:\n%s", got)
	}
	if !strings.Contains(got, "added") {
		t.Fatalf("the added reply is missing:\n%s", got)
	}
	// The source's own binding is still intact alongside it.
	if !strings.Contains(got, `xmlns:a="http://example.invalid/not-drawingml"`) {
		t.Errorf("the source's binding for a was dropped:\n%s", got)
	}
}

// A part this library generates from nothing still takes the canonical
// declarations, so ordinary output is unchanged.
func TestGeneratedPartKeepsCanonicalDeclarations(t *testing.T) {
	part := &ModernCommentPart{Comment: &ModernComment{
		ID: "{1}", AuthorID: "{2}", Created: "2025-01-30T11:09:28.599", BodyText: "hello",
	}}
	out, err := part.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`,
		`xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("generated part missing %s:\n%s", want, out)
		}
	}
}
