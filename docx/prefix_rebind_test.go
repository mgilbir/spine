package docx

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// A declaration this library adds must never rebind a prefix the source already
// bound to something else.
//
// The marshalers add a declaration when a part needs markup in a namespace the
// source did not declare — w14/w15 on the comment part, m: on a document
// containing math. The test for "did not declare" was by URI, which is not the
// same question as "is this prefix free": a source that binds m: to some other
// namespace has not declared NSMath, so the by-URI test said "add it" and the
// root came out carrying xmlns:m twice. Every m: name in the document then
// resolves through whichever wins, on a save asked to change nothing.
//
// FuzzDocxCommentsXML found this on the w15 path. The math path below has the
// same shape and no fuzz target that reaches it — it came out of sweeping for
// the pattern rather than from a crasher, so it gets a test of its own.
func TestAddedDeclarationDoesNotRebindASourcePrefix(t *testing.T) {
	// A document that binds m: to a namespace of its own and contains math, so
	// the marshaler wants NSMath declared and the canonical prefix is taken.
	const foreignMath = "urn:example:not-office-math"
	part := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:m="` + foreignMath + `">` +
		`<w:body><w:p><m:oMath/></w:p></w:body></w:document>`

	seed, err := Create().SaveBytes()
	if err != nil {
		t.Fatalf("building the seed document: %v", err)
	}
	pkg := fuzzseed.ReplaceZipEntry(seed, "word/document.xml", []byte(part))
	if pkg == nil {
		t.Fatal("could not substitute word/document.xml")
	}

	d, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Skipf("the fixture does not open, so the rebinding path is not reached: %v", err)
	}
	defer func() { _ = d.Close() }()

	// Force regeneration; an untouched part is written back byte for byte and
	// never reaches the declaration logic.
	d.AddParagraph().SetText("forces the part to be regenerated")

	out, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	emitted := zipPartBytes(out, "word/document.xml")
	if len(emitted) == 0 {
		t.Fatal("the saved package has no word/document.xml")
	}

	// Scoped to the root start tag. An inline declaration deeper in the part is
	// a separate question — a captured element in a foreign namespace is
	// replayed under the binding the model expects, which re-homes it, and that
	// is not what this test is about.
	root := emitted
	if i := bytes.IndexByte(emitted, '>'); i >= 0 {
		if j := bytes.IndexByte(emitted[i+1:], '>'); j >= 0 {
			root = emitted[:i+1+j+1]
		}
	}

	// Whatever prefix the library chose for its own math markup, the source's
	// binding must still say what the source said, and must say it once.
	decls := regexp.MustCompile(`xmlns:m="([^"]*)"`).FindAllSubmatch(root, -1)
	switch {
	case len(decls) == 0:
		t.Errorf("the source's xmlns:m declaration was dropped:\n%.400s", root)
	case len(decls) > 1:
		t.Errorf("xmlns:m is declared %d times on the root element, which is not well-formed:\n%.400s",
			len(decls), root)
	case string(decls[0][1]) != foreignMath:
		t.Errorf("xmlns:m was rebound from %q to %q — every m: name in the part changed meaning",
			foreignMath, decls[0][1])
	}
}
