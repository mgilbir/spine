package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/mgilbir/spine/opc"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// contentTypeGlossary is the content type of a WordprocessingML glossary
// (building blocks) part. Declared locally so the opc package stays free of
// WordprocessingML-specific constants.
const contentTypeGlossary = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"

// relTypeGlossaryDocument is the relationship type linking a document to its
// glossary (building blocks / AutoText) part. Defined locally so the opc package
// stays free of WordprocessingML-specific constants.
const relTypeGlossaryDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument"

// defaultGlossaryPart is the conventional glossary part name, used when no
// glossaryDocument relationship names another.
const defaultGlossaryPart = "/word/glossary/document.xml"

// BuildingBlock is one entry of the glossary document (a w:docPart) — a reusable
// content fragment such as an AutoText entry, a cover page, or a header/footer
// gallery item. BuildingBlocks reads the existing entries; AddBuildingBlock
// authors new ones (regenerating, or newly creating, the glossary part on
// save). A glossary part that this session never touches is preserved verbatim.
type BuildingBlock struct {
	name        string
	gallery     string
	category    string
	types       []string
	description string
	guid        string
}

// Name returns the building block's name (w:docPartPr/w:name), the identifier
// Word shows in its galleries.
func (b *BuildingBlock) Name() string { return b.name }

// Gallery returns the building block's gallery (w:category/w:gallery), e.g.
// "AutoText", "coverPg", or "placeholder".
func (b *BuildingBlock) Gallery() string { return b.gallery }

// Category returns the building block's category name (w:category/w:name), e.g.
// "General".
func (b *BuildingBlock) Category() string { return b.category }

// Types returns the building block's declared types (w:docPartPr/w:types), e.g.
// "bbPlcHdr" for a placeholder or "autoTxt" for an AutoText entry.
func (b *BuildingBlock) Types() []string { return b.types }

// Description returns the building block's description (w:docPartPr/w:description),
// or "" when unset.
func (b *BuildingBlock) Description() string { return b.description }

// GUID returns the building block's identifier (w:docPartPr/w:guid) in "{GUID}"
// form, or "" when unset.
func (b *BuildingBlock) GUID() string { return b.guid }

// glossaryPartName resolves the glossary part, following the document's
// glossaryDocument relationship when present and falling back to the
// conventional name.
func (d *Document) glossaryPartName() string {
	for _, rel := range d.relationships[d.mainPart()] {
		if rel.Type == relTypeGlossaryDocument && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(d.mainPart(), rel.Target)
		}
	}
	return defaultGlossaryPart
}

// HasGlossary reports whether the document carries a glossary (building blocks)
// part.
func (d *Document) HasGlossary() bool {
	_, ok := d.preservedParts[d.glossaryPartName()]
	return ok
}

// ctGlossaryMeta parses only the docPart properties of a glossary document,
// leaving each docPart's body (which the read API does not expose) untouched.
// The docParts are wrapped in a w:docParts container within w:glossaryDocument.
type ctGlossaryMeta struct {
	DocParts struct {
		DocPart []ctDocPartMeta `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docPart"`
	} `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docParts"`
}

type ctDocPartMeta struct {
	Pr *ctDocPartPrMeta `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docPartPr"`
}

type ctDocPartPrMeta struct {
	Name        ctValAttr  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name"`
	Description ctValAttr  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main description"`
	Guid        ctValAttr  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main guid"`
	Category    *ctDPCat   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main category"`
	Types       *ctDPTypes `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main types"`
}

type ctDPCat struct {
	Name    ctValAttr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name"`
	Gallery ctValAttr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gallery"`
}

type ctDPTypes struct {
	Type []ctValAttr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type"`
}

type ctValAttr struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// BuildingBlocks returns the document's building blocks (glossary docParts) in
// document order, including blocks added this session with AddBuildingBlock, or
// nil when the document has neither a glossary part nor any pending block. The
// existing docParts are preserved verbatim on save; the session-added ones are
// appended (mirroring where writeGlossaryPart splices them, before the closing
// </w:docParts>), so read-your-writes holds within a session.
func (d *Document) BuildingBlocks() []*BuildingBlock {
	var out []*BuildingBlock
	if part, ok := d.preservedParts[d.glossaryPartName()]; ok {
		var meta ctGlossaryMeta
		if err := xmlb.Unmarshal(part.Data, &meta); err == nil {
			for _, dp := range meta.DocParts.DocPart {
				bb := &BuildingBlock{}
				if pr := dp.Pr; pr != nil {
					bb.name = pr.Name.Val
					bb.description = pr.Description.Val
					bb.guid = pr.Guid.Val
					if pr.Category != nil {
						bb.category = pr.Category.Name.Val
						bb.gallery = pr.Category.Gallery.Val
					}
					if pr.Types != nil {
						for _, t := range pr.Types.Type {
							if t.Val != "" {
								bb.types = append(bb.types, t.Val)
							}
						}
					}
				}
				out = append(out, bb)
			}
		}
	}
	for _, def := range d.pendingBuildingBlocks {
		out = append(out, buildingBlockFromDef(def))
	}
	return out
}

