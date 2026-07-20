package docx

import (
	"encoding/xml"
	"strings"

	"github.com/mgilbir/spine/opc"
)

// defaultWebSettingsPart is the conventional web-settings part name, used when
// no webSettings relationship names another. Framesets (a legacy web-layout
// feature) live in this part as a w:frameset tree.
const defaultWebSettingsPart = "/word/webSettings.xml"

// Frameset is a web-layout frameset (w:frameset): a recursive split of the
// window into rows or columns of frames and nested framesets. Framesets are
// read-only in this API; the web-settings part is preserved verbatim on save.
type Frameset struct {
	size      string
	layout    string
	title     string
	framesets []*Frameset
	frames    []*Frame
}

// Size returns the frameset's size specification (w:sz), e.g. "*,240" splitting
// the space among its children, or "" when unset.
func (f *Frameset) Size() string { return f.size }

// Layout returns the split direction (w:frameLayout val): "rows", "cols", or
// "none", or "" when unset.
func (f *Frameset) Layout() string { return f.layout }

// Title returns the frameset's title (w:title), or "" when unset.
func (f *Frameset) Title() string { return f.title }

// Framesets returns the nested child framesets, in document order.
func (f *Frameset) Framesets() []*Frameset { return f.framesets }

// Frames returns the leaf frames directly under this frameset, in document
// order.
func (f *Frameset) Frames() []*Frame { return f.frames }

// Frame is a single leaf frame (w:frame) of a frameset, displaying the document
// its source relationship points to.
type Frame struct {
	name         string
	title        string
	size         string
	scrollbar    string
	sourceRID    string
	sourceTarget string
}

// Name returns the frame's name (w:name), or "" when unset.
func (f *Frame) Name() string { return f.name }

// Title returns the frame's title (w:title), or "" when unset.
func (f *Frame) Title() string { return f.title }

// Size returns the frame's size (w:sz), or "" when unset.
func (f *Frame) Size() string { return f.size }

// Scrollbar returns the frame's scrollbar setting (w:scrollbar val): "on",
// "off", or "auto", or "" when unset.
func (f *Frame) Scrollbar() string { return f.scrollbar }

// SourceID returns the relationship id (r:id) of the frame's source document
// (w:sourceFileName), or "" when unset.
func (f *Frame) SourceID() string { return f.sourceRID }

// SourceTarget returns the resolved relationship target of the frame's source
// document (the URL or part the frame displays), or "" when the relationship
// cannot be resolved.
func (f *Frame) SourceTarget() string { return f.sourceTarget }

// webSettingsPartName resolves the web-settings part, following the document's
// webSettings relationship when present and falling back to the conventional
// name.
func (d *Document) webSettingsPartName() string {
	for _, rel := range d.relationships[d.mainPart()] {
		if rel.Type == opc.RelTypeWebSettings && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(d.mainPart(), rel.Target)
		}
	}
	return defaultWebSettingsPart
}

// Frameset returns the document's top-level frameset (the window split defined
// in the web-settings part), or nil when the document is not a frameset
// document. Framesets are read-only; the web-settings part is preserved
// verbatim on save.
func (d *Document) Frameset() *Frameset {
	wsName := d.webSettingsPartName()
	part, ok := d.preservedParts[wsName]
	if !ok {
		return nil
	}
	dec := xml.NewDecoder(strings.NewReader(string(part.Data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local == "frameset" {
			fs := parseFrameset(dec, se, d.relationships[wsName])
			return fs
		}
	}
}

// parseFrameset walks a w:frameset element (already consumed as start) and its
// descendants, resolving each frame's source relationship against rels.
func parseFrameset(dec *xml.Decoder, start xml.StartElement, rels []*opc.Relationship) *Frameset {
	fs := &Frameset{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return fs
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sz":
				fs.size = attrVal(t, "val")
			case "frameLayout":
				fs.layout = attrVal(t, "val")
			case "title":
				fs.title = attrVal(t, "val")
			case "frameset":
				fs.framesets = append(fs.framesets, parseFrameset(dec, t, rels))
			case "frame":
				fs.frames = append(fs.frames, parseFrame(dec, t, rels))
			default:
				_ = dec.Skip()
			}
		case xml.EndElement:
			if t.Name.Local == "frameset" {
				return fs
			}
		}
	}
}

// parseFrame walks a w:frame element (already consumed as start), resolving its
// w:sourceFileName relationship against rels.
func parseFrame(dec *xml.Decoder, start xml.StartElement, rels []*opc.Relationship) *Frame {
	fr := &Frame{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return fr
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "name":
				fr.name = attrVal(t, "val")
			case "title":
				fr.title = attrVal(t, "val")
			case "sz":
				fr.size = attrVal(t, "val")
			case "scrollbar":
				fr.scrollbar = attrVal(t, "val")
			case "sourceFileName":
				fr.sourceRID = ridAttr(t)
				fr.sourceTarget = resolveRel(rels, fr.sourceRID)
			}
			_ = dec.Skip()
		case xml.EndElement:
			if t.Name.Local == "frame" {
				return fr
			}
		}
	}
}

// attrVal returns the value of the local-named attribute, ignoring namespace.
func attrVal(se xml.StartElement, local string) string {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// ridAttr returns the relationship id attribute (r:id), matched by its
// relationships namespace or the "r" prefix undeclared decoders leave behind.
func ridAttr(se xml.StartElement) string {
	for _, a := range se.Attr {
		if a.Name.Local == "id" && (a.Name.Space == "http://schemas.openxmlformats.org/officeDocument/2006/relationships" ||
			a.Name.Space == "r") {
			return a.Value
		}
	}
	return ""
}

// resolveRel returns the target of the relationship with the given id, or "".
func resolveRel(rels []*opc.Relationship, id string) string {
	if id == "" {
		return ""
	}
	for _, rel := range rels {
		if rel.ID == id {
			return rel.Target
		}
	}
	return ""
}
