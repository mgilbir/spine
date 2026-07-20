package xlsx

import (
	"encoding/xml"
	"sort"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

// ActiveX content types, not defined in the opc package (the control reader is
// their only consumer).
const (
	contentTypeActiveXXML = "application/vnd.ms-office.activeX+xml"
	contentTypeActiveXBin = "application/vnd.ms-office.activeX"
)

// FormControlType classifies a legacy Excel form control (the kind on the
// Developer > Insert > Form Controls palette), stored as a VML shape whose
// x:ClientData carries an ObjectType.
type FormControlType string

const (
	FormControlButton    FormControlType = "button"
	FormControlCheckBox  FormControlType = "checkbox"
	FormControlDropDown  FormControlType = "dropdown"
	FormControlListBox   FormControlType = "listbox"
	FormControlRadio     FormControlType = "radio"
	FormControlSpinner   FormControlType = "spinner"
	FormControlScrollBar FormControlType = "scrollbar"
	FormControlLabel     FormControlType = "label"
	FormControlGroupBox  FormControlType = "groupbox"
	FormControlEditBox   FormControlType = "editbox"
	FormControlDialog    FormControlType = "dialog"
	FormControlUnknown   FormControlType = "unknown"
)

// FormControl is a legacy Excel form control on a worksheet, reconstructed from
// the sheet's VML drawing (the x:ClientData shape) and, best-effort, the
// worksheet's <control> block. Extraction is read-only; the control parts
// (VML, ctrlProps) round-trip byte-for-byte on a subsequent save.
type FormControl struct {
	// Type is the control kind derived from the VML ObjectType.
	Type FormControlType
	// Name is the control's display name from the worksheet <control> block
	// (e.g. "Check Box 1"); best-effort and may be empty.
	Name string
	// LinkedCell is the cell the control's value is bound to (x:FmlaLink), such
	// as "$B$2"; empty when the control has no linked cell.
	LinkedCell string
	// SourceRange is the input range feeding a list box or dropdown (x:FmlaRange),
	// such as "$D$1:$D$5"; empty otherwise.
	SourceRange string
	// Checked reports the initial state of a checkbox or radio control.
	Checked bool
	// Anchor is the control's raw two-cell VML anchor (comma-separated
	// col,dx,row,dy pairs); preserved verbatim for callers that need placement.
	Anchor string
	// VMLPart is the OPC part name of the legacy VML drawing that hosts the
	// control shape (e.g. "/xl/drawings/vmlDrawing1.vml").
	VMLPart string
	// CtrlPropPart is the OPC part name of the control's properties part
	// (xl/ctrlProps/ctrlPropN.xml), resolved through the worksheet <control>
	// relationship; best-effort and may be empty.
	CtrlPropPart string
}

// FormControls returns the legacy form controls on the sheet, in the order
// their shapes appear in the sheet's VML drawing. Buttons, checkboxes,
// dropdowns, list boxes, option buttons, spinners, and scroll bars are all
// reported with their linked cell. Extraction is read-only.
func (s *Sheet) FormControls() []FormControl {
	if s.workbook == nil || s.worksheet == nil || s.worksheet.LegacyDrawing == nil {
		return nil
	}
	vmlPart := s.resolveRelTarget(s.partName, s.worksheet.LegacyDrawing.RID)
	if vmlPart == "" {
		return nil
	}
	part, ok := s.workbook.preservedParts[vmlPart]
	if !ok {
		return nil
	}
	var vml vmlDrawing
	if err := xml.Unmarshal(part.Data, &vml); err != nil {
		return nil
	}
	blocks := s.controlBlocks()

	var out []FormControl
	for i := range vml.Shapes {
		cd := vml.Shapes[i].ClientData
		if cd == nil {
			continue
		}
		typ := formControlType(cd.ObjectType)
		if typ == FormControlUnknown && cd.ObjectType == "" {
			continue // a plain VML shape (e.g. a comment), not a control
		}
		fc := FormControl{
			Type:        typ,
			LinkedCell:  strings.TrimSpace(cd.FmlaLink),
			SourceRange: strings.TrimSpace(cd.FmlaRange),
			Checked:     strings.TrimSpace(cd.Checked) == "1",
			Anchor:      strings.TrimSpace(cd.Anchor),
			VMLPart:     vmlPart,
		}
		if blk, ok := blocks[shapeNumericID(vml.Shapes[i].ID)]; ok {
			fc.Name = blk.name
			fc.CtrlPropPart = s.resolveRelTarget(s.partName, blk.rid)
		}
		out = append(out, fc)
	}
	return out
}

// controlBlock is the worksheet <control> metadata joined to a VML shape by its
// numeric shape id.
type controlBlock struct {
	name string
	rid  string
}

// controlBlocks scans the raw worksheet XML for <control> elements (which may be
// nested inside mc:AlternateContent), keyed by numeric shape id, so their name
// and ctrlProp relationship can be attached to the matching VML shape.
func (s *Sheet) controlBlocks() map[string]controlBlock {
	out := map[string]controlBlock{}
	if s.workbook == nil {
		return out
	}
	part, ok := s.workbook.preservedParts[s.partName]
	if !ok {
		return out
	}
	dec := xml.NewDecoder(strings.NewReader(string(part.Data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "control" {
			continue
		}
		var blk controlBlock
		var shapeID string
		for _, a := range se.Attr {
			switch a.Name.Local {
			case "name":
				blk.name = a.Value
			case "shapeId":
				shapeID = a.Value
			case "id":
				if a.Name.Space == nsR || a.Name.Space == "r" {
					blk.rid = a.Value
				}
			}
		}
		if shapeID != "" {
			out[shapeID] = blk
		}
	}
	return out
}

// formControlType maps a VML x:ClientData ObjectType to a FormControlType.
func formControlType(objectType string) FormControlType {
	switch objectType {
	case "Button":
		return FormControlButton
	case "Checkbox":
		return FormControlCheckBox
	case "Drop":
		return FormControlDropDown
	case "List":
		return FormControlListBox
	case "Radio":
		return FormControlRadio
	case "Spin":
		return FormControlSpinner
	case "Scroll":
		return FormControlScrollBar
	case "Label":
		return FormControlLabel
	case "GBox":
		return FormControlGroupBox
	case "EditBox":
		return FormControlEditBox
	case "Dialog":
		return FormControlDialog
	default:
		return FormControlUnknown
	}
}

// shapeNumericID extracts the trailing digits of a VML shape id
// (e.g. "_x0000_s1025" -> "1025"), which equal the worksheet <control> shapeId.
func shapeNumericID(id string) string {
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	return id[i:]
}

// --- minimal VML parse model (read-only) ---

// vmlDrawing decodes a legacy VML drawing part, matching elements by local name
// so the v:/o:/x: prefixes need not be bound. Only the shapes and their
// x:ClientData control descriptors are read.
type vmlDrawing struct {
	Shapes []vmlShape `xml:"shape"`
}

type vmlShape struct {
	ID         string         `xml:"id,attr"`
	ClientData *vmlClientData `xml:"ClientData"`
}

type vmlClientData struct {
	ObjectType string `xml:"ObjectType,attr"`
	Anchor     string `xml:"Anchor"`
	FmlaLink   string `xml:"FmlaLink"`
	FmlaRange  string `xml:"FmlaRange"`
	Checked    string `xml:"Checked"`
}

// --- ActiveX ---

// ActiveXControl is an ActiveX control embedded in a workbook: the ax:ocx
// control part (xl/activeX/activeXN.xml) plus its persistence binary
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

// ActiveXControls returns the workbook's ActiveX controls, ordered by part name
// for determinism. Controls are located by their ax:ocx content type across all
// preserved parts; each control's persistence binary is resolved through the
// control part's relationships (falling back to the sibling .bin part).
// Extraction is read-only and leaves every part byte-for-byte unchanged.
func (w *Workbook) ActiveXControls() []ActiveXControl {
	return collectActiveXControls(w.preservedParts, w.relationships)
}

// collectActiveXControls is the shared ActiveX enumeration over a preserved-part
// map and its relationships, used by the xlsx reader (and mirrored by docx/pptx).
func collectActiveXControls(parts map[string]*coxml.RawPart, rels map[string][]*opc.Relationship) []ActiveXControl {
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []ActiveXControl
	for _, name := range names {
		part := parts[name]
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
		if binName, binData := resolveActiveXBinary(name, parts, rels); binName != "" {
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
func resolveActiveXBinary(xmlPart string, parts map[string]*coxml.RawPart, rels map[string][]*opc.Relationship) (string, []byte) {
	for _, rel := range rels[xmlPart] {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		target := opc.ResolvePartName(xmlPart, rel.Target)
		if part, ok := parts[target]; ok && (strings.HasSuffix(strings.ToLower(target), ".bin") || part.ContentType == contentTypeActiveXBin) {
			return target, part.Data
		}
	}
	if i := strings.LastIndex(strings.ToLower(xmlPart), ".xml"); i >= 0 {
		sibling := xmlPart[:i] + ".bin"
		if part, ok := parts[sibling]; ok {
			return sibling, part.Data
		}
	}
	return "", nil
}
