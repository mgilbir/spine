package docx

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// ErrNilDocument is returned when Append is given a nil source document.
var ErrNilDocument = errors.New("docx: source document is nil")

// Reference-rewriting patterns applied to the merged body content. Relationship
// ids (r:embed, r:id, r:link, ...), style references, and numbering references
// all carry ids that may collide between the two documents and are remapped.
var (
	relIDRefRe  = regexp.MustCompile(`(\br:[A-Za-z]+=")(rId\d+)(")`)
	pStyleRefRe = regexp.MustCompile(`(<w:pStyle\b[^>]*\bw:val=")([^"]*)(")`)
	rStyleRefRe = regexp.MustCompile(`(<w:rStyle\b[^>]*\bw:val=")([^"]*)(")`)
	tblStyleRe  = regexp.MustCompile(`(<w:tblStyle\b[^>]*\bw:val=")([^"]*)(")`)
	numIDRefRe  = regexp.MustCompile(`(<w:numId\b[^>]*\bw:val=")([^"]*)(")`)
	// hdrFtrRefRe matches a whole header/footer reference element in either the
	// self-closing (<w:headerReference .../>) or the paired-empty form
	// (<w:headerReference ...></w:headerReference>) a producer may emit.
	hdrFtrRefRe = regexp.MustCompile(`<w:(?:header|footer)Reference\b[^>]*(?:/>|>\s*</w:(?:header|footer)Reference>)`)
	// ridAttrRe extracts the r:id value from a reference element.
	ridAttrRe = regexp.MustCompile(`\br:id="([^"]*)"`)
)

// Append appends the body content (paragraphs, tables, and block-level
// structured document tags) of other to this document. Images and other media
// referenced by the copied content are brought over as new package parts with
// remapped relationship ids; style and numbering definitions are copied, and
// their ids are remapped when they collide with differing definitions already
// in this document, with every reference in the copied content rewritten to
// match.
//
// The final (body-level) section properties of other are not copied, so the
// appended content joins this document's last section. Section breaks inside
// other that survive the append (paragraph-level section properties) keep their
// header/footer references: the referenced header and footer parts — and any
// images they embed — are carried over as new package parts with remapped
// relationship ids. Header/footer references belonging to other's dropped final
// section are not carried. Theme reconciliation is deferred.
func (d *Document) Append(other *Document) error {
	if other == nil {
		return ErrNilDocument
	}
	if other == d {
		return errors.New("docx: cannot append a document to itself")
	}
	if other.document == nil || other.document.Body == nil {
		return nil
	}

	relRemap, err := d.importRelationships(other)
	if err != nil {
		return err
	}
	styleRemap := d.importStyles(other)
	numRemap, err := d.importNumbering(other)
	if err != nil {
		return err
	}
	hdrFtrRemap, err := d.importHeadersFooters(other)
	if err != nil {
		return err
	}

	// Serialize other's main part, rewrite the colliding ids in the body
	// content, then re-parse and append the body children in order.
	data, err := marshalDocumentXML(other.document)
	if err != nil {
		return err
	}
	data = rewriteReferences(data, relRemap, styleRemap, numRemap, hdrFtrRemap)

	var rewritten oxml.CT_Document
	if err := xml.Unmarshal(data, &rewritten); err != nil {
		return err
	}
	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	d.document.Body.AppendAllFrom(rewritten.Body)
	return nil
}

// importRelationships copies the media (and external link) relationships of
// other's main part into this document, returning a map from other's
// relationship id to the newly assigned id.
func (d *Document) importRelationships(other *Document) (map[string]string, error) {
	remap := make(map[string]string)
	for _, rel := range other.relationships[other.mainPart()] {
		if rel == nil {
			continue
		}
		switch {
		case rel.TargetMode == opc.TargetModeExternal:
			// External links (hyperlinks, external images): re-register with a
			// fresh id pointing at the same external target.
			newID := fmt.Sprintf("rId%d", d.nextRelID())
			d.addPartRelationship(d.mainPart(), &opc.Relationship{
				ID:         newID,
				Type:       rel.Type,
				Target:     rel.Target,
				TargetMode: opc.TargetModeExternal,
			})
			remap[rel.ID] = newID
		case rel.Type == opc.RelTypeImage:
			data, contentType, ok := other.imageBytes(rel)
			if !ok {
				continue
			}
			ext := path.Ext(rel.Target)
			newID, _ := d.registerImagePart(d.mainPart(), data, contentType, ext)
			remap[rel.ID] = newID
		}
	}
	return remap, nil
}