// buildingBlockFromDef builds a read view of a building block queued this
// session, so BuildingBlocks() surfaces it before the document is reopened.
func buildingBlockFromDef(def BuildingBlockDef) *BuildingBlock {
	return &BuildingBlock{
		name:        def.Name,
		gallery:     def.Gallery,
		category:    def.Category,
		types:       append([]string(nil), def.Types...),
		description: def.Description,
		guid:        def.GUID,
	}
}

// BuildingBlockDef describes a building block (a glossary w:docPart) to author
// with Document.AddBuildingBlock. It mirrors the fields the read-side
// BuildingBlock accessors expose; the value type is separate because
// BuildingBlock keeps its fields behind accessors.
//
// Name is the building-block identifier Word shows in its galleries (required).
// Gallery and Category place the block in Word's building-block organizer (e.g.
// Gallery "AutoText"/"placeholder", Category "General"). Types are w:type
// values (e.g. "bbPlcHdr"). Style is an optional style-id reference. GUID is the
// block's identifier in "{GUID}" form; a fresh one is generated when empty.
//
// The authored block's body is a single empty paragraph: this API models a
// building block's metadata, not its reusable body content. An existing glossary
// part's docParts (and their bodies) are preserved verbatim; the new docPart is
// appended.
type BuildingBlockDef struct {
	Name        string
	Gallery     string
	Category    string
	Types       []string
	Style       string
	Description string
	GUID        string
}

// AddBuildingBlock appends a building block to the document's glossary,
// creating the glossary part (word/glossary/document.xml), its document
// relationship, and its content-type override when the document has none. A
// block with no Name is rejected: Word identifies a building block by its name.
// The block is assigned a fresh GUID when GUID is empty.
//
// An existing glossary part round-trips byte-for-byte until the first
// AddBuildingBlock call; from then on the new docPart is spliced in before the
// closing </w:docParts> so every existing docPart (and its body) is preserved
// verbatim.
//
// The block created here is metadata only: it registers the name, gallery,
// category, types, style and description, and its body is a single empty
// paragraph. There is no way to give a new block reusable content — the name
// "building block" notwithstanding — so it appears in Word's gallery and
// inserts nothing. Blocks that came with the opened package keep their bodies.
func (d *Document) AddBuildingBlock(def BuildingBlockDef) error {
	if def.Name == "" {
		return fmt.Errorf("docx: AddBuildingBlock: building block name must not be empty")
	}
	if def.GUID == "" {
		def.GUID = newBibGUID()
	}
	d.pendingBuildingBlocks = append(d.pendingBuildingBlocks, def)
	d.glossaryModified = true
	return nil
}

// writeGlossaryPart writes the glossary part when the session authored a
// building block. When the opened package already carried a glossary part, the
// queued docParts are spliced into its preserved bytes so existing entries stay
// byte-identical; otherwise a fresh glossary part is generated. In both cases
// the document.xml relationship and content-type override are registered when
// absent.
func (d *Document) writeGlossaryPart(writer *opc.Writer) error {
	if !d.glossaryModified || len(d.pendingBuildingBlocks) == 0 {
		return nil
	}
	name := d.glossaryPartName()
	var data []byte
	if part, ok := d.preservedParts[name]; ok {
		spliced, err := spliceDocParts(part.Data, d.pendingBuildingBlocks)
		if err != nil {
			return err
		}
		data = spliced
	} else {
		data = marshalNewGlossary(d.pendingBuildingBlocks)
	}
	if err := writer.WritePart(name, contentTypeGlossary, data); err != nil {
		return err
	}
	d.ensureDocRelationship(relTypeGlossaryDocument, relativePartTarget(d.mainPart(), name))
	return nil
}

// relativePartTarget returns target expressed relative to the directory of
// base, both absolute OPC part names (e.g. base "/word/document.xml", target
// "/word/glossary/document.xml" -> "glossary/document.xml"). It handles the
// common case where target lives under the base's directory and otherwise walks
// up with "../" segments, matching how Word writes relationship targets.
func relativePartTarget(base, target string) string {
	baseSegs := strings.Split(strings.TrimPrefix(base, "/"), "/")
	baseDir := baseSegs[:len(baseSegs)-1] // drop the file name
	targetSegs := strings.Split(strings.TrimPrefix(target, "/"), "/")
	// Drop the shared leading directory segments.
	i := 0
	for i < len(baseDir) && i < len(targetSegs)-1 && baseDir[i] == targetSegs[i] {
		i++
	}
	var rel []string
	for j := i; j < len(baseDir); j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, targetSegs[i:]...)
	return strings.Join(rel, "/")
}

