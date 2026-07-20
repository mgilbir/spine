package docx

import (
	"github.com/mgilbir/spine/opc"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// relTypeGlossaryDocument is the relationship type linking a document to its
// glossary (building blocks / AutoText) part. Defined locally so the opc package
// stays free of WordprocessingML-specific constants.
const relTypeGlossaryDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument"

// defaultGlossaryPart is the conventional glossary part name, used when no
// glossaryDocument relationship names another.
const defaultGlossaryPart = "/word/glossary/document.xml"

// BuildingBlock is one entry of the glossary document (a w:docPart) — a reusable
// content fragment such as an AutoText entry, a cover page, or a header/footer
// gallery item. Building blocks are read-only in this API; the glossary part is
// preserved verbatim on save.
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
// document order, or nil when the document has no glossary part. The glossary
// part is preserved verbatim on save; this API is read-only.
func (d *Document) BuildingBlocks() []*BuildingBlock {
	part, ok := d.preservedParts[d.glossaryPartName()]
	if !ok {
		return nil
	}
	var meta ctGlossaryMeta
	if err := xmlb.Unmarshal(part.Data, &meta); err != nil {
		return nil
	}
	out := make([]*BuildingBlock, 0, len(meta.DocParts.DocPart))
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
	return out
}
