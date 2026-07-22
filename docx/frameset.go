package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
)

// defaultWebSettingsPart is the conventional web-settings part name, used when
// no webSettings relationship names another. Framesets (a legacy web-layout
// feature) live in this part as a w:frameset tree.
const defaultWebSettingsPart = "/word/webSettings.xml"

// relTypeFrame is the relationship type linking a frame (w:sourceFileName) to
// the document it displays. Declared locally so the opc package stays free of
// WordprocessingML-specific constants.
const relTypeFrame = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/frame"

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

// FramesetDef describes a frameset (a w:frameset) to author with
// Document.SetFrameset: a window split into rows or columns of nested framesets
// and leaf frames. Layout is the split direction ("rows", "cols", or "none");
// Size is the child-size specification (w:sz, e.g. "*,240"); Title labels the
// split. Framesets are nested splits; Frames are the leaf frames under this
// split.
type FramesetDef struct {
	Size      string
	Layout    string
	Title     string
	Framesets []FramesetDef
	Frames    []FrameDef
}

// FrameDef describes one leaf frame (a w:frame) of a frameset. Name and Title
// label the frame; Size is its size (w:sz); Scrollbar is "on", "off", or "auto".
// SourceTarget is the document the frame displays: SetFrameset creates an
// external frame relationship to it (a w:sourceFileName r:id), matching how a
// web-layout frameset references its HTML sources.
type FrameDef struct {
	Name         string
	Title        string
	Size         string
	Scrollbar    string
	SourceTarget string
}

// SetFrameset authors the document's web-layout frameset, writing it to the
// web-settings part (word/webSettings.xml) on save. When the document has no
// web-settings part one is created, along with its document relationship and
// content-type override; when it already has one, the existing frameset subtree
// is replaced (or a frameset is inserted) while every other setting in the part
// is preserved verbatim. Each frame's SourceTarget becomes an external frame
// relationship in the web-settings part's .rels.
//
// An unmodified web-settings part round-trips byte-for-byte until the first
// SetFrameset call.
func (d *Document) SetFrameset(def FramesetDef) error {
	if err := validateFramesetDef(def); err != nil {
		return err
	}
	d.pendingFrameset = &def
	d.framesetModified = true
	return nil
}

// validateFramesetDef rejects out-of-range layout and scrollbar values before
// they reach the serializer, so a caller learns of the mistake at the call
// site rather than producing a schema-invalid part.
func validateFramesetDef(def FramesetDef) error {
	switch def.Layout {
	case "", "rows", "cols", "none":
	default:
		return fmt.Errorf("docx: SetFrameset: invalid layout %q (want rows, cols, or none)", def.Layout)
	}
	for _, fr := range def.Frames {
		switch fr.Scrollbar {
		case "", "on", "off", "auto":
		default:
			return fmt.Errorf("docx: SetFrameset: invalid scrollbar %q (want on, off, or auto)", fr.Scrollbar)
		}
	}
	for _, child := range def.Framesets {
		if err := validateFramesetDef(child); err != nil {
			return err
		}
	}
	return nil
}

// writeFramesetPart writes the web-settings part when the session authored a
// frameset. The frameset subtree and its frame relationships are generated from
// the pending definition; an existing part's other settings are preserved
// verbatim, and a fresh part is generated when the document had none.
func (d *Document) writeFramesetPart(writer *opc.Writer) error {
	if !d.framesetModified || d.pendingFrameset == nil {
		return nil
	}
	wsName := d.webSettingsPartName()

	var raw []byte
	if part, ok := d.preservedParts[wsName]; ok {
		raw = part.Data
	}

	wPrefix, rPrefix, rDeclared := "w", "r", true
	if raw != nil {
		if pw, pr, decl, ok := webSettingsPrefixes(raw); ok {
			wPrefix, rPrefix, rDeclared = pw, pr, decl
		}
	}

	// Generate the frameset fragment and collect the frame relationships,
	// seeding relationship ids past the ones already on the part.
	seed := maxRelIDNumber(d.relationships[wsName])
	var frameRels []*opc.Relationship
	fragment := marshalFramesetFragment(wPrefix, rPrefix, *d.pendingFrameset, &seed, &frameRels)

	var data []byte
	if raw == nil {
		data = marshalNewWebSettings(wPrefix, rPrefix, fragment)
	} else {
		data = buildExistingWebSettings(raw, fragment, wPrefix, rPrefix, rDeclared, len(frameRels) > 0)
	}
	if err := writer.WritePart(wsName, opc.ContentTypeDocWebSettings, data); err != nil {
		return err
	}
	d.ensureDocRelationship(opc.RelTypeWebSettings, relativePartTarget(d.mainPart(), wsName))

	// Rewrite the part's .rels: keep any non-frame relationships and append the
	// authored frame relationships (the previous frame relationships, if any, are
	// dropped along with the frameset they described).
	var rels []*opc.Relationship
	for _, r := range d.relationships[wsName] {
		if r.Type != relTypeFrame {
			rels = append(rels, r)
		}
	}
	rels = append(rels, frameRels...)
	if len(rels) > 0 {
		if err := writer.WritePartRelationships(wsName, rels); err != nil {
			return err
		}
	}
	return nil
}

