package docx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// A comments part carrying an attribute named "w:" — a bound prefix with an
// empty local part. Go's decoder accepts it, so it parses and is captured; a
// namespace-aware parser refuses it, so writing it back produces a part Word
// cannot read.
const unwritableCommentsXML = `<w:comments ` +
	`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">` +
	`<w:comment w:id="1" w:author="Ada Lovelace" w:="x">` +
	`<w:p w14:paraId="0001"><w:r><w:t>a comment</w:t></w:r></w:p>` +
	`</w:comment></w:comments>`

// TestReplayedAttributeNameIsHeldToTheSameStandard covers the last of the three
// ways a name could reach the output unexamined.
//
// The captured-attribute replay path wrote a name verbatim and returned before
// the guard the composed paths go through. This document therefore saved
// *successfully* while emitting `w:="x"`, which expat and libxml2 both reject —
// so the library produced a part it holds itself to being able to read back.
//
// The save now fails instead. Nothing in the 3600-document corpus carries such
// a name, so this refuses nothing that occurs in practice.
func TestReplayedAttributeNameIsHeldToTheSameStandard(t *testing.T) {
	valid := buildRichDocxFuzzSeed(t)
	pkg := fuzzseed.EditZip(valid, [][2]string{{"word/comments.xml", unwritableCommentsXML}})
	if pkg == nil {
		t.Fatal("could not build the fixture package")
	}

	d, err := OpenReader(strings.NewReader(string(pkg)), int64(len(pkg)))
	if err != nil {
		t.Fatalf("a document with a parseable comments part does not open: %v", err)
	}
	defer func() { _ = d.Close() }()

	comments := d.Comments()
	if len(comments) == 0 {
		t.Fatal("fixture setup failed: the document reports no comments")
	}
	// A mutation, so the part is regenerated rather than preserved verbatim.
	comments[0].SetResolved(true)
	comments[0].SetInitials("AL")

	if _, err := d.SaveBytes(); err == nil {
		t.Fatal("saving a document whose comments part carries a non-QName attribute name reported no error")
	} else if !strings.Contains(err.Error(), `"w:"`) {
		t.Errorf("save error does not name the offending attribute: %v", err)
	}
}

// An untouched document must still save: the guard fires on what is written,
// and a part that is preserved verbatim is never written through the Builder.
func TestGuardDoesNotBreakAnUntouchedDocument(t *testing.T) {
	valid := buildRichDocxFuzzSeed(t)
	pkg := fuzzseed.EditZip(valid, [][2]string{{"word/comments.xml", unwritableCommentsXML}})
	d, err := OpenReader(strings.NewReader(string(pkg)), int64(len(pkg)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = d.Close() }()

	if _, err := d.SaveBytes(); err != nil {
		t.Fatalf("a document nobody edited no longer saves: %v", err)
	}
}
