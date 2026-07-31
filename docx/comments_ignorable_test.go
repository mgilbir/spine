package docx

import (
	"strings"
	"testing"
)

// An authored comments part declares mc:Ignorable for the extension prefixes it
// writes.
//
// Comment paragraphs carry w14:paraId, and declaring xmlns:w14 is not what lets
// a consumer skip it — mc:Ignorable is. Without the attribute the part is one a
// strict consumer must reject, which is how the schema-conformance suite found
// it ("Element 'p', attribute 'paraId': The attribute 'paraId' is not allowed").
// That suite needs the ISO schemas, which are not redistributable and so are
// absent in CI; this test states the same requirement in terms of the bytes and
// runs everywhere.
func TestAuthoredCommentsDeclareIgnorable(t *testing.T) {
	d := Create()
	p := d.AddParagraph()
	p.SetText("body")
	p.AddComment("Author", "a remark")

	out, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	part := string(zipPartBytes(out, "word/comments.xml"))
	if part == "" {
		t.Fatal("no word/comments.xml in the saved package")
	}

	if !strings.Contains(part, "w14:paraId") {
		t.Skip("this build no longer writes w14:paraId, so there is nothing to declare ignorable")
	}
	root := part[strings.Index(part, "<w:comments"):]
	root = root[:strings.IndexByte(root, '>')+1]

	ignorable := rootAttrValue(root, "mc:Ignorable")
	if ignorable == "" {
		t.Fatalf("the comments root declares no mc:Ignorable, so w14:paraId is markup a strict "+
			"consumer must reject rather than skip:\n%s", root)
	}
	for _, want := range []string{"w14", "w15"} {
		if !strings.Contains(ignorable, want) {
			t.Errorf("mc:Ignorable = %q, missing %s", ignorable, want)
		}
		if !strings.Contains(root, "xmlns:"+want+"=") {
			t.Errorf("%s is named in mc:Ignorable but never bound:\n%s", want, root)
		}
	}
	// A prefix declared ignorable but not bound resolves to nothing, and one
	// bound but not declared ignorable is what this test exists for; both are
	// checked above, so a duplicate binding is the remaining way to be wrong.
	if strings.Count(root, "xmlns:mc=") != 1 {
		t.Errorf("mc: is declared %d times on the root; a duplicate attribute is malformed XML "+
			"that Go's decoder accepts and a real parser rejects:\n%s", strings.Count(root, "xmlns:mc="), root)
	}
}

// rootAttrValue returns the value of a double-quoted attribute in a start tag.
func rootAttrValue(tag, name string) string {
	i := strings.Index(tag, " "+name+"=\"")
	if i < 0 {
		return ""
	}
	rest := tag[i+len(name)+3:]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}
