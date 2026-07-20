package docx

import (
	"sort"

	"github.com/mgilbir/spine/opc"
)

// InkAnnotation is an ink (pen-stroke) annotation extracted from a document. Ink
// is stored as an InkML content part (application/inkml+xml) referenced by a
// w:contentPart element through a customXml relationship. The Data bytes are the
// InkML exactly as stored; spine does not parse the stroke geometry.
//
// Extraction is read-only and leaves every part byte-for-byte unchanged on a
// subsequent save. Authoring new ink strokes is not yet supported.
type InkAnnotation struct {
	// PartName is the OPC part name of the InkML part (e.g. "/word/ink/ink1.xml").
	PartName string
	// ContentType is the part's content type (opc.ContentTypeInk).
	ContentType string
	// Data is the raw InkML, carried verbatim.
	Data []byte
	// RelID is the relationship id (the w:contentPart r:id) that references the
	// ink part.
	RelID string
	// Owner is the OPC part name of the part that references the ink (e.g. the
	// main document part or a header/footer).
	Owner string
}

// InkAnnotations returns the document's ink annotations, located through the
// customXml relationships (in every part's scope) whose target is an InkML part.
// The result is ordered by (owner, relationship id) for determinism; a part
// referenced more than once is reported once per referencing relationship.
func (d *Document) InkAnnotations() []InkAnnotation {
	owners := make([]string, 0, len(d.relationships))
	for owner := range d.relationships {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	var inks []InkAnnotation
	for _, owner := range owners {
		for _, rel := range d.relationships[owner] {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			part, ok := d.preservedParts[target]
			if !ok || part.ContentType != opc.ContentTypeInk {
				continue
			}
			inks = append(inks, InkAnnotation{
				PartName:    target,
				ContentType: part.ContentType,
				Data:        part.Data,
				RelID:       rel.ID,
				Owner:       owner,
			})
		}
	}
	sort.Slice(inks, func(i, j int) bool {
		if inks[i].Owner != inks[j].Owner {
			return inks[i].Owner < inks[j].Owner
		}
		return inks[i].RelID < inks[j].RelID
	})
	return inks
}