// imageBytes resolves the bytes and content type for an image relationship of
// other, looking first at images added this session and then at preserved
// package parts.
func (d *Document) imageBytes(rel *opc.Relationship) ([]byte, string, bool) {
	partName := opc.ResolvePartName(d.mainPart(), rel.Target)
	for _, ip := range d.imageParts {
		if ip.partName == partName {
			return ip.data, ip.contentType, true
		}
	}
	if part, ok := d.preservedParts[partName]; ok {
		return part.Data, part.ContentType, true
	}
	return nil, "", false
}

// importStyles copies other's style definitions into this document. A style
// whose id is unused (or already identical) is copied verbatim; a style whose
// id collides with a different definition is given a fresh id, recorded in the
// returned remap so references in the copied content can be rewritten.
func (d *Document) importStyles(other *Document) map[string]string {
	remap := make(map[string]string)
	if other.styles == nil || len(other.styles.Style) == 0 {
		return remap
	}
	d.ensureStyles()

	existing := make(map[string]*oxml.CT_Style, len(d.styles.Style))
	for _, s := range d.styles.Style {
		existing[s.StyleId] = s
	}
	used := make(map[string]bool, len(d.styles.Style))
	for id := range existing {
		used[id] = true
	}

	// First pass: decide the destination id for each source style.
	var toAdd []*oxml.CT_Style
	for _, srcStyle := range other.styles.Style {
		clone := &oxml.CT_Style{}
		if err := deepCopyXML(clone, srcStyle); err != nil {
			continue
		}
		if cur, ok := existing[clone.StyleId]; ok {
			if stylesEquivalent(cur, clone) {
				continue // identical definition already present
			}
			newID := uniqueStyleID(used, clone.StyleId)
			used[newID] = true
			remap[clone.StyleId] = newID
			clone.StyleId = newID
		} else {
			used[clone.StyleId] = true
		}
		toAdd = append(toAdd, clone)
	}

	// Second pass: rewrite intra-style references (basedOn/next/link) to the
	// remapped ids, then append.
	for _, s := range toAdd {
		remapStyleRef(&s.BasedOn, remap)
		remapStyleRef(&s.Next, remap)
		remapStyleRef(&s.Link, remap)
		d.styles.Style = append(d.styles.Style, s)
	}
	if len(toAdd) > 0 {
		d.stylesModified = true
	}
	return remap
}

// importNumbering copies other's numbering definitions (abstract numbering and
// numbering instances) into this document with non-colliding ids, returning a
// map from other's numId to the new numId for reference rewriting. Both
// session-added definitions and definitions preserved verbatim in an opened
// source (kept as raw XML in CT_Numbering.Raw) are carried: the source part is
// serialized and re-parsed fully typed so the raw originals surface as
// CT_AbstractNum/CT_Num that go through the same remapping path.
func (d *Document) importNumbering(other *Document) (map[string]string, error) {
	numRemap := make(map[string]string)
	if other.numbering == nil {
		return numRemap, nil
	}
	absDefs, numDefs, err := typedNumberingDefs(other.numbering)
	if err != nil {
		return nil, err
	}
	if len(absDefs) == 0 && len(numDefs) == 0 {
		return numRemap, nil
	}
	if d.numbering == nil {
		d.numbering = &oxml.CT_Numbering{}
	}

	absRemap := make(map[string]string)
	for _, srcAbs := range absDefs {
		clone := &oxml.CT_AbstractNum{}
		if err := deepCopyXML(clone, srcAbs); err != nil {
			return nil, err
		}
		newID := strconv.Itoa(d.nextAbstractNumID())
		absRemap[clone.AbstractNumId] = newID
		clone.AbstractNumId = newID
		d.numbering.AbstractNum = append(d.numbering.AbstractNum, clone)
		d.numbering.ParsedAbstractNumIDs = append(d.numbering.ParsedAbstractNumIDs, newID)
	}
	for _, srcNum := range numDefs {
		clone := &oxml.CT_Num{}
		if err := deepCopyXML(clone, srcNum); err != nil {
			return nil, err
		}
		newID := strconv.Itoa(d.nextNumID())
		numRemap[clone.NumId] = newID
		clone.NumId = newID
		if clone.AbstractNumId != nil {
			old := strconv.Itoa(clone.AbstractNumId.Val)
			if mapped, ok := absRemap[old]; ok {
				if v, err := strconv.Atoi(mapped); err == nil {
					clone.AbstractNumId.Val = v
				}
			}
		}
		d.numbering.Num = append(d.numbering.Num, clone)
		d.numbering.ParsedNumIDs = append(d.numbering.ParsedNumIDs, newID)
	}
	d.numberingModified = true
	return numRemap, nil
}

