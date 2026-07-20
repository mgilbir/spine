package pptx

import (
	"sort"

	"github.com/mgilbir/spine/opc"
)

// InkAnnotation is an ink (pen-stroke) annotation extracted from a slide. Ink is
// stored as an InkML content part (application/inkml+xml) referenced by a
// p:contentPart element in the slide's shape tree through a customXml
// relationship. The Data bytes are the InkML exactly as stored; spine does not
// parse the stroke geometry.
//
// Extraction is read-only and leaves every part byte-for-byte unchanged on a
// subsequent save (existing ink round-trips verbatim through the raw shape-tree
// preservation). Authoring new ink strokes is not yet supported.
type InkAnnotation struct {
	// PartName is the OPC part name of the InkML part (e.g. "/ppt/ink/ink1.xml").
	PartName string
	// ContentType is the part's content type (opc.ContentTypeInk).
	ContentType string
	// Data is the raw InkML, carried verbatim.
	Data []byte
	// RelID is the relationship id (the p:contentPart r:id) that references the
	// ink part from the slide.
	RelID string
}

// isInkPart reports whether a part carried in otherParts under the given name is
// an ink (InkML) content part.
func (p *Presentation) isInkPart(name string) bool {
	part, ok := p.otherParts[name]
	return ok && part.ContentType == opc.ContentTypeInk
}

// InkAnnotations returns the slide's ink annotations, located through the
// slide's customXml relationships whose target is an InkML part. The result is
// ordered by relationship id for determinism.
func (s *Slide) InkAnnotations() []InkAnnotation {
	if s.presentation == nil || s.partName == "" {
		return nil
	}
	p := s.presentation
	var inks []InkAnnotation
	seen := make(map[string]bool)
	for _, rel := range p.relationships[s.partName] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		target := opc.ResolvePartName(s.partName, rel.Target)
		if seen[rel.ID] || !p.isInkPart(target) {
			continue
		}
		seen[rel.ID] = true
		part := p.otherParts[target]
		inks = append(inks, InkAnnotation{
			PartName:    target,
			ContentType: part.ContentType,
			Data:        part.Data,
			RelID:       rel.ID,
		})
	}
	sort.Slice(inks, func(i, j int) bool { return inks[i].RelID < inks[j].RelID })
	return inks
}

// InkAnnotations returns every ink annotation in the presentation, aggregated
// across all slides and ordered by (slide index, relationship id). A slide's
// ink is reported once per referencing relationship.
func (p *Presentation) InkAnnotations() []InkAnnotation {
	var all []InkAnnotation
	for _, s := range p.slides {
		all = append(all, s.InkAnnotations()...)
	}
	return all
}
