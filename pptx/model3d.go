package pptx

import (
	"sort"
	"strings"

	"github.com/mgilbir/spine/opc"
)

// Model3D is an embedded 3D model extracted from a slide: an opaque binary glTF
// asset (typically /ppt/media/*.glb) referenced by an am3d:model3D element. The
// Data bytes are the model exactly as stored; spine does not parse the model
// geometry.
//
// Extraction is read-only and leaves every part byte-for-byte unchanged on a
// subsequent save (the model3D graphicFrame is carried verbatim through the raw
// shape-tree preservation). Embedding a new 3D model is not yet supported.
type Model3D struct {
	// PartName is the OPC part name of the 3D model (e.g. "/ppt/media/model1.glb").
	PartName string
	// ContentType is the part's content type (usually opc.ContentTypeModel3D).
	ContentType string
	// Data is the raw glTF-binary model, carried verbatim.
	Data []byte
	// RelID is the relationship id (the model3D r:embed) that references the
	// model part from the slide.
	RelID string
}

// isModel3DRelType reports whether a relationship type URI names a 3D model
// relationship. Office has used more than one dated URI for this Microsoft
// extension, so the year is matched loosely by the "model3d" token.
func isModel3DRelType(relType string) bool {
	return strings.Contains(strings.ToLower(relType), "model3d")
}

// isModel3DPart reports whether a part carried in otherParts under the given
// name is an embedded 3D model (glTF binary).
func (p *Presentation) isModel3DPart(name string) bool {
	part, ok := p.otherParts[name]
	return ok && part.ContentType == opc.ContentTypeModel3D
}

// Model3D returns the slide's embedded 3D models, located through the slide's
// relationships: any relationship whose type names a 3D model, plus any
// relationship whose target part is typed as a glTF-binary model. The result is
// ordered by relationship id for determinism.
func (s *Slide) Model3D() []Model3D {
	if s.presentation == nil || s.partName == "" {
		return nil
	}
	p := s.presentation
	var models []Model3D
	seen := make(map[string]bool)
	for _, rel := range p.relationships[s.partName] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		target := opc.ResolvePartName(s.partName, rel.Target)
		if seen[rel.ID] {
			continue
		}
		if !isModel3DRelType(rel.Type) && !p.isModel3DPart(target) {
			continue
		}
		part, ok := p.otherParts[target]
		if !ok {
			continue
		}
		seen[rel.ID] = true
		models = append(models, Model3D{
			PartName:    target,
			ContentType: part.ContentType,
			Data:        part.Data,
			RelID:       rel.ID,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].RelID < models[j].RelID })
	return models
}

// Model3D returns every embedded 3D model in the presentation, aggregated
// across all slides and ordered by (slide index, relationship id).
func (p *Presentation) Model3D() []Model3D {
	var all []Model3D
	for _, s := range p.slides {
		all = append(all, s.Model3D()...)
	}
	return all
}