// typedNumberingDefs returns the abstractNum and num definitions of a numbering
// part as fully typed values, covering both session-added definitions (already
// typed) and opened-source definitions preserved as raw XML. It serializes the
// part (which emits raw originals plus session-added definitions) and re-parses
// it through a typed view so both kinds surface uniformly.
func typedNumberingDefs(n *oxml.CT_Numbering) ([]*oxml.CT_AbstractNum, []*oxml.CT_Num, error) {
	// Fast path: nothing preserved raw, so the typed slices are authoritative
	// and need no re-parse.
	if len(n.Raw) == 0 {
		return n.AbstractNum, n.Num, nil
	}
	data, err := marshalNumberingXML(n)
	if err != nil {
		return nil, nil, err
	}
	var typed struct {
		XMLName     xml.Name               `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numbering"`
		AbstractNum []*oxml.CT_AbstractNum `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main abstractNum"`
		Num         []*oxml.CT_Num         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main num"`
	}
	if err := xml.Unmarshal(data, &typed); err != nil {
		return nil, nil, err
	}
	return typed.AbstractNum, typed.Num, nil
}

// importHeadersFooters carries the header and footer parts referenced by the
// section breaks of other that survive the append — the paragraph-level section
// properties (other's final body-level section is dropped, so its references are
// not carried). It returns a remap from other's header/footer relationship id to
// the fresh id assigned in this document, for rewriting the copied references.
func (d *Document) importHeadersFooters(other *Document) (map[string]string, error) {
	remap := make(map[string]string)
	used := usedHdrFtrRIDs(other.document.Body)
	if len(used) == 0 {
		return remap, nil
	}
	for _, rel := range other.relationships[other.mainPart()] {
		if rel == nil || !used[rel.ID] {
			continue
		}
		var kind string
		switch rel.Type {
		case opc.RelTypeHeader:
			kind = "header"
		case opc.RelTypeFooter:
			kind = "footer"
		default:
			continue
		}
		newID, err := d.carryHdrFtrPart(other, rel, kind)
		if err != nil {
			return nil, err
		}
		if newID != "" {
			remap[rel.ID] = newID
		}
	}
	return remap, nil
}

// usedHdrFtrRIDs collects the header/footer relationship ids referenced by the
// paragraph-level section properties in body — the sections carried by an
// append. The body-level section properties (dropped on append) are not scanned.
func usedHdrFtrRIDs(body *oxml.CT_Body) map[string]bool {
	used := make(map[string]bool)
	if body == nil {
		return used
	}
	for _, p := range body.Paragraphs() {
		if p == nil || p.PPr == nil || p.PPr.SectPr == nil {
			continue
		}
		sp := p.PPr.SectPr
		for _, ref := range sp.HeaderReference {
			if ref != nil && ref.RID != "" {
				used[ref.RID] = true
			}
		}
		for _, ref := range sp.FooterReference {
			if ref != nil && ref.RID != "" {
				used[ref.RID] = true
			}
		}
	}
	return used
}

