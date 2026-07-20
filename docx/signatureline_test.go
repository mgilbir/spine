package docx

import (
	"bytes"
	"strings"
	"testing"
)

// TestAddSignatureLineRoundTrip is the headline scenario: add a signature line,
// save, reopen, and read it back.
func TestAddSignatureLineRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("Please sign below:")
	doc.AddSignatureLine(SignatureLineOptions{
		Signer:       "Jane Doe",
		Title:        "Director",
		Email:        "jane@example.com",
		Instructions: "Sign here to approve.",
	})

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	body := zipPart(t, data, "word/document.xml")
	for _, want := range []string{
		`<o:signatureline`,
		`issignatureline="t"`,
		`suggestedsigner="Jane Doe"`,
		`suggestedsigner2="Director"`,
		`suggestedsigneremail="jane@example.com"`,
		`signinginstructions="Sign here to approve."`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("document.xml missing %q\n%s", want, body)
		}
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	lines := doc2.SignatureLines()
	if len(lines) != 1 {
		t.Fatalf("SignatureLines() = %d, want 1", len(lines))
	}
	sl := lines[0]
	if sl.Signer != "Jane Doe" || sl.Title != "Director" || sl.Email != "jane@example.com" || sl.Instructions != "Sign here to approve." {
		t.Errorf("read signature line mismatch: %+v", sl)
	}
	if sl.ID == "" {
		t.Error("signature line has no id")
	}
	if rep := doc2.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors: %v", rep)
	}
}

// TestMultipleSignatureLines: several lines are all read back, distinctly.
func TestMultipleSignatureLines(t *testing.T) {
	doc := Create()
	doc.AddSignatureLine(SignatureLineOptions{Signer: "Alice"})
	doc.AddSignatureLine(SignatureLineOptions{Signer: "Bob", Title: "CTO"})

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	lines := doc2.SignatureLines()
	if len(lines) != 2 {
		t.Fatalf("SignatureLines() = %d, want 2", len(lines))
	}
	if lines[0].Signer != "Alice" || lines[1].Signer != "Bob" || lines[1].Title != "CTO" {
		t.Errorf("signature lines mismatch: %+v", lines)
	}
	// Distinct GUIDs.
	if lines[0].ID == lines[1].ID {
		t.Errorf("signature lines share an id: %s", lines[0].ID)
	}
}

// TestSignatureLineInlineInParagraph: the paragraph-level API appends the shape
// to an existing paragraph inline.
func TestSignatureLineInlineInParagraph(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddText("Signed: ")
	p.AddSignatureLine(SignatureLineOptions{Signer: "Sam"})

	data, _ := doc.SaveBytes()
	body := zipPart(t, data, "word/document.xml")
	iText := strings.Index(body, ">Signed: <")
	iSig := strings.Index(body, "<o:signatureline")
	if iText < 0 || iSig < 0 || iText >= iSig {
		t.Errorf("inline text/signature order wrong: %q", body)
	}
}

// TestVMLAttrBoundary: attribute extraction must not confuse
// signinginstructions with signinginstructionsset, nor id with provid.
func TestVMLAttrBoundary(t *testing.T) {
	attrs := ` v:ext="edit" id="{ABC}" provid="{DEF}" signinginstructionsset="t" signinginstructions="Do it"`
	if got := vmlAttr(attrs, "id"); got != "{ABC}" {
		t.Errorf(`vmlAttr id = %q, want {ABC}`, got)
	}
	if got := vmlAttr(attrs, "signinginstructions"); got != "Do it" {
		t.Errorf(`vmlAttr signinginstructions = %q, want "Do it"`, got)
	}
}
