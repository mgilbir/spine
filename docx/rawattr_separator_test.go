package docx

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Two attributes with no white space between them must not be re-emitted that
// way, because the second one's captured verbatim rendering begins where the
// first one's closing quote ended.
//
// Go's decoder does not require the separator: it reads <pgSz w:w="0"A=""/> as
// two attributes and reports no error. The capture then records A="" with the
// whitespace that preceded it — none — and replaying that verbatim produced
// <w:pgSzA=""/>, a start tag that does not parse. FuzzDocxDocumentPart found
// it, but its input declared no namespaces at all, so the undeclared-prefix
// check now rejects that document before the separator matters and the
// reproducer no longer reaches this code. This test declares the namespaces
// properly, which is the case that still gets here.
func TestAttributesWithNoSeparatorAreSeparatedOnOutput(t *testing.T) {
	seed, err := Create().SaveBytes()
	if err != nil {
		t.Fatalf("building the seed document: %v", err)
	}

	const wns = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	// w:w="0" and A="" are adjacent with nothing between them.
	part := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + wns + `><w:body><w:sectPr><w:pgSz w:w="0"A=""/></w:sectPr></w:body></w:document>`

	pkg := fuzzseed.ReplaceZipEntry(seed, "word/document.xml", []byte(part))
	if pkg == nil {
		t.Fatal("could not substitute word/document.xml")
	}

	d, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("the document does not open: %v", err)
	}
	defer func() { _ = d.Close() }()

	// An untouched part is written back byte for byte, which reproduces the
	// source's spacing and never reaches the replay path. The defect is in
	// regeneration, so the document has to be modified first — the fuzzer's
	// mutation step is what exposed it.
	d.AddParagraph().SetText("forces the part to be regenerated")

	out, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("saving: %v", err)
	}

	emitted := zipPartBytes(out, "word/document.xml")
	if len(emitted) == 0 {
		t.Fatal("the saved package has no word/document.xml")
	}
	// Without the guard this is the byte sequence that appears, and it is the
	// one the decoder below rejects; naming it makes a failure legible.
	if bytes.Contains(emitted, []byte(`"A=`)) && !bytes.Contains(emitted, []byte(`" A=`)) {
		t.Errorf("the two attributes were emitted with no separator between them")
	}
	dec := xml.NewDecoder(bytes.NewReader(emitted))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("the emitted word/document.xml does not parse: %v\n%s", err, truncateForLog(emitted))
		}
	}
}

func truncateForLog(b []byte) string {
	const max = 400
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + strings.Repeat(".", 3)
}