// carryHdrFtrPart copies one header/footer part (referenced by rel) from other
// into this document under a fresh part name and relationship id, carrying any
// images it embeds with remapped ids, and returns the new relationship id. It
// returns "" when the part's bytes cannot be resolved.
func (d *Document) carryHdrFtrPart(other *Document, rel *opc.Relationship, kind string) (string, error) {
	srcPart := opc.ResolvePartName(other.mainPart(), rel.Target)
	data, ok := other.hdrFtrBytes(srcPart)
	if !ok {
		return "", nil
	}

	newPart := d.nextHdrFtrPartName(kind)
	newID := fmt.Sprintf("rId%d", d.nextRelID())

	// Carry the part's own relationships (embedded images, external links) into
	// the new part's scope, then rewrite the part bytes so its references resolve
	// to the freshly registered ids.
	subRemap, err := d.carryHdrFtrRels(other, srcPart, newPart)
	if err != nil {
		return "", err
	}
	data = replaceAttr(data, relIDRefRe, subRemap)

	hf := &oxml.CT_HdrFtr{}
	if err := xml.Unmarshal(data, hf); err != nil {
		return "", err
	}

	if kind == "header" {
		d.headers[newPart] = &headerPart{hdr: hf, contentType: opc.ContentTypeDocHeader}
		d.newHeaderParts = append(d.newHeaderParts, &hdrFtrPart{partName: newPart, relID: newID})
		d.addPartRelationship(d.mainPart(), &opc.Relationship{
			ID:     newID,
			Type:   opc.RelTypeHeader,
			Target: newPart[len("/word/"):],
		})
	} else {
		d.footers[newPart] = &footerPart{ftr: hf, contentType: opc.ContentTypeDocFooter}
		d.newFooterParts = append(d.newFooterParts, &hdrFtrPart{partName: newPart, relID: newID})
		d.addPartRelationship(d.mainPart(), &opc.Relationship{
			ID:     newID,
			Type:   opc.RelTypeFooter,
			Target: newPart[len("/word/"):],
		})
	}
	return newID, nil
}

// carryHdrFtrRels copies the image and external-link relationships of the source
// header/footer part into the new part's relationship scope, returning a remap
// from the source's relationship ids to the freshly assigned ones.
func (d *Document) carryHdrFtrRels(other *Document, srcPart, newPart string) (map[string]string, error) {
	subRemap := make(map[string]string)
	for _, rel := range other.relationships[srcPart] {
		if rel == nil {
			continue
		}
		switch {
		case rel.TargetMode == opc.TargetModeExternal:
			newID := fmt.Sprintf("rId%d", d.nextRelID())
			d.addPartRelationship(newPart, &opc.Relationship{
				ID:         newID,
				Type:       rel.Type,
				Target:     rel.Target,
				TargetMode: opc.TargetModeExternal,
			})
			subRemap[rel.ID] = newID
		case rel.Type == opc.RelTypeImage:
			data, contentType, ok := other.partImageBytes(srcPart, rel)
			if !ok {
				continue
			}
			ext := path.Ext(rel.Target)
			newID, _ := d.registerImagePart(newPart, data, contentType, ext)
			subRemap[rel.ID] = newID
		}
	}
	return subRemap, nil
}

// hdrFtrBytes resolves the serialized bytes of a header/footer part of other,
// preferring the preserved raw bytes of an opened source and falling back to
// marshaling a session-added header/footer from its parsed model.
func (d *Document) hdrFtrBytes(partName string) ([]byte, bool) {
	if part, ok := d.preservedParts[partName]; ok {
		return part.Data, true
	}
	if h, ok := d.headers[partName]; ok {
		if data, err := marshalHdrFtrXML(h.hdr, "hdr"); err == nil {
			return data, true
		}
	}
	if f, ok := d.footers[partName]; ok {
		if data, err := marshalHdrFtrXML(f.ftr, "ftr"); err == nil {
			return data, true
		}
	}
	return nil, false
}

// partImageBytes resolves the bytes and content type of an image relationship
// whose target is relative to base (e.g. a header part that embeds an image),
// looking first at images added this session and then at preserved parts.
func (d *Document) partImageBytes(base string, rel *opc.Relationship) ([]byte, string, bool) {
	partName := opc.ResolvePartName(base, rel.Target)
	for _, ip := range d.imageParts {
		if ip.partName == partName {
			return ip.data, ip.contentType, true
		}
	}
	if part, ok := d.preservedParts[partName]; ok {
		return part.Data, part.ContentType, true
	}
	return nil, "", false
}