// maxRelIDNumber returns the highest numeric rId suffix among rels, or 0.
func maxRelIDNumber(rels []*opc.Relationship) int {
	max := 0
	for _, r := range rels {
		if n := relIDNumber(r.ID); n > max {
			max = n
		}
	}
	return max
}

// marshalFramesetFragment serializes a frameset subtree (no XML header, no root
// element) and appends a frame relationship for every frame that names a source,
// allocating ids from *counter.
func marshalFramesetFragment(wPrefix, rPrefix string, def FramesetDef, counter *int, rels *[]*opc.Relationship) []byte {
	b := xmlb.NewBuilder()
	b.SetCollapseEmptyElements(true)
	writeFrameset(b, wPrefix, rPrefix, def, counter, rels)
	_ = b.Finish()
	return b.Bytes()
}

// writeFrameset serializes a w:frameset element under wPrefix, following the
// CT_Frameset child order (sz, frameLayout, title, then nested framesets and
// frames).
func writeFrameset(b *xmlb.Builder, wPrefix, rPrefix string, def FramesetDef, counter *int, rels *[]*opc.Relationship) {
	val := func(local, value string) {
		b.EmptyElementLiteral(wPrefix, local, xmlb.Attr{Name: wPrefix + ":val", Value: value})
	}
	b.StartElementLiteral(wPrefix, "frameset", nil)
	if def.Size != "" {
		val("sz", def.Size)
	}
	if def.Layout != "" {
		val("frameLayout", def.Layout)
	}
	if def.Title != "" {
		val("title", def.Title)
	}
	for _, child := range def.Framesets {
		writeFrameset(b, wPrefix, rPrefix, child, counter, rels)
	}
	for _, fr := range def.Frames {
		writeFrame(b, wPrefix, rPrefix, fr, counter, rels)
	}
	b.EndElementLiteral(wPrefix, "frameset")
}

// writeFrame serializes a w:frame element under wPrefix, following the CT_Frame
// child order (sz, name, title, sourceFileName, scrollbar). A named source
// yields an external frame relationship whose id is written as the
// sourceFileName's r:id.
func writeFrame(b *xmlb.Builder, wPrefix, rPrefix string, fr FrameDef, counter *int, rels *[]*opc.Relationship) {
	val := func(local, value string) {
		b.EmptyElementLiteral(wPrefix, local, xmlb.Attr{Name: wPrefix + ":val", Value: value})
	}
	b.StartElementLiteral(wPrefix, "frame", nil)
	if fr.Size != "" {
		val("sz", fr.Size)
	}
	if fr.Name != "" {
		val("name", fr.Name)
	}
	if fr.Title != "" {
		val("title", fr.Title)
	}
	if fr.SourceTarget != "" {
		*counter++
		id := fmt.Sprintf("rId%d", *counter)
		*rels = append(*rels, &opc.Relationship{
			ID:         id,
			Type:       relTypeFrame,
			Target:     fr.SourceTarget,
			TargetMode: opc.TargetModeExternal,
		})
		b.EmptyElementLiteral(wPrefix, "sourceFileName", xmlb.Attr{Name: rPrefix + ":id", Value: id})
	}
	if fr.Scrollbar != "" {
		val("scrollbar", fr.Scrollbar)
	}
	b.EndElementLiteral(wPrefix, "frame")
}

