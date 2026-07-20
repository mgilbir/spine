package xlsx

import (
	"path"

	"github.com/mgilbir/spine/opc"
)

// VBA (Visual Basic for Applications) macros travel in a single binary part,
// vbaProject.bin, referenced from the workbook part. The part is an opaque
// MS-OVBA/CFB blob: spine carries it verbatim and never parses or executes its
// contents. Injecting a project taken from another file transplants that
// source's macros — and its trust — unchanged, so treat injected bytes as
// untrusted code from their origin.

// resolveVBAPartName returns the vbaProject.bin part name for this workbook:
// the target of the workbook part's existing VBA relationship when present, or
// the conventional "<mainDir>/vbaProject.bin" otherwise.
func (w *Workbook) resolveVBAPartName() string {
	main := w.mainPart()
	for _, rel := range w.relationships[main] {
		if rel != nil && rel.Type == opc.RelTypeVBAProject && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(main, rel.Target)
		}
	}
	return path.Dir(main) + "/vbaProject.bin"
}

// vbaRelID returns the ID of the workbook part's VBA relationship, or "".
func (w *Workbook) vbaRelID() string {
	for _, rel := range w.relationships[w.mainPart()] {
		if rel != nil && rel.Type == opc.RelTypeVBAProject {
			return rel.ID
		}
	}
	return ""
}

// HasMacros reports whether the workbook carries a VBA project (vbaProject.bin),
// accounting for a project injected or removed in this session.
func (w *Workbook) HasMacros() bool {
	if w.vbaModified {
		return !w.vbaRemove
	}
	_, ok := w.preservedParts[w.resolveVBAPartName()]
	return ok
}

// VBAProject returns the raw bytes of the workbook's VBA project part
// (vbaProject.bin), or nil if the workbook carries no macros. The bytes are the
// opaque MS-OVBA/CFB blob exactly as stored; spine does not parse them.
func (w *Workbook) VBAProject() []byte {
	if w.vbaModified {
		return w.vbaData
	}
	if part, ok := w.preservedParts[w.resolveVBAPartName()]; ok {
		return part.Data
	}
	return nil
}

// SetVBAProject injects or replaces the workbook's VBA project with the given
// vbaProject.bin bytes, wiring the content-type override and the workbook
// relationship and flipping the main part to the macro-enabled flavor (.xlsm /
// .xltm) when it is not already macro-enabled. The bytes are stored as-is and
// written verbatim on save.
//
// Security: the bytes are executable VBA carried opaquely. Injecting a project
// extracted from another document transplants that document's macros and their
// trust; only inject bytes from a source you trust.
func (w *Workbook) SetVBAProject(data []byte) {
	name := w.resolveVBAPartName()
	w.vbaPartName = name
	w.vbaData = append([]byte(nil), data...)
	w.vbaRemove = false
	w.vbaModified = true

	main := w.mainPart()
	w.relationships[main] = ensureRelationship(w.relationships[main], opc.RelTypeVBAProject, path.Base(name))
	w.flavor = opc.MacroFlavor(w.Flavor())
}

// RemoveVBAProject removes the workbook's VBA project part, dropping its
// content-type override and workbook relationship and flipping the main part
// back to the regular (non-macro) flavor. It is a no-op on a workbook that
// carries no macros.
func (w *Workbook) RemoveVBAProject() {
	if !w.HasMacros() {
		return
	}
	name := w.resolveVBAPartName()
	w.vbaPartName = name
	w.vbaData = nil
	w.vbaRemove = true
	w.vbaModified = true

	main := w.mainPart()
	if id := w.vbaRelID(); id != "" {
		w.relationships[main] = removeRelationshipByID(w.relationships[main], id)
	}
	delete(w.preservedParts, name)
	w.flavor = opc.PlainFlavor(w.Flavor())
}

// writeVBAProject writes or drops the VBA project part during save. It is a
// no-op unless the session injected, replaced, or removed the project. On
// removal it clears the stale content-type override from the writer's (cloned)
// content types; on injection/replacement it writes the part, which registers
// the override.
func (w *Workbook) writeVBAProject(writer *opc.Writer) error {
	if !w.vbaModified {
		return nil
	}
	if w.vbaRemove {
		if writer.ContentTypes != nil {
			writer.ContentTypes.RemoveOverride(w.vbaPartName)
		}
		return nil
	}
	return writer.WritePart(w.vbaPartName, opc.ContentTypeVBAProject, w.vbaData)
}

// removeRelationshipByID returns rels without the relationship of the given ID.
func removeRelationshipByID(rels []*opc.Relationship, id string) []*opc.Relationship {
	out := rels[:0]
	for _, rel := range rels {
		if rel != nil && rel.ID == id {
			continue
		}
		out = append(out, rel)
	}
	return out
}
