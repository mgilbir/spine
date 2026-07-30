package docx

import (
	"testing"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// threadFrom walks a queue it appends to. A comment whose w15:paraIdParent
// equals its own w15:paraId therefore re-queued itself on every visit and grew
// the slice without bound — a wild file can say that and nothing in the parse
// rejects it.
//
// Comments() filters such a comment out as a reply before threadFrom is
// reached, so no public call path hits it today. That is a coincidence of the
// filter rather than a guard, which is exactly the kind of reachability
// argument that stops holding when someone changes the filter. This calls
// threadFrom directly.
//
// Found by the secondary-part fuzzing sweep, which reported it as latent rather
// than fixing it; verified unreachable through Comments() before hardening.
func TestThreadFromTerminatesOnASelfParentingComment(t *testing.T) {
	const paraID = "1A2B3C4D"

	self := &oxml.CT_Comment{
		Id: "1",
		P:  []*oxml.CT_P{{ParaId: paraID}},
	}
	d := &Document{
		comments: &oxml.CT_Comments{Comment: []*oxml.CT_Comment{self}},
		commentsExtended: &oxml.CT_CommentsEx{
			CommentEx: []*oxml.CT_CommentEx{{ParaId: paraID, ParaIdParent: paraID}},
		},
	}

	// Without the visited set this never returns; the test binary dies on
	// memory rather than failing, so bound it in the assertion instead.
	got := d.threadFrom(self)
	if len(got) != 1 {
		t.Fatalf("threadFrom returned %d comments for a single self-parenting comment, want 1", len(got))
	}
	if got[0] != self {
		t.Errorf("threadFrom returned the wrong comment")
	}
}
