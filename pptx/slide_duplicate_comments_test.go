package pptx

import (
	"fmt"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// commentPartsOf returns the absolute part names of the comment parts (legacy
// and modern) the given slide part references, in relationship order.
func commentPartsOf(p *Presentation, slidePart string) []string {
	var out []string
	for _, rel := range p.relationships[slidePart] {
		if rel == nil {
			continue
		}
		if rel.Type == opc.RelTypeComments || rel.Type == opc.RelTypeModernComments {
			out = append(out, opc.ResolvePartName(slidePart, rel.Target))
		}
	}
	return out
}

// C414: Duplicate cloned every slide relationship but deep-cloned only the notes
// slide, so original and duplicate shared one comments part — which ECMA models
// as per-slide — and the modern part's pc:sldMk sldId still named the source
// slide, anchoring the duplicate's thread to a different slide.
func TestDuplicateDeepClonesModernComments(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("original")
	if c := s.AddComment("Ann", "first thought"); c == nil {
		t.Fatal("AddComment returned nil")
	}

	srcParts := commentPartsOf(p, s.partName)
	if len(srcParts) != 1 {
		t.Fatalf("fixture setup failed: source slide has %d comment parts", len(srcParts))
	}

	dup := s.Duplicate()
	if dup == nil {
		t.Fatal("Duplicate returned nil")
	}
	dupParts := commentPartsOf(p, dup.partName)
	if len(dupParts) != 1 {
		t.Fatalf("duplicate has %d comment parts, want 1", len(dupParts))
	}
	if dupParts[0] == srcParts[0] {
		t.Fatalf("duplicate shares the source's comment part %q", dupParts[0])
	}
	if _, ok := p.otherParts[dupParts[0]]; !ok {
		t.Fatalf("duplicate's comment part %q was never created", dupParts[0])
	}

	// The thread must anchor to the duplicate, not the slide it was copied from.
	wantAnchor := fmt.Sprintf(`sldId="%d"`, dup.id)
	body := string(p.otherParts[dupParts[0]].Data)
	if !strings.Contains(body, wantAnchor) {
		t.Errorf("duplicate's comment part does not anchor to the duplicate (%s):\n%s", wantAnchor, body)
	}
	if srcAnchor := fmt.Sprintf(`sldId="%d"`, s.id); strings.Contains(body, srcAnchor) {
		t.Errorf("duplicate's comment part still anchors to the source slide (%s)", srcAnchor)
	}
	// The source must be untouched.
	if src := string(p.otherParts[srcParts[0]].Data); !strings.Contains(src, fmt.Sprintf(`sldId="%d"`, s.id)) {
		t.Errorf("source comment part lost its own anchor:\n%s", src)
	}

	// Editing one thread must not change the other.
	dupComments := dup.Comments()
	if len(dupComments) != 1 {
		t.Fatalf("duplicate has %d comments, want 1", len(dupComments))
	}
	dupComments[0].Reply("Bob", "reply on the duplicate only")
	if strings.Contains(string(p.otherParts[srcParts[0]].Data), "reply on the duplicate only") {
		t.Error("a reply added to the duplicate's thread leaked into the source's comment part")
	}

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts := zipParts(t, out)
	for _, name := range append(append([]string{}, srcParts...), dupParts...) {
		if _, ok := parts[name]; !ok {
			t.Errorf("comment part %q is missing from the saved package", name)
		}
	}
}

// C414, legacy comments: the pre-2018 p:cmLst part is per-slide too and must be
// copied rather than shared.
func TestDuplicateDeepClonesLegacyComments(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("original")

	const legacyPart = "/ppt/comments/comment1.xml"
	p.otherParts[legacyPart] = &coxml.RawPart{
		ContentType: opc.ContentTypePresentationComments,
		Data: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:cmLst xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:cm authorId="0" dt="2024-01-01T00:00:00" idx="1"><p:pos x="100" y="100"/><p:text>legacy</p:text></p:cm></p:cmLst>`),
	}
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         s.nextRelID(),
		Type:       opc.RelTypeComments,
		Target:     relativeTarget(s.partName, legacyPart),
		TargetMode: opc.TargetModeInternal,
	})

	dup := s.Duplicate()
	if dup == nil {
		t.Fatal("Duplicate returned nil")
	}
	dupParts := commentPartsOf(p, dup.partName)
	if len(dupParts) != 1 {
		t.Fatalf("duplicate has %d comment parts, want 1", len(dupParts))
	}
	if dupParts[0] == legacyPart {
		t.Fatalf("duplicate shares the source's legacy comment part %q", legacyPart)
	}
	if _, ok := p.otherParts[dupParts[0]]; !ok {
		t.Fatalf("duplicate's legacy comment part %q was never created", dupParts[0])
	}
}

// C414: the deep clone must survive a RemoveSlide of the original — the copies
// are independent parts, so removing one slide leaves the other's intact.
func TestDuplicateCommentsSurviveOriginalRemoval(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddComment("Ann", "keep me")
	dup := s.Duplicate()
	if dup == nil {
		t.Fatal("Duplicate returned nil")
	}
	dupPart := commentPartsOf(p, dup.partName)
	if len(dupPart) != 1 {
		t.Fatalf("duplicate has %d comment parts, want 1", len(dupPart))
	}

	if err := p.RemoveSlide(0); err != nil {
		t.Fatalf("RemoveSlide: %v", err)
	}
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if _, ok := zipParts(t, out)[dupPart[0]]; !ok {
		t.Errorf("the surviving slide's comment part %q was deleted with the original", dupPart[0])
	}
}
