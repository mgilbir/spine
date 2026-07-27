package docx

import (
	"strings"
	"testing"
)

// TestCommentInTableCellReplyAndAnchor verifies that a comment anchored in a
// table-cell paragraph is reachable by the threading and anchor-text accessors
// (C267): Reply() must place the reply's range markers (not orphan them) and
// AnchorText() must return the cell's text. Both walked only the top-level body
// paragraphs before, so a table-nested anchor was invisible.
func TestCommentInTableCellReplyAndAnchor(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("before the table")
	tbl := doc.AddTable(1, 1)
	cellPara := tbl.Rows()[0].Cells()[0].Paragraphs()[0]
	cellPara.SetText("cell anchor text")

	c := cellPara.AddComment("Alice", "root comment")
	if got := c.AnchorText(); got != "cell anchor text" {
		t.Errorf("AnchorText() = %q, want %q (table-cell anchor not found)", got, "cell anchor text")
	}

	reply := c.Reply("Bob", "a reply")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	docXML := zipEntryString(t, saved, "word/document.xml")

	// The reply's reference marker must be placed in the document (nested in the
	// parent's range), not orphaned because the anchor lives in a table cell.
	wantRef := `<w:commentReference w:id="` + reply.ID() + `"/>`
	if !strings.Contains(docXML, wantRef) {
		t.Errorf("reply commentReference %s not written (orphan reply):\n%s", wantRef, docXML)
	}
}
