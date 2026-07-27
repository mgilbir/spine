package docx

import "testing"

// C296: a range op whose end run is nested (not a direct paragraph child, e.g.
// a run inside a hyperlink) must not half-complete — no dangling bookmarkStart
// and no orphan comment may be left behind.

func TestAddBookmarkOnRange_NestedEndRun_NoDanglingStart(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	start := p.AddRun()
	start.SetText("start")
	h := p.AddHyperlink("link", "http://example.com")
	nested := h.Runs()
	if len(nested) == 0 {
		t.Fatal("hyperlink has no runs")
	}
	end := nested[0] // a run nested inside the hyperlink, not a direct child

	if got := d.AddBookmarkOnRange("bm", start, end); got != nil {
		t.Fatalf("AddBookmarkOnRange with nested end run = %v, want nil", got)
	}
	if n := len(p.p.BookmarkStart); n != 0 {
		t.Errorf("dangling bookmarkStart left behind: got %d, want 0", n)
	}
	if n := len(p.p.BookmarkEnd); n != 0 {
		t.Errorf("bookmarkEnd unexpectedly placed: got %d, want 0", n)
	}
}

func TestAddCommentOnRange_NestedEndRun_NoOrphanComment(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	start := p.AddRun()
	start.SetText("start")
	h := p.AddHyperlink("link", "http://example.com")
	nested := h.Runs()
	if len(nested) == 0 {
		t.Fatal("hyperlink has no runs")
	}
	end := nested[0] // a run nested inside the hyperlink, not a direct child

	if got := d.AddCommentOnRange(start, end, "Author", "note"); got != nil {
		t.Fatalf("AddCommentOnRange with nested end run = %v, want nil", got)
	}
	if cs := d.Comments(); len(cs) != 0 {
		t.Errorf("orphan comment left in comments.xml: got %d, want 0", len(cs))
	}
	if n := len(p.p.CommentRangeStart); n != 0 {
		t.Errorf("dangling commentRangeStart left behind: got %d, want 0", n)
	}
}