// marshalNewGlossary builds a fresh glossary part containing the given building
// blocks wrapped in the w:docParts container Word uses.
func marshalNewGlossary(defs []BuildingBlockDef) []byte {
	b := xmlb.NewBuilder()
	b.SetCollapseEmptyElements(true)
	b.WriteHeader()
	b.StartElementLiteral("w", "glossaryDocument", nil,
		xmlb.Attr{Name: "xmlns:w", Value: xmlb.NSWordprocessingML},
		xmlb.Attr{Name: "xmlns:r", Value: xmlb.NSOfficeDocumentRels},
	)
	b.StartElementLiteral("w", "docParts", nil)
	for _, def := range defs {
		writeDocPart(b, "w", def)
	}
	b.EndElementLiteral("w", "docParts")
	b.EndElementLiteral("w", "glossaryDocument")
	_ = b.Finish()
	return b.Bytes()
}

// spliceDocParts inserts the given building blocks, serialized as w:docPart
// elements, immediately before the closing tag of the w:docParts container in
// an existing glossary part's raw bytes. Every other byte — including every
// existing docPart and its body — is preserved verbatim.
func spliceDocParts(raw []byte, defs []BuildingBlockDef) ([]byte, error) {
	offset, prefix, ok := docPartsInsertPoint(raw)
	if !ok {
		return nil, fmt.Errorf("docx: AddBuildingBlock: no <w:docParts> container found in glossary part")
	}
	b := xmlb.NewBuilder()
	b.SetCollapseEmptyElements(true)
	for _, def := range defs {
		writeDocPart(b, prefix, def)
	}
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: AddBuildingBlock: %w", err)
	}
	frag := b.Bytes()
	out := make([]byte, 0, len(raw)+len(frag))
	out = append(out, raw[:offset]...)
	out = append(out, frag...)
	out = append(out, raw[offset:]...)
	return out, nil
}

// docPartsInsertPoint locates the byte offset just before the closing tag of the
// w:docParts container and the namespace prefix used on that tag ("" for a
// default-namespace part), so a new docPart can be spliced in ahead of it.
func docPartsInsertPoint(raw []byte) (offset int, prefix string, ok bool) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var prev int64
	for {
		prev = dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			return 0, "", false
		}
		end, isEnd := tok.(xml.EndElement)
		if !isEnd || end.Name.Local != "docParts" {
			continue
		}
		p := int(prev)
		for p < len(raw) && raw[p] != '<' {
			p++
		}
		return p, tagPrefix(raw, p), true
	}
}

// tagPrefix returns the namespace prefix of the element tag beginning at raw[p]
// (raw[p] == '<'), or "" for an unprefixed tag. It reads the prefix that
// precedes the ':' in the tag name and works for both start and end tags.
func tagPrefix(raw []byte, p int) string {
	i := p + 1
	if i < len(raw) && raw[i] == '/' {
		i++
	}
	start := i
	for i < len(raw) {
		switch raw[i] {
		case ':':
			return string(raw[start:i])
		case ' ', '\t', '\r', '\n', '>', '/':
			return ""
		}
		i++
	}
	return ""
}

// writeDocPart serializes one building block as a w:docPart element under the
// given prefix, following the CT_DocPartPr child order (name, style, category,
// types, description, guid). The body is a single empty paragraph.
func writeDocPart(b *xmlb.Builder, prefix string, def BuildingBlockDef) {
	val := func(local, value string) {
		b.EmptyElementLiteral(prefix, local, xmlb.Attr{Name: prefix + ":val", Value: value})
	}
	b.StartElementLiteral(prefix, "docPart", nil)
	b.StartElementLiteral(prefix, "docPartPr", nil)
	val("name", def.Name)
	if def.Style != "" {
		val("style", def.Style)
	}
	if def.Category != "" || def.Gallery != "" {
		b.StartElementLiteral(prefix, "category", nil)
		val("name", def.Category)
		if def.Gallery != "" {
			val("gallery", def.Gallery)
		}
		b.EndElementLiteral(prefix, "category")
	}
	if len(def.Types) > 0 {
		b.StartElementLiteral(prefix, "types", nil)
		for _, t := range def.Types {
			val("type", t)
		}
		b.EndElementLiteral(prefix, "types")
	}
	if def.Description != "" {
		val("description", def.Description)
	}
	val("guid", def.GUID)
	b.EndElementLiteral(prefix, "docPartPr")
	b.StartElementLiteral(prefix, "docPartBody", nil)
	b.EmptyElementLiteral(prefix, "p")
	b.EndElementLiteral(prefix, "docPartBody")
	b.EndElementLiteral(prefix, "docPart")
}
