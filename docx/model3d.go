package docx

import (
	"sort"
	"strings"

	"github.com/mgilbir/spine/opc"
)

// Model3D is an embedded 3D model extracted from a document: an opaque binary
// glTF asset (typically /word/media/*.glb) referenced by an am3d:model3D
// element. The Data bytes are the model exactly as stored; spine does not parse
// the model geometry.
//
// Extraction is read-only and leaves every part byte-for-byte unchanged on a
// subsequent save. Embedding a new 3D model is not yet supported.
type Model3D struct {
	// PartName is the OPC part name of the 3D model (e.g. "/word/media/model1.glb").
	PartName string
	// ContentType is the part's content type (usually opc.ContentTypeModel3D).
	ContentType string
	// Data is the raw glTF-binary model, carried verbatim.
	Data []byte
	// RelID is the relationship id (the model3D r:embed) that references the
	// model part.
	RelID string
	// Owner is the OPC part name of the part that references the model (e.g. the
	// main document part or a header/footer).
	Owner string
}

// isModel3DRelType reports whether a relationship type URI names a 3D model
// relationship. Office has used more than one dated URI for this Microsoft
// extension, so the year is matched loosely by the "model3d" token.
func isModel3DRelType(relType string) bool {
	return strings.Contains(strings.ToLower(relType), "model3d")
}

// Model3D returns the document's embedded 3D models, located through
// relationships (in every part's scope) whose type names a 3D model or whose
// target part is typed as a glTF-binary model. The result is ordered by
// (owner, relationship id) for determinism.
func (d *Document) Model3D() []Model3D {
	owners := make([]string, 0, len(d.relationships))
	for owner := range d.relationships {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	var models []Model3D
	for _, owner := range owners {
		for _, rel := range d.relationships[owner] {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			part, ok := d.preservedParts[target]
			if !ok {
				continue
			}
			if !isModel3DRelType(rel.Type) && part.ContentType != opc.ContentTypeModel3D {
				continue
			}
			models = append(models, Model3D{
				PartName:    target,
				ContentType: part.ContentType,
				Data:        part.Data,
				RelID:       rel.ID,
				Owner:       owner,
			})
		}
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].Owner != models[j].Owner {
			return models[i].Owner < models[j].Owner
		}
		return models[i].RelID < models[j].RelID
	})
	return models
}
