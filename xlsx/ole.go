package xlsx

import (
	"sort"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// OLEObject is an embedded OLE object extracted from a workbook: an opaque
// binary part (typically /xl/embeddings/oleObjectN.bin) plus the metadata
// needed to identify it. The Data bytes are the object exactly as stored;
// spine does not parse the embedded OLE/CFB stream.
type OLEObject struct {
	// Name is the OPC part name of the embedded object.
	Name string
	// ContentType is the part's content type (usually opc.ContentTypeOLEObject).
	ContentType string
	// Data is the raw embedded object, carried verbatim.
	Data []byte
	// ProgID is the OLE server programmatic identifier declared by the
	// referencing element (e.g. "Excel.Sheet.12"), or "" when none is declared
	// in a form spine recognizes.
	ProgID string
}

// OLEObjects returns the workbook's embedded OLE objects. Objects are located
// through the package's oleObject relationships; any remaining
// /xl/embeddings/*.bin parts typed as OLE objects are included as a fallback.
// The result is ordered by part name for determinism. Extraction is read-only
// and leaves every part byte-for-byte unchanged on a subsequent save.
func (w *Workbook) OLEObjects() []OLEObject {
	seen := make(map[string]bool)
	var objects []OLEObject

	owners := make([]string, 0, len(w.relationships))
	for owner := range w.relationships {
		owners = append(owners, owner)
	}
	sort.Strings(owners)

	for _, owner := range owners {
		for _, rel := range w.relationships[owner] {
			if rel == nil || rel.Type != opc.RelTypeOLEObject || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			part, ok := w.preservedParts[target]
			if !ok || seen[target] {
				continue
			}
			seen[target] = true
			progID := ""
			if src, ok := w.preservedParts[owner]; ok {
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

	names := make([]string, 0, len(w.preservedParts))
	for name := range w.preservedParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if seen[name] {
			continue
		}
		part := w.preservedParts[name]
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
