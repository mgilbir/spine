package pptx

import (
	"fmt"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// presentationPartName is the OPC part name of the main presentation part;
// package-level relationships (including the VBA project relationship) hang off
// it.
const presentationPartName = "/ppt/presentation.xml"

// vbaProjectPartName is the conventional VBA project part name for a
// presentation.
const vbaProjectPartName = "/ppt/vbaProject.bin"

// VBA (Visual Basic for Applications) macros travel in a single binary part,
// vbaProject.bin, referenced from the presentation part. The part is an opaque
// MS-OVBA/CFB blob: spine carries it verbatim and never parses or executes its
// contents. Injecting a project taken from another file transplants that
// source's macros — and its trust — unchanged, so treat injected bytes as
// untrusted code from their origin.

// resolveVBAPartName returns the vbaProject.bin part name for this
// presentation: the target of the presentation part's existing VBA
// relationship when present, or the conventional /ppt/vbaProject.bin otherwise.
func (p *Presentation) resolveVBAPartName() string {
	for _, rel := range p.relationships[presentationPartName] {
		if rel != nil && rel.Type == opc.RelTypeVBAProject && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(presentationPartName, rel.Target)
		}
	}
	return vbaProjectPartName
}

// vbaRelID returns the ID of the presentation part's VBA relationship, or "".
func (p *Presentation) vbaRelID() string {
	for _, rel := range p.relationships[presentationPartName] {
		if rel != nil && rel.Type == opc.RelTypeVBAProject {
			return rel.ID
		}
	}
	return ""
}

// HasMacros reports whether the presentation carries a VBA project
// (vbaProject.bin), accounting for a project injected or removed this session.
func (p *Presentation) HasMacros() bool {
	_, ok := p.otherParts[p.resolveVBAPartName()]
	return ok
}

// VBAProject returns the raw bytes of the presentation's VBA project part
// (vbaProject.bin), or nil if the presentation carries no macros. The bytes are
// the opaque MS-OVBA/CFB blob exactly as stored; spine does not parse them.
func (p *Presentation) VBAProject() []byte {
	if part, ok := p.otherParts[p.resolveVBAPartName()]; ok {
		return part.Data
	}
	return nil
}

// SetVBAProject injects or replaces the presentation's VBA project with the
// given vbaProject.bin bytes, wiring the content-type override and the
// presentation relationship and flipping the main part to the macro-enabled
// flavor (.pptm / .ppsm / .potm) when it is not already macro-enabled. The
// bytes are stored as-is and written verbatim on save.
//
// Security: the bytes are executable VBA carried opaquely. Injecting a project
// extracted from another document transplants that document's macros and their
// trust; only inject bytes from a source you trust.
func (p *Presentation) SetVBAProject(data []byte) {
	name := p.resolveVBAPartName()
	p.otherParts[name] = &coxml.RawPart{
		ContentType: opc.ContentTypeVBAProject,
		Data:        append([]byte(nil), data...),
	}
	delete(p.removedParts, name)

	if p.vbaRelID() == "" {
		p.relationships[presentationPartName] = append(p.relationships[presentationPartName], &opc.Relationship{
			ID:     fmt.Sprintf("rId%d", p.nextPresentationRelID()),
			Type:   opc.RelTypeVBAProject,
			Target: "vbaProject.bin",
		})
	}
	p.flavor = opc.MacroFlavor(p.Flavor())
}

// RemoveVBAProject removes the presentation's VBA project part, dropping its
// content-type override and presentation relationship and flipping the main
// part back to the regular (non-macro) flavor. It is a no-op on a presentation
// that carries no macros.
func (p *Presentation) RemoveVBAProject() {
	name := p.resolveVBAPartName()
	if _, ok := p.otherParts[name]; !ok {
		return
	}
	if id := p.vbaRelID(); id != "" {
		rels := p.relationships[presentationPartName]
		for i, rel := range rels {
			if rel != nil && rel.ID == id {
				p.relationships[presentationPartName] = append(rels[:i], rels[i+1:]...)
				break
			}
		}
	}
	delete(p.otherParts, name)
	p.markPartRemoved(name)
	p.flavor = opc.PlainFlavor(p.Flavor())
}

// nextPresentationRelID returns a relationship id not used by any existing
// presentation relationship, seeding past both p.nextRelID and the highest id
// currently on the presentation part's relationships.
func (p *Presentation) nextPresentationRelID() int {
	max := p.nextRelID - 1
	for _, rel := range p.relationships[presentationPartName] {
		if rel == nil {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(rel.ID, "rId%d", &id); err == nil && id > max {
			max = id
		}
	}
	p.nextRelID = max + 2
	return max + 1
}
