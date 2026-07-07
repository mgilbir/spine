package pptx

import (
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// C138: duplicating a slide that has speaker notes gives the duplicate its own
// notes part (not the original's), with the notes' slide back-reference
// repointed to the duplicate.
func TestDuplicate_DeepClonesNotes(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.partName = "/ppt/slides/slide1.xml"

	const srcNotes = "/ppt/notesSlides/notesSlide1.xml"
	p.otherParts[srcNotes] = &coxml.RawPart{
		ContentType: "application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml",
		Data:        []byte("<notes>original</notes>"),
	}
	p.relationships[s.partName] = []*opc.Relationship{
		{ID: "rId1", Type: opc.RelTypeNotesSlide, Target: "../notesSlides/notesSlide1.xml", TargetMode: opc.TargetModeInternal},
	}
	p.relationships[srcNotes] = []*opc.Relationship{
		{ID: "rId1", Type: opc.RelTypeSlide, Target: "../slides/slide1.xml", TargetMode: opc.TargetModeInternal},
	}

	dup := s.Duplicate()

	// The duplicate must reference a different notes part.
	dupNotesTarget := ""
	for _, r := range p.relationships[dup.partName] {
		if r.Type == opc.RelTypeNotesSlide {
			dupNotesTarget = r.Target
		}
	}
	if dupNotesTarget == "" {
		t.Fatal("duplicate has no notesSlide relationship")
	}
	dupNotes := opc.ResolvePartName(dup.partName, dupNotesTarget)
	if dupNotes == srcNotes {
		t.Fatalf("duplicate shares the original notes part %q", dupNotes)
	}
	if _, ok := p.otherParts[dupNotes]; !ok {
		t.Errorf("duplicate's notes part %q was not created", dupNotes)
	}
	// The original notes part is untouched.
	if _, ok := p.otherParts[srcNotes]; !ok {
		t.Error("original notes part was removed")
	}

	// The new notes part's slide back-reference points to the duplicate slide.
	back := ""
	for _, r := range p.relationships[dupNotes] {
		if r.Type == opc.RelTypeSlide {
			back = opc.ResolvePartName(dupNotes, r.Target)
		}
	}
	if back != dup.partName {
		t.Errorf("new notes back-reference = %q, want the duplicate slide %q", back, dup.partName)
	}
}
