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
	hdrFtrRefRe = regexp.MustCompile(`<w:(?:header|footer)Reference\b[^>]*/>`)
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
// other that reference headers or footers have those references dropped (the
// header/footer parts are not carried). Full header/footer and theme
// reconciliation is deferred.
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

	// Serialize other's main part, rewrite the colliding ids in the body
	// content, then re-parse and append the body children in order.
	data, err := marshalDocumentXML(other.document)
	if err != nil {
		return err
	}
	data = rewriteReferences(data, relRemap, styleRemap, numRemap)

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

// importNumbering copies other's session-added numbering definitions (abstract
// numbering and numbering instances) into this document with non-colliding ids,
// returning a map from other's numId to the new numId for reference rewriting.
// Numbering definitions preserved verbatim in an opened source (kept as raw
// XML) are not carried; that case is deferred.
func (d *Document) importNumbering(other *Document) (map[string]string, error) {
	numRemap := make(map[string]string)
	if other.numbering == nil {
		return numRemap, nil
	}
	if len(other.numbering.AbstractNum) == 0 && len(other.numbering.Num) == 0 {
		return numRemap, nil
	}
	if d.numbering == nil {
		d.numbering = &oxml.CT_Numbering{}
	}

	absRemap := make(map[string]string)
	for _, srcAbs := range other.numbering.AbstractNum {
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
	for _, srcNum := range other.numbering.Num {
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

// rewriteReferences applies the relationship-id, style-id, and numbering-id
// remaps to the serialized document bytes and drops header/footer references
// (whose parts are not carried across).
func rewriteReferences(data []byte, relRemap, styleRemap, numRemap map[string]string) []byte {
	data = replaceAttr(data, relIDRefRe, relRemap)
	data = replaceAttr(data, pStyleRefRe, styleRemap)
	data = replaceAttr(data, rStyleRefRe, styleRemap)
	data = replaceAttr(data, tblStyleRe, styleRemap)
	data = replaceAttr(data, numIDRefRe, numRemap)
	data = hdrFtrRefRe.ReplaceAll(data, nil)
	return data
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
