package docx

import (
	"sort"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// OLEObject is an embedded OLE object extracted from a document: an opaque
// binary part (typically /word/embeddings/oleObjectN.bin) plus the metadata
// needed to identify it. The Data bytes are the object exactly as stored;
// spine does not parse the embedded OLE/CFB stream.
type OLEObject struct {
	// Name is the OPC part name of the embedded object (e.g.
	// "/word/embeddings/oleObject1.bin").
	Name string
	// ContentType is the part's content type. Embedded objects are usually
	// typed opc.ContentTypeOLEObject, but a specific server may use its own
	// (e.g. a binary Excel worksheet).
	ContentType string
	// Data is the raw embedded object, carried verbatim.
	Data []byte
	// ProgID is the OLE server programmatic identifier declared by the
	// referencing element (e.g. "Excel.Sheet.12"), or "" when the document
	// does not declare one in a form spine recognizes.
	ProgID string
}

// OLEObjects returns the document's embedded OLE objects. Objects are located
// through the package's oleObject relationships; any remaining
// /word/embeddings/*.bin parts typed as OLE objects are included as a fallback.
// The result is ordered by part name for determinism. Extraction is read-only
// and leaves every part byte-for-byte unchanged on a subsequent save.
func (d *Document) OLEObjects() []OLEObject {
	seen := make(map[string]bool)
	var objects []OLEObject

	// Deterministic iteration over the owning parts so ProgID resolution and
	// ordering do not depend on map order.
	owners := make([]string, 0, len(d.relationships))
	for owner := range d.relationships {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	for _, owner := range owners {
		for _, rel := range d.relationships[owner] {
			if rel == nil || rel.Type != opc.RelTypeOLEObject || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			part, ok := d.preservedParts[target]
			if !ok || seen[target] {
				continue
			}
			seen[target] = true
			progID := ""
			if src, ok := d.preservedParts[owner]; ok {
				progID = coxml.ExtractOLEProgID(src.Data, rel.ID)
			}
			objects = append(objects, OLEObject{
				Name:        target,
				ContentType: part.ContentType,
				Data:        part.Data,
				ProgID:      progID,
			})
		}
	}

	// Fallback: embedded object parts that no oleObject relationship named but
	// that are typed as OLE objects still count as embedded objects.
	names := make([]string, 0, len(d.preservedParts))
	for name := range d.preservedParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if seen[name] {
			continue
		}
		part := d.preservedParts[name]
		if part.ContentType != opc.ContentTypeOLEObject || !strings.Contains(strings.ToLower(name), "/embeddings/") {
			continue
		}
		seen[name] = true
		objects = append(objects, OLEObject{
			Name:        name,
			ContentType: part.ContentType,
			Data:        part.Data,
		})
	}

	sort.Slice(objects, func(i, j int) bool { return objects[i].Name < objects[j].Name })
	return objects
}
