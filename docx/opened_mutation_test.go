package docx

import (
	"path/filepath"
	"strings"
	"testing"
)

// C2: core mutators on a document opened from a file must persist through save
// (previously they were appended to typed slices the child-order-gated marshal
// never read, so they were silently dropped).
func TestOpenedDocument_MutationsPersist(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}

	// AddParagraph on an opened document.
	doc.AddParagraphWithText("INSERTED-PARAGRAPH")

	// AddRun on a parsed paragraph.
	if paras := doc.Paragraphs(); len(paras) > 0 {
		paras[0].AddRun().SetText("APPENDED-RUN")
	}

	tmp := filepath.Join(t.TempDir(), "out.docx")
	if err := doc.Save(tmp); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	var foundPara, foundRun bool
	for _, p := range reopened.Paragraphs() {
		txt := p.Text()
		if strings.Contains(txt, "INSERTED-PARAGRAPH") {
			foundPara = true
		}
		if strings.Contains(txt, "APPENDED-RUN") {
			foundRun = true
		}
	}
	if !foundPara {
		t.Error("paragraph added to an opened document was dropped on save")
	}
	if !foundRun {
		t.Error("run added to a parsed paragraph was dropped on save")
	}
}