// rewriteReferences applies the relationship-id, style-id, numbering-id, and
// header/footer-reference-id remaps to the serialized document bytes. Header and
// footer references share the main-part relationship id space with image rels,
// so both are remapped in one pass; any header/footer reference whose part was
// not carried across (other's dropped final section) is stripped so no dangling
// relationship id survives.
func rewriteReferences(data []byte, relRemap, styleRemap, numRemap, hdrFtrRemap map[string]string) []byte {
	combined := relRemap
	if len(hdrFtrRemap) > 0 {
		combined = make(map[string]string, len(relRemap)+len(hdrFtrRemap))
		for k, v := range relRemap {
			combined[k] = v
		}
		for k, v := range hdrFtrRemap {
			combined[k] = v
		}
	}
	data = replaceAttr(data, relIDRefRe, combined)
	data = replaceAttr(data, pStyleRefRe, styleRemap)
	data = replaceAttr(data, rStyleRefRe, styleRemap)
	data = replaceAttr(data, tblStyleRe, styleRemap)
	data = replaceAttr(data, numIDRefRe, numRemap)
	data = dropUncarriedHdrFtrRefs(data, hdrFtrRemap)
	return data
}

// dropUncarriedHdrFtrRefs removes header/footer reference elements whose r:id is
// not one of the freshly assigned ids of a carried part. Carried references were
// already rewritten to their new id by rewriteReferences; every other reference
// points at a part that was not imported and would dangle.
func dropUncarriedHdrFtrRefs(data []byte, hdrFtrRemap map[string]string) []byte {
	carried := make(map[string]bool, len(hdrFtrRemap))
	for _, v := range hdrFtrRemap {
		carried[v] = true
	}
	return hdrFtrRefRe.ReplaceAllFunc(data, func(m []byte) []byte {
		sub := ridAttrRe.FindSubmatch(m)
		if sub != nil && carried[string(sub[1])] {
			return m
		}
		return nil
	})
}

// replaceAttr rewrites the captured attribute value (group 2) of every match of
// re using remap, leaving values absent from remap untouched. The single-pass
// ReplaceAll keys each substitution on the original value, so chained remaps
// (a→b, b→c) never cascade.
func replaceAttr(data []byte, re *regexp.Regexp, remap map[string]string) []byte {
	if len(remap) == 0 {
		return data
	}
	return re.ReplaceAllFunc(data, func(m []byte) []byte {
		sub := re.FindSubmatch(m)
		if sub == nil {
			return m
		}
		val, ok := remap[string(sub[2])]
		if !ok {
			return m
		}
		out := make([]byte, 0, len(sub[1])+len(val)+len(sub[3]))
		out = append(out, sub[1]...)
		out = append(out, val...)
		out = append(out, sub[3]...)
		return out
	})
}

// deepCopyXML clones an xml-tagged value by marshaling and re-unmarshaling it.
func deepCopyXML(dst, src any) error {
	data, err := xml.Marshal(src)
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, dst)
}

// stylesEquivalent reports whether two style definitions are structurally equal
// ignoring the styleId (which the caller compares separately).
func stylesEquivalent(a, b *oxml.CT_Style) bool {
	ac, bc := *a, *b
	ac.StyleId, bc.StyleId = "", ""
	return reflect.DeepEqual(ac, bc)
}

// uniqueStyleID returns id if unused, otherwise id with a numeric suffix that is
// not present in used.
func uniqueStyleID(used map[string]bool, id string) string {
	if !used[id] {
		return id
	}
	for i := 1; ; i++ {
		cand := id + "-" + strconv.Itoa(i)
		if !used[cand] {
			return cand
		}
	}
}

// remapStyleRef rewrites a style reference (basedOn/next/link) through remap.
func remapStyleRef(ref **oxml.CT_String, remap map[string]string) {
	if *ref == nil {
		return
	}
	if newID, ok := remap[(*ref).Val]; ok {
		(*ref).Val = newID
	}
}
