package pptx

import (
	"sort"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// ActiveX content types (not defined in the opc package).
const (
	contentTypeActiveXXML = "application/vnd.ms-office.activeX+xml"
	contentTypeActiveXBin = "application/vnd.ms-office.activeX"
)

// ActiveXControl is an ActiveX control embedded in a presentation: the ax:ocx
// control part (ppt/.../activeXN.xml) plus its persistence binary
// (activeXN.bin). spine reads, enumerates, and preserves these parts verbatim;
// authoring the ActiveX persistence binary is out of scope.
type ActiveXControl struct {
	// Name is the OPC part name of the control's ax:ocx XML part.
	Name string
	// ContentType is that part's content type (application/vnd.ms-office.activeX+xml).
	ContentType string
	// Data is the ax:ocx XML, carried verbatim.
	Data []byte
	// ClassID is the control server's COM class id (e.g.
	// "{8BD21D40-EC42-11CE-9E0D-00AA006002F3}"), best-effort from the part root.
	ClassID string
	// Persistence names how the control state is stored (e.g. "persistPropertyBag").
	Persistence string
	// BinaryName is the OPC part name of the control's persistence binary
	// (activeXN.bin), or "" when the control declares none.
	BinaryName string
	// BinaryData is the persistence binary, carried verbatim.
	BinaryData []byte
}

// ActiveXControls returns the presentation's ActiveX controls, ordered by part
// name for determinism. Controls are located by their ax:ocx content type (or
// an activeX/ path) across all preserved parts; each control's persistence
// binary is resolved through the control part's relationships, falling back to
// the sibling .bin part. Extraction is read-only and leaves every part
// byte-for-byte unchanged on a subsequent save.
func (p *Presentation) ActiveXControls() []ActiveXControl {
	names := make([]string, 0, len(p.otherParts))
	for name := range p.otherParts {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []ActiveXControl
	for _, name := range names {
		part := p.otherParts[name]
		lower := strings.ToLower(name)
		isActiveX := part.ContentType == contentTypeActiveXXML ||
			(strings.Contains(lower, "/activex/") && strings.HasSuffix(lower, ".xml"))
		if !isActiveX {
			continue
		}
		classID, persistence := coxml.ExtractActiveXControlInfo(part.Data)
		ctrl := ActiveXControl{
			Name:        name,
			ContentType: part.ContentType,
			Data:        part.Data,
			ClassID:     classID,
			Persistence: persistence,
		}
		if binName, binData := p.resolveActiveXBinary(name); binName != "" {
			ctrl.BinaryName = binName
			ctrl.BinaryData = binData
		}
		out = append(out, ctrl)
	}
	return out
}

// resolveActiveXBinary finds an ActiveX control part's persistence binary: first
// through a relationship of the control part whose target is a .bin, otherwise
// the sibling part with the .bin extension.
func (p *Presentation) resolveActiveXBinary(xmlPart string) (string, []byte) {
	for _, rel := range p.relationships[xmlPart] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		target := opc.ResolvePartName(xmlPart, rel.Target)
		if part, ok := p.otherParts[target]; ok && (strings.HasSuffix(strings.ToLower(target), ".bin") || part.ContentType == contentTypeActiveXBin) {
			return target, part.Data
		}
	}
	if i := strings.LastIndex(strings.ToLower(xmlPart), ".xml"); i >= 0 {
		sibling := xmlPart[:i] + ".bin"
		if part, ok := p.otherParts[sibling]; ok {
			return sibling, part.Data
		}
	}
	return "", nil
}
