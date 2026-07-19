package pptx

import (
	"encoding/xml"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Notes returns the slide's speaker-notes text: the text of the notes slide's
// body placeholder, with paragraphs joined by "\n". It returns "" when the
// slide has no notes slide or the notes slide has no body text.
func (s *Slide) Notes() string {
	ns, _ := s.loadNotesSlide()
	if ns == nil {
		return ""
	}
	sp := notesBodyPlaceholder(ns)
	if sp == nil || sp.TxBody == nil {
		return ""
	}
	return notesBodyText(sp.TxBody)
}

// SetNotes sets the slide's speaker-notes text. When the slide already has a
// notes slide, only its body-placeholder text is replaced; the rest of the
// notes slide is preserved. Otherwise a new notesSlide part is created (wired to
// the slide, and to the notes master when the deck has one) and registered so it
// is written on the next save.
func (s *Slide) SetNotes(text string) {
	p := s.presentation
	if ns, partName := s.loadNotesSlide(); ns != nil {
		setNotesBodyText(ns, text)
		p.otherParts[partName] = &coxml.RawPart{
			ContentType: opc.ContentTypeNotesSlide,
			Data:        marshalNotesSlide(ns),
		}
		return
	}

	partName := p.nextAvailableNotesName()
	p.otherParts[partName] = &coxml.RawPart{
		ContentType: opc.ContentTypeNotesSlide,
		Data:        marshalNotesSlide(newNotesSlideModel(text)),
	}

	// slide -> notesSlide relationship.
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         s.nextRelID(),
		Type:       opc.RelTypeNotesSlide,
		Target:     relativeTarget(s.partName, partName),
		TargetMode: opc.TargetModeInternal,
	})

	// notesSlide -> slide back-reference, plus -> notesMaster when one exists.
	notesRels := []*opc.Relationship{{
		ID:         "rId1",
		Type:       opc.RelTypeSlide,
		Target:     relativeTarget(partName, s.partName),
		TargetMode: opc.TargetModeInternal,
	}}
	if master := p.notesMasterPartName(); master != "" {
		notesRels = append(notesRels, &opc.Relationship{
			ID:         "rId2",
			Type:       opc.RelTypeNotesMaster,
			Target:     relativeTarget(partName, master),
			TargetMode: opc.TargetModeInternal,
		})
	}
	p.relationships[partName] = notesRels
}

// loadNotesSlide finds and parses the slide's notes slide part, returning the
// parsed model and its part name, or (nil, "") when there is no notes slide.
func (s *Slide) loadNotesSlide() (*oxml.NotesSlide, string) {
	partName := s.notesSlidePartName()
	if partName == "" {
		return nil, ""
	}
	data := s.presentation.rawPartData(partName)
	if data == nil {
		return nil, ""
	}
	var ns oxml.NotesSlide
	if err := xml.Unmarshal(data, &ns); err != nil {
		return nil, ""
	}
	return &ns, partName
}

// notesSlidePartName resolves the slide's notesSlide relationship to a part
// name, or "" when the slide has no (internal) notes slide.
func (s *Slide) notesSlidePartName() string {
	for _, rel := range s.presentation.relationships[s.partName] {
		if rel != nil && rel.Type == opc.RelTypeNotesSlide && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(s.partName, rel.Target)
		}
	}
	return ""
}

// notesMasterPartName returns the deck's notes master part name, or "" when the
// deck has none. Keys are scanned in sorted order so the choice is deterministic
// on the rare deck that carries more than one.
func (p *Presentation) notesMasterPartName() string {
	for _, name := range sortedKeys(p.otherParts) {
		if strings.HasPrefix(name, "/ppt/notesMasters/") && strings.HasSuffix(name, ".xml") {
			return name
		}
	}
	return ""
}

// notesBodyPlaceholder returns the notes slide's body placeholder shape (the
// p:sp whose nvSpPr/nvPr/ph@type="body"), or nil when there is none.
func notesBodyPlaceholder(ns *oxml.NotesSlide) *oxml.Shape {
	if ns.CSld == nil || ns.CSld.SpTree == nil {
		return nil
	}
	for _, sp := range ns.CSld.SpTree.Sp {
		if sp == nil || sp.NvSpPr == nil || sp.NvSpPr.NvPr == nil {
			continue
		}
		if ph := sp.NvSpPr.NvPr.Ph; ph != nil && ph.Type == "body" {
			return sp
		}
	}
	return nil
}

// notesBodyText extracts the plain text of a notes body: the runs of each
// paragraph concatenated, paragraphs joined by "\n".
func notesBodyText(tb *dml.TxBody) string {
	paras := make([]string, 0, len(tb.P))
	for _, p := range tb.P {
		var sb strings.Builder
		for _, r := range p.R {
			sb.WriteString(r.T)
		}
		paras = append(paras, sb.String())
	}
	return strings.Join(paras, "\n")
}

