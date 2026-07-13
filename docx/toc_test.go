package docx

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddTableOfContents(t *testing.T) {
	doc := Create()
	if err := doc.AddTableOfContents(TOCOptions{}); err != nil {
		t.Fatalf("AddTableOfContents: %v", err)
	}
	doc.AddHeading("One", 1)
	doc.AddParagraphWithText("body one")
	doc.AddHeading("Two", 2)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")

	for _, want := range []string{
		`<w:sdt>`,
		`<w:docPartGallery w:val="Table of Contents"/>`,
		`<w:docPartUnique/>`,
		`<w:fldChar w:fldCharType="begin" w:dirty="true"/>`,
		`TOC \o "1-3" \h \z \u`,
		`<w:fldChar w:fldCharType="separate"/>`,
		`<w:fldChar w:fldCharType="end"/>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("document.xml missing %s", want)
		}
	}

	// The instruction sequence must be ordered begin → instr → separate → end.
	iBegin := strings.Index(body, `w:fldCharType="begin"`)
	iInstr := strings.Index(body, `<w:instrText`)
	iSep := strings.Index(body, `w:fldCharType="separate"`)
	iEnd := strings.Index(body, `w:fldCharType="end"`)
	if iBegin >= iInstr || iInstr >= iSep || iSep >= iEnd {
		t.Errorf("field character sequence out of order: begin=%d instr=%d sep=%d end=%d", iBegin, iInstr, iSep, iEnd)
	}

	// Reopen and re-save: the SDT must survive the round-trip.
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	data2, err := doc2.SaveBytes()
	if err != nil {
		t.Fatalf("re-save: %v", err)
	}
	body2 := zipPart(t, data2, "word/document.xml")
	if !strings.Contains(body2, `<w:docPartGallery w:val="Table of Contents"/>`) {
		t.Error("TOC SDT lost on reopen+save")
	}
}

func TestAddTableOfContentsLevels(t *testing.T) {
	doc := Create()
	if err := doc.AddTableOfContents(TOCOptions{MinLevel: 2, MaxLevel: 5}); err != nil {
		t.Fatalf("AddTableOfContents: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	if !strings.Contains(body, `\o "2-5"`) {
		t.Errorf("custom level range not in instruction")
	}
}

func TestAddTableOfContentsValidation(t *testing.T) {
	doc := Create()
	for _, opts := range []TOCOptions{
		{MinLevel: -1},
		{MaxLevel: 10},
		{MinLevel: 4, MaxLevel: 2},
	} {
		if err := doc.AddTableOfContents(opts); err == nil {
			t.Errorf("expected error for %+v", opts)
		}
	}
}

// TestTOCBeforeExistingContent: the SDT participates in body child ordering —
// content added after the TOC must come after it in the XML.
func TestTOCOrderInBody(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("before toc")
	if err := doc.AddTableOfContents(TOCOptions{}); err != nil {
		t.Fatalf("AddTableOfContents: %v", err)
	}
	doc.AddParagraphWithText("after toc")

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	iBefore := strings.Index(body, "before toc")
	iSdt := strings.Index(body, "<w:sdt>")
	iAfter := strings.Index(body, "after toc")
	if iBefore >= iSdt || iSdt >= iAfter {
		t.Errorf("body order wrong: before=%d sdt=%d after=%d", iBefore, iSdt, iAfter)
	}
}
