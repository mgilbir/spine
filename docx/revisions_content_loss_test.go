package docx

import (
	"strings"
	"testing"
)

// C242: AcceptAllRevisions/RejectAllRevisions rebuilt every container from its
// recorded child order, but API-built simple fields (AddMergeField appends to
// CT_SimpleField.R) and hyperlinks (AddHyperlink sets CT_Hyperlink.R) carry
// typed children with no childOrder entry. The revision transform therefore read
// zero items from those containers and rebuilt them empty, deleting the field
// and hyperlink text even though neither held a tracked change. The transform
// must backfill child order from the typed slices first, like every marshal-path
// mutator, so accepting/rejecting revisions leaves untracked content intact.
func TestAcceptAllRevisionsKeepsAPIBuiltFieldAndHyperlink(t *testing.T) {
	assertKeeps := func(t *testing.T, apply func(*Document) error) {
		t.Helper()
		doc := Create()
		p := doc.AddParagraph()
		p.AddText("Dear ")
		p.AddMergeField("FirstName")
		p.AddText(" see ")
		p.AddHyperlink("our site", "https://example.com")
		p.AddText(".")

		if err := apply(doc); err != nil {
			t.Fatal(err)
		}

		got := p.Text()
		for _, want := range []string{"Dear ", "«FirstName»", " see ", "our site", "."} {
			if !strings.Contains(got, want) {
				t.Errorf("paragraph text lost %q after revision transform; got %q", want, got)
			}
		}
	}

	t.Run("Accept", func(t *testing.T) {
		assertKeeps(t, (*Document).AcceptAllRevisions)
	})
	t.Run("Reject", func(t *testing.T) {
		assertKeeps(t, (*Document).RejectAllRevisions)
	})
}
