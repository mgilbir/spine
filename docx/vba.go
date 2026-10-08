package docx

import (
	"fmt"
	"path"

	"github.com/mgilbir/spine/opc"
)

// VBA (Visual Basic for Applications) macros travel in a single binary part,
// vbaProject.bin, referenced from the main document part. The part is an opaque
// MS-OVBA/CFB blob: spine carries it verbatim and never parses or executes its
// contents. Injecting a project taken from another file transplants that
// source's macros — and its trust — unchanged, so treat injected bytes as
// untrusted code from their origin.

// resolveVBAPartName returns the vbaProject.bin part name for this document:
// the target of the main part's existing VBA relationship when present, or the
// conventional "<mainDir>/vbaProject.bin" otherwise.
func (d *Document) resolveVBAPartName() string {
	main := d.mainPart()
	for _, rel := range d.relationships[main] {
		if rel != nil && rel.Type == opc.RelTypeVBAProject && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(main, rel.Target)
		}
	}
	return path.Dir(main) + "/vbaProject.bin"
}

// vbaRelID returns the ID of the main part's VBA relationship, or "" if none.
func (d *Document) vbaRelID() string {
	for _, rel := range d.relationships[d.mainPart()] {
		if rel != nil && rel.Type == opc.RelTypeVBAProject {
			return rel.ID
		}
	}
	return ""
}

// HasMacros reports whether the document carries a VBA project (vbaProject.bin),
// accounting for a project injected or removed in this session.
func (d *Document) HasMacros() bool {
	if d.vbaModified {
		return !d.vbaRemove
	}
	name := d.resolveVBAPartName()
	_, ok := d.preservedParts[name]
	return ok
}

// VBAProject returns the raw bytes of the document's VBA project part
// (vbaProject.bin), or nil if the document carries no macros. The bytes are the
// opaque MS-OVBA/CFB blob exactly as stored; spine does not parse them.
func (d *Document) VBAProject() []byte {
	if d.vbaModified {
		return d.vbaData
	}
	name := d.resolveVBAPartName()
	if part, ok := d.preservedParts[name]; ok {
		return part.Data
	}
	return nil
}

// SetVBAProject injects or replaces the document's VBA project with the given
// vbaProject.bin bytes, wiring the content-type override and the main-part
// relationship and flipping the main part to the macro-enabled flavor (.docm /
// .dotm) when it is not already macro-enabled. The bytes are stored as-is and
// written verbatim on save.
//
// Replacing a project drops the signatures of the one it replaces (the
// project's signature relationships and the parts they target, unless another
// relationship still targets them): they sign the old bytes, so Office would
// report them as invalid. Other parts of the project's relationships, such as
// Word's macro data (vbaData.xml), are kept.
//
// Security: the bytes are executable VBA carried opaquely. Injecting a project
// extracted from another document transplants that document's macros and their
// trust; only inject bytes from a source you trust.
func (d *Document) SetVBAProject(data []byte) {
	name := d.resolveVBAPartName()
	d.dropVBASignatures(name)
	d.vbaPartName = name
	d.vbaData = append([]byte(nil), data...)
	d.vbaRemove = false
	d.markVBAModified()

	// Wire the main-part relationship if absent (idempotent across repeated
	// calls and repeated saves).
	if d.vbaRelID() == "" {
		d.addDocRelationship(&opc.Relationship{
			ID:     fmt.Sprintf("rId%d", d.nextRelID()),
			Type:   opc.RelTypeVBAProject,
			Target: path.Base(name),
		})
	}
	d.flavor = opc.MacroFlavor(d.Flavor())
}

// RemoveVBAProject removes the document's VBA project part, dropping its
// content-type override and main-part relationship and flipping the main part
// back to the regular (non-macro) flavor. The parts the project's own
// relationships target — its signatures and Word's macro data (vbaData.xml) —
// are removed with it, along with the project's .rels, unless another
// relationship in the package still targets them. It is a no-op on a document
// that carries no macros.
func (d *Document) RemoveVBAProject() {
	if !d.HasMacros() {
		return
	}
	name := d.resolveVBAPartName()
	d.vbaPartName = name
	d.vbaData = nil
	d.vbaRemove = true
	d.markVBAModified()

	if id := d.vbaRelID(); id != "" {
		d.removeDocRelationship(id)
	}
	delete(d.preservedParts, name)
	delete(d.otherParts, name)
	d.dropVBADependents(name)
	d.flavor = opc.PlainFlavor(d.Flavor())
}

// dropVBADependents drops the removed project's .rels and the parts its
// relationships targeted, each only when no other relationship targets it.
func (d *Document) dropVBADependents(name string) {
	var targets []string
	for _, rel := range d.relationships[name] {
		if rel != nil && rel.TargetMode != opc.TargetModeExternal {
			targets = append(targets, opc.ResolvePartName(name, rel.Target))
		}
	}
	delete(d.relationships, name)
	if relsName := opc.GetRelationshipsPartName(name); d.preservedParts[relsName] != nil {
		if d.droppedParts == nil {
			d.droppedParts = make(map[string]bool)
		}
		d.droppedParts[relsName] = true
	}
	for _, target := range targets {
		d.dropUntargetedPreservedPart(target)
	}
}

// writeVBAProject writes or drops the VBA project part during save. It is a
// no-op unless the session injected, replaced, or removed the project. On
// removal it clears the stale content-type override from the writer's (cloned)
// content types; on injection/replacement it writes the part, which registers
// the override.
func (d *Document) writeVBAProject(writer *opc.Writer) error {
	if !d.vbaModified {
		return nil
	}
	if d.vbaRemove {
		if writer.ContentTypes != nil {
			writer.ContentTypes.RemoveOverride(d.vbaPartName)
		}
		return nil
	}
	return writer.WritePart(d.vbaPartName, opc.ContentTypeVBAProject, d.vbaData)
}

// dropVBASignatures removes the project's signature relationships and drops
// the parts they targeted when nothing else targets them. The project's .rels
// is then written from the model, or left out once empty.
func (d *Document) dropVBASignatures(name string) {
	var targets []string
	kept := make([]*opc.Relationship, 0, len(d.relationships[name]))
	for _, rel := range d.relationships[name] {
		if rel == nil || !opc.IsVBASignatureRelType(rel.Type) {
			kept = append(kept, rel)
			continue
		}
		if rel.TargetMode != opc.TargetModeExternal {
			targets = append(targets, opc.ResolvePartName(name, rel.Target))
		}
	}
	if len(kept) == len(d.relationships[name]) {
		return
	}
	d.relationships[name] = kept
	if d.rewrittenRels == nil {
		d.rewrittenRels = make(map[string]bool)
	}
	d.rewrittenRels[name] = true
	for _, target := range targets {
		d.dropUntargetedPreservedPart(target)
	}
}