// setNotesBodyText replaces the body placeholder's text on an existing notes
// slide, adding a body placeholder when the notes slide lacks one.
func setNotesBodyText(ns *oxml.NotesSlide, text string) {
	sp := notesBodyPlaceholder(ns)
	if sp == nil {
		if ns.CSld == nil {
			ns.CSld = &oxml.CommonSlideData{}
		}
		if ns.CSld.SpTree == nil {
			ns.CSld.SpTree = newShapeTree()
		}
		sp = newNotesBodyShape(nextNotesShapeID(ns))
		ns.CSld.SpTree.AppendSp(sp)
	}
	sp.TxBody = notesBodyTxBody(text)
}

// newNotesSlideModel builds a minimal valid CT_NotesSlide whose body
// placeholder holds text.
func newNotesSlideModel(text string) *oxml.NotesSlide {
	tree := newShapeTree()
	tree.Sp = []*oxml.Shape{newNotesBodyShapeWithText(2, text)}
	return &oxml.NotesSlide{
		CSld:      &oxml.CommonSlideData{SpTree: tree},
		ClrMapOvr: &dml.ClrMapOvr{MasterClrMapping: &dml.MasterClrMapping{}},
	}
}

// newNotesBodyShape builds an empty notes body placeholder shape with the given
// shape id.
func newNotesBodyShape(id uint32) *oxml.Shape {
	return newNotesBodyShapeWithText(id, "")
}

func newNotesBodyShapeWithText(id uint32, text string) *oxml.Shape {
	return &oxml.Shape{
		NvSpPr: &oxml.NvSpPr{
			CNvPr:   &dml.CNvPr{Id: id, Name: "Notes Placeholder"},
			CNvSpPr: &dml.CNvSpPr{SpLocks: &dml.SpLocks{NoGrp: true}},
			NvPr:    &oxml.NvPr{Ph: &oxml.Placeholder{Type: "body", Idx: 1}},
		},
		SpPr:   &dml.SpPr{},
		TxBody: notesBodyTxBody(text),
	}
}

// notesBodyTxBody builds a DrawingML text body from plain text: one paragraph
// per "\n"-separated line, each non-empty line carrying a single run.
func notesBodyTxBody(text string) *dml.TxBody {
	tb := &dml.TxBody{
		BodyPr:   &dml.BodyPr{},
		LstStyle: &dml.LstStyle{},
	}
	for _, line := range strings.Split(text, "\n") {
		p := &dml.P{}
		if line != "" {
			p.R = []*dml.R{{T: line}}
		}
		tb.P = append(tb.P, p)
	}
	return tb
}

// nextNotesShapeID returns a shape id not already used in the notes slide's
// shape tree.
func nextNotesShapeID(ns *oxml.NotesSlide) uint32 {
	max := uint32(1)
	if ns.CSld == nil || ns.CSld.SpTree == nil {
		return max + 1
	}
	if gp := ns.CSld.SpTree.NvGrpSpPr; gp != nil && gp.CNvPr != nil && gp.CNvPr.Id > max {
		max = gp.CNvPr.Id
	}
	for _, sp := range ns.CSld.SpTree.Sp {
		if sp != nil && sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil && sp.NvSpPr.CNvPr.Id > max {
			max = sp.NvSpPr.CNvPr.Id
		}
	}
	return max + 1
}

// marshalNotesSlide serializes a notes slide part (p:notes) with the standard
// PresentationML namespace declarations.
func marshalNotesSlide(ns *oxml.NotesSlide) []byte {
	b := xmlb.NewPresentationMLBuilder()
	b.SetCollapseEmptyElements(true)
	b.WriteHeader()

	var attrs []xmlb.Attr
	if ns.ShowMasterSp {
		attrs = append(attrs, xmlb.StrAttr("showMasterSp", "1"))
	}
	if ns.ShowMasterPhAnim {
		attrs = append(attrs, xmlb.StrAttr("showMasterPhAnim", "1"))
	}
	b.StartElementWithNS(nsP, "notes", xmlb.PresentationMLNamespaces(), attrs...)
	if ns.CSld != nil {
		b.MarshalElement(nsP, "cSld", ns.CSld)
	}
	if ns.ClrMapOvr != nil {
		b.MarshalElement(nsP, "clrMapOvr", ns.ClrMapOvr)
	}
	if ns.ExtLst != nil {
		b.MarshalElement(nsP, "extLst", ns.ExtLst)
	}
	b.EndElement(nsP, "notes")
	_ = b.Finish()
	return b.Bytes()
}