// marshalNewWebSettings builds a fresh web-settings part whose only content is
// the given frameset fragment.
func marshalNewWebSettings(wPrefix, rPrefix string, fragment []byte) []byte {
	b := xmlb.NewBuilder()
	b.WriteHeader()
	b.StartElementLiteral(wPrefix, "webSettings", nil,
		xmlb.Attr{Name: "xmlns:" + wPrefix, Value: xmlb.NSWordprocessingML},
		xmlb.Attr{Name: "xmlns:" + rPrefix, Value: xmlb.NSOfficeDocumentRels},
	)
	b.WriteRaw(fragment)
	b.EndElementLiteral(wPrefix, "webSettings")
	_ = b.Finish()
	return b.Bytes()
}

// buildExistingWebSettings splices the frameset fragment into an existing
// web-settings part, replacing an existing top-level frameset subtree or
// inserting one as the first child, and injecting an xmlns:r declaration on the
// root when frames need it and the part lacks one. Every other byte is
// preserved. A self-closing (empty) root falls back to a freshly generated part.
func buildExistingWebSettings(raw, fragment []byte, wPrefix, rPrefix string, rDeclared, hasFrames bool) []byte {
	_, gt, selfClosing, ok := rootStartTagBounds(raw)
	if !ok || selfClosing {
		return marshalNewWebSettings(wPrefix, rPrefix, fragment)
	}
	root := raw
	if hasFrames && !rDeclared {
		decl := []byte(` xmlns:` + rPrefix + `="` + xmlb.NSOfficeDocumentRels + `"`)
		root = spliceRange(raw, gt, gt, decl)
	}
	if s, e, found := framesetSpan(root); found {
		return spliceRange(root, s, e, fragment)
	}
	_, gt2, _, _ := rootStartTagBounds(root)
	insert := gt2 + 1
	return spliceRange(root, insert, insert, fragment)
}

// spliceRange returns raw with the byte range [s,e) replaced by ins.
func spliceRange(raw []byte, s, e int, ins []byte) []byte {
	out := make([]byte, 0, len(raw)-(e-s)+len(ins))
	out = append(out, raw[:s]...)
	out = append(out, ins...)
	out = append(out, raw[e:]...)
	return out
}

// webSettingsPrefixes reads the namespace prefixes bound to the WordprocessingML
// and relationships namespaces on the part's root element, reporting whether the
// relationships namespace is declared there. Defaults ("w", "r") stand in for an
// undeclared namespace.
func webSettingsPrefixes(raw []byte) (wPrefix, rPrefix string, rDeclared, ok bool) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", "", false, false
		}
		se, isStart := tok.(xml.StartElement)
		if !isStart {
			continue
		}
		wPrefix, rPrefix = "w", "r"
		for _, a := range se.Attr {
			if a.Name.Space != "xmlns" {
				continue
			}
			switch a.Value {
			case xmlb.NSWordprocessingML:
				wPrefix = a.Name.Local
			case xmlb.NSOfficeDocumentRels:
				rPrefix = a.Name.Local
				rDeclared = true
			}
		}
		return wPrefix, rPrefix, rDeclared, true
	}
}

// rootStartTagBounds locates the document element's start tag: gt is the offset
// of its closing '>', selfClosing reports a '/>' close. The XML declaration and
// any leading processing instructions are skipped.
func rootStartTagBounds(raw []byte) (lt, gt int, selfClosing, ok bool) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var prev int64
	for {
		prev = dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			return 0, 0, false, false
		}
		if _, isStart := tok.(xml.StartElement); !isStart {
			continue
		}
		end := int(dec.InputOffset()) // just past the start tag's '>'
		l := int(prev)
		for l < len(raw) && raw[l] != '<' {
			l++
		}
		g := end - 1
		self := g-1 >= 0 && raw[g-1] == '/'
		return l, g, self, true
	}
}

// framesetSpan locates the top-level w:frameset child of the root element,
// returning the byte range [s,e) it occupies (s at its '<', e just past its
// end tag's '>').
func framesetSpan(raw []byte) (s, e int, ok bool) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	depth := 0
	targetDepth := -1
	start := 0
	var prev int64
	for {
		prev = dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			return 0, 0, false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if targetDepth == -1 && depth == 2 && t.Name.Local == "frameset" {
				p := int(prev)
				for p < len(raw) && raw[p] != '<' {
					p++
				}
				start = p
				targetDepth = depth
			}
		case xml.EndElement:
			if targetDepth != -1 && depth == targetDepth && t.Name.Local == "frameset" {
				return start, int(dec.InputOffset()), true
			}
			depth--
		}
	}
}
