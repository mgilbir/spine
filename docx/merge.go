package docx

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
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
	// Note and comment references carry a w:id that indexes the footnotes,
	// endnotes, or comments part — remapped when the imported definitions are
	// given fresh, non-colliding ids. Each pattern is scoped to its own element
	// so an unrelated w:id (a tracked-change id, a bookmark id) is never touched.
	ftnRefRe     = regexp.MustCompile(`(<w:footnoteReference\b[^>]*\bw:id=")([^"]*)(")`)
	ednRefRe     = regexp.MustCompile(`(<w:endnoteReference\b[^>]*\bw:id=")([^"]*)(")`)
	commentRefRe = regexp.MustCompile(`(<w:comment(?:Reference|RangeStart|RangeEnd)\b[^>]*\bw:id=")([^"]*)(")`)
)

// Append appends the body content (paragraphs, tables, and block-level
// structured document tags) of other to this document. Images and other media
// referenced by the copied content are brought over as new package parts with
// remapped relationship ids; style and numbering definitions are copied, and
// their ids are remapped when they collide with differing definitions already
// in this document, with every reference in the copied content rewritten to
// match.
//
// Footnotes, endnotes, and comments referenced by the copied content are merged
// too: the source definitions are imported with fresh ids disjoint from this
// document's, and the copied reference marks are rewritten to point at them, so
// an appended footnote keeps its own text rather than aliasing onto a
// destination note that happens to share its id. Comment threading metadata
// (commentsExtended, people) is not merged.
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
	if other.doc() == nil || other.doc().Body == nil {
		return nil
	}

	d.markEdited()
	// Serialize other's main part up front: the reference-id remaps are driven by
	// the relationship ids the copied body actually references, and the serialized
	// bytes are the same ones rewritten and re-parsed below.
	data, err := marshalDocumentXML(other.doc())
	if err != nil {
		return err
	}
	refd := referencedRelIDs(data)

	relRemap, err := d.importRelationships(other, refd)
	if err != nil {
		return err
	}
	styleRemap, addedStyles := d.importStyles(other)
	numRemap, addedAbstractNums, err := d.importNumbering(other)
	if err != nil {
		return err
	}
	// The style and numbering remaps are now both known, so rewrite the
	// cross-references buried inside the copied definitions themselves (a style's
	// numId, an abstractNum level's pStyle/numStyleLink) — the serialized-body
	// rewrite below never reaches them.
	remapCopiedCrossRefs(addedStyles, addedAbstractNums, styleRemap, numRemap)
	hdrFtrRemap, err := d.importHeadersFooters(other)
	if err != nil {
		return err
	}
	noteRemap, err := d.importNotesComments(other)
	if err != nil {
		return err
	}

	// Rewrite the colliding ids in the body content, then re-parse and append the
	// body children in order.
	data = rewriteReferences(data, relRemap, styleRemap, numRemap, hdrFtrRemap, noteRemap)
	// Any relationship reference the import could not carry across — a source
	// image whose bytes would not resolve, a reference already dangling in the
	// source — keeps the source's rId, which in the destination silently
	// resolves to whatever unrelated relationship happens to hold that id.
	// Aliasing is worse than dangling, so strip those attributes (C489).
	data = stripUnresolvableRelRefs(data, carriedRelIDs(relRemap, hdrFtrRemap))

	var rewritten oxml.CT_Document
	// UnmarshalWithSource, not xml.Unmarshal: the capture kit only preserves
	// unmodeled children when the decoder has its source bytes registered, and
	// this re-parse is what the appended body is built from (C370).
	if err := xmlb.UnmarshalWithSource(data, &rewritten); err != nil {
		return err
	}
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	// Bookmark ids and names are per-document, so the imported markers are
	// renumbered (and colliding names renamed) before the content joins this
	// body — two documents both carrying Word's _GoBack, very often id 0,
	// otherwise produce two bookmarkStart/bookmarkEnd pairs sharing an id
	// (mispaired) and two bookmarks sharing a name (an ambiguous internal
	// hyperlink target) (C503).
	d.remapImportedBookmarks(rewritten.Body)
	d.doc().Body.AppendAllFrom(rewritten.Body)
	return nil
}

// remapImportedBookmarks renumbers every bookmark in the body about to be
// appended so its ids sit above this document's, and renames the ones whose
// name is already taken here.
func (d *Document) remapImportedBookmarks(src *oxml.CT_Body) {
	if src == nil {
		return
	}
	taken := make(map[string]bool)
	for _, b := range d.Bookmarks() {
		taken[b.Name()] = true
	}
	next, _ := strconv.Atoi(d.nextBookmarkID())
	ids := make(map[string]string)
	names := make(map[string]string)
	src.RemapBookmarks(oxml.BookmarkRemap{
		ID: func(id string) string {
			if v, ok := ids[id]; ok {
				return v
			}
			v := strconv.Itoa(next)
			next++
			ids[id] = v
			return v
		},
		Name: func(name string) string {
			if v, ok := names[name]; ok {
				return v
			}
			v := name
			if taken[name] {
				v = uniqueBookmarkName(taken, name)
			}
			taken[v] = true
			names[name] = v
			return v
		},
	})
}

// uniqueBookmarkName derives a free bookmark name from base by appending an
// incrementing suffix, mirroring uniqueStyleID for style ids. Word caps
// bookmark names at 40 characters, so the base is trimmed to leave room.
func uniqueBookmarkName(taken map[string]bool, base string) string {
	const maxBookmarkName = 40
	for i := 1; ; i++ {
		suffix := "_" + strconv.Itoa(i)
		trimmed := base
		if len(trimmed)+len(suffix) > maxBookmarkName {
			trimmed = trimmed[:maxBookmarkName-len(suffix)]
		}
		candidate := trimmed + suffix
		if !taken[candidate] {
			return candidate
		}
	}
}

// importRelationships copies the relationships of other's main part that the
// copied body references into this document, returning a map from other's
// relationship id to the newly assigned id. External links and images are
// re-registered through the media path; every other internal relationship the
// body references (charts, OLE objects, ActiveX, ...) is brought over as a fresh
// package part — with its own transitive relationships — so no reference in the
// merged body is left dangling or silently aliased onto an unrelated part.
// Header and footer relationships are handled separately by importHeadersFooters
// and skipped here. refd is the set of relationship ids the copied body actually
// references; a main-part relationship the body does not reference (styles,
// numbering, footnotes, ...) is not a body reference and is carried by its own
// dedicated importer, so it is skipped.
func (d *Document) importRelationships(other *Document, refd map[string]bool) (map[string]string, error) {
	remap := make(map[string]string)
	visited := make(map[string]string)
	for _, rel := range other.relationships[other.mainPart()] {
		if rel == nil {
			continue
		}
		switch {
		case rel.TargetMode == opc.TargetModeExternal:
			// External links (hyperlinks, external images) the copied body
			// actually references: re-register with a fresh id pointing at the
			// same external target. Externals used to be imported
			// unconditionally, so a source main-part external the body never
			// mentions — an attachedTemplate, an external subDoc — was copied
			// into the destination as a stray relationship (C489).
			if !refd[rel.ID] {
				continue
			}
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
		case rel.Type == opc.RelTypeHeader || rel.Type == opc.RelTypeFooter:
			// Carried by importHeadersFooters, which remaps these ids itself.
			continue
		case refd[rel.ID]:
			// An internal part the body references (chart, OLE object, ...):
			// import the part tree and remap the reference.
			newID, err := d.importInternalPart(other, rel, visited)
			if err != nil {
				return nil, err
			}
			if newID != "" {
				remap[rel.ID] = newID
			}
		}
	}
	return remap, nil
}

// referencedRelIDs returns the set of relationship ids (rId...) referenced by
// the r:*-namespaced attributes of the serialized body — the ids that must
// resolve to a relationship after the merge.
func referencedRelIDs(data []byte) map[string]bool {
	set := make(map[string]bool)
	for _, m := range relIDRefRe.FindAllSubmatch(data, -1) {
		set[string(m[2])] = true
	}
	return set
}

// importInternalPart copies the source part targeted by an internal main-part
// relationship (a chart, OLE object, ActiveX control, ...) into this document
// under a fresh, collision-free part name — transitively importing the part's
// own relationships — and registers a fresh main-part relationship pointing at
// it, returning the new relationship id. It returns "" when the target part's
// bytes cannot be resolved.
func (d *Document) importInternalPart(other *Document, rel *opc.Relationship, visited map[string]string) (string, error) {
	srcPart := opc.ResolvePartName(other.mainPart(), rel.Target)
	newName, err := d.importPartTree(other, srcPart, visited)
	if err != nil {
		return "", err
	}
	if newName == "" {
		return "", nil
	}
	newID := fmt.Sprintf("rId%d", d.nextRelID())
	d.addPartRelationship(d.mainPart(), &opc.Relationship{
		ID:     newID,
		Type:   rel.Type,
		Target: relativePartTarget(d.mainPart(), newName),
	})
	return newID, nil
}

// importPartTree copies srcPart (of other) into this document under a fresh part
// name, recursively importing every part it relates to and rewriting its own
// r:id references to the freshly assigned ids. visited memoizes source part →
// new part name so a part shared by several references is imported once and any
// relationship cycle terminates. It returns the new part name, or "" when the
// source bytes cannot be resolved.
func (d *Document) importPartTree(other *Document, srcPart string, visited map[string]string) (string, error) {
	if newName, ok := visited[srcPart]; ok {
		return newName, nil
	}
	data, contentType, ok := other.rawPartBytes(srcPart)
	if !ok {
		return "", nil
	}
	newName := d.freshImportName(srcPart)
	ip := &importedPart{partName: newName, contentType: contentType}
	// Reserve the name (so nested imports pick a different one) and record it
	// before recursing so a cycle back to srcPart resolves to this same part.
	d.importedParts = append(d.importedParts, ip)
	visited[srcPart] = newName

	subRemap := make(map[string]string)
	for _, r := range other.relationships[srcPart] {
		if r == nil {
			continue
		}
		if r.TargetMode == opc.TargetModeExternal {
			newID := fmt.Sprintf("rId%d", d.nextRelID())
			d.addPartRelationship(newName, &opc.Relationship{
				ID:         newID,
				Type:       r.Type,
				Target:     r.Target,
				TargetMode: opc.TargetModeExternal,
			})
			subRemap[r.ID] = newID
			continue
		}
		childSrc := opc.ResolvePartName(srcPart, r.Target)
		childNew, err := d.importPartTree(other, childSrc, visited)
		if err != nil {
			return "", err
		}
		if childNew == "" {
			continue
		}
		newID := fmt.Sprintf("rId%d", d.nextRelID())
		d.addPartRelationship(newName, &opc.Relationship{
			ID:     newID,
			Type:   r.Type,
			Target: relativePartTarget(newName, childNew),
		})
		subRemap[r.ID] = newID
	}
	// Rewrite the r:id references inside XML parts only; binary parts (embedded
	// workbooks, images, OLE binaries) must pass through byte-for-byte.
	if strings.HasSuffix(strings.ToLower(srcPart), ".xml") {
		data = replaceAttr(data, relIDRefRe, subRemap)
	}
	ip.data = data
	return newName, nil
}

// rawPartBytes resolves the bytes and content type of one of this document's own
// package parts by name, covering parts preserved from an opened package, parts
// captured raw, images and charts added through the mutation API, and (last) the
// source reader. Used by Append to read the parts of the document being merged.
func (d *Document) rawPartBytes(name string) ([]byte, string, bool) {
	if p, ok := d.preservedParts[name]; ok {
		return p.Data, p.ContentType, true
	}
	if p, ok := d.otherParts[name]; ok {
		return p.Data, p.ContentType, true
	}
	for _, ip := range d.imageParts {
		if ip.partName == name {
			return ip.data, ip.contentType, true
		}
	}
	for _, cp := range d.chartParts {
		if cp.partName == name {
			return cp.data, opc.ContentTypeChart, true
		}
		if cp.embedName == name {
			return cp.embedData, opc.ContentTypeSpreadsheetPackage, true
		}
	}
	if d.reader != nil {
		if f := d.reader.GetFile(name); f != nil {
			if data, err := f.ReadAll(); err == nil {
				return data, f.ContentType, true
			}
		}
	}
	return nil, "", false
}

// freshImportName returns a part name based on srcPart that collides with no
// part already present in this document, appending a numeric suffix before the
// extension when the preferred name is taken.
func (d *Document) freshImportName(srcPart string) string {
	if !d.partNameTaken(srcPart) {
		return srcPart
	}
	ext := path.Ext(srcPart)
	base := srcPart[:len(srcPart)-len(ext)]
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s_%d%s", base, i, ext)
		if !d.partNameTaken(cand) {
			return cand
		}
	}
}

// partNameTaken reports whether a part name is already occupied by any part this
// document will write, comparing case-insensitively (OPC part names are
// case-insensitive).
func (d *Document) partNameTaken(name string) bool {
	lname := strings.ToLower(name)
	if _, ok := d.preservedParts[name]; ok {
		return true
	}
	if _, ok := d.otherParts[name]; ok {
		return true
	}
	if _, ok := d.headers[name]; ok {
		return true
	}
	if _, ok := d.footers[name]; ok {
		return true
	}
	for _, ip := range d.importedParts {
		if strings.ToLower(ip.partName) == lname {
			return true
		}
	}
	for _, ip := range d.imageParts {
		if strings.ToLower(ip.partName) == lname {
			return true
		}
	}
	for _, cp := range d.chartParts {
		if strings.ToLower(cp.partName) == lname || strings.ToLower(cp.embedName) == lname {
			return true
		}
	}
	if d.reader != nil && d.reader.GetFile(name) != nil {
		return true
	}
	return false
}

// writeImportedParts writes the parts carried over by Append (charts, OLE
// objects, ...) together with their own relationships. Called from
// writeAddedParts on both save lifecycles.
func (d *Document) writeImportedParts(writer *opc.Writer) error {
	for _, ip := range d.importedParts {
		if err := writer.WritePart(ip.partName, ip.contentType, ip.data); err != nil {
			return err
		}
		if rels := d.relationships[ip.partName]; len(rels) > 0 {
			if err := writer.WritePartRelationships(ip.partName, rels); err != nil {
				return err
			}
		}
	}
	return nil
}

// importedPart is a package part copied verbatim from a source document during
// Append, together with its resolved content type. Its own relationships live in
// d.relationships keyed by partName.
type importedPart struct {
	partName    string
	contentType string
	data        []byte
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
func (d *Document) importStyles(other *Document) (map[string]string, []*oxml.CT_Style) {
	remap := make(map[string]string)
	if other.styles == nil || len(other.styles.Style) == 0 {
		return remap, nil
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
		d.markStylesModified()
	}
	return remap, toAdd
}

// importNumbering copies other's numbering definitions (abstract numbering and
// numbering instances) into this document with non-colliding ids, returning a
// map from other's numId to the new numId for reference rewriting. Both
// session-added definitions and definitions preserved verbatim in an opened
// source (kept as raw XML in CT_Numbering.Raw) are carried: the source part is
// serialized and re-parsed fully typed so the raw originals surface as
// CT_AbstractNum/CT_Num that go through the same remapping path.
func (d *Document) importNumbering(other *Document) (map[string]string, []*oxml.CT_AbstractNum, error) {
	numRemap := make(map[string]string)
	if other.numbering == nil {
		return numRemap, nil, nil
	}
	absDefs, numDefs, err := typedNumberingDefs(other.numbering)
	if err != nil {
		return nil, nil, err
	}
	if len(absDefs) == 0 && len(numDefs) == 0 {
		return numRemap, nil, nil
	}
	if d.numbering == nil {
		d.numbering = &oxml.CT_Numbering{}
	}

	var addedAbs []*oxml.CT_AbstractNum
	absRemap := make(map[string]string)
	for _, srcAbs := range absDefs {
		clone := &oxml.CT_AbstractNum{}
		if err := deepCopyXML(clone, srcAbs); err != nil {
			return nil, nil, err
		}
		newID := strconv.Itoa(d.nextAbstractNumID())
		absRemap[clone.AbstractNumId] = newID
		clone.AbstractNumId = newID
		d.numbering.AbstractNum = append(d.numbering.AbstractNum, clone)
		d.numbering.ParsedAbstractNumIDs = append(d.numbering.ParsedAbstractNumIDs, newID)
		addedAbs = append(addedAbs, clone)
	}
	for _, srcNum := range numDefs {
		clone := &oxml.CT_Num{}
		if err := deepCopyXML(clone, srcNum); err != nil {
			return nil, nil, err
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
	d.markNumberingModified()
	return numRemap, addedAbs, nil
}

// noteCommentRemap holds the footnote, endnote, and comment id remaps produced
// by importNotesComments, keyed by the source w:id and giving the fresh id the
// definition was imported under.
type noteCommentRemap struct {
	footnote map[string]string
	endnote  map[string]string
	comment  map[string]string
}

// importNotesComments merges the source's footnotes, endnotes, and comments into
// this document. Each imported definition is given a fresh id disjoint from this
// document's existing ids, and the returned remaps let the copied body's
// reference marks be rewritten to point at their own imported definitions
// instead of silently resolving to this document's unrelated notes/comments.
func (d *Document) importNotesComments(other *Document) (noteCommentRemap, error) {
	var out noteCommentRemap
	var err error
	if out.footnote, err = d.importFootnotes(other); err != nil {
		return out, err
	}
	if out.endnote, err = d.importEndnotes(other); err != nil {
		return out, err
	}
	if out.comment, err = d.importComments(other); err != nil {
		return out, err
	}
	return out, nil
}

// importFootnotes copies the source's user footnotes into this document with
// fresh ids (past this document's highest), returning a map from the source id
// to the assigned id. The mandatory separator notes are shared infrastructure
// and are not duplicated. Serialize-then-reparse produces independent copies:
// note bodies use xml:"-" fields that deepCopyXML cannot clone.
func (d *Document) importFootnotes(other *Document) (map[string]string, error) {
	remap := make(map[string]string)
	if other.footnotes == nil || len(other.footnotes.Footnote) == 0 {
		return remap, nil
	}
	data, err := marshalFootnotesXML(other.footnotes)
	if err != nil {
		return nil, err
	}
	var parsed oxml.CT_Footnotes
	if err := xmlb.UnmarshalWithSource(data, &parsed); err != nil {
		return nil, err
	}
	d.ensureFootnotes()
	next := d.footnotes.MaxID() + 1
	if next < 1 {
		next = 1
	}
	for _, n := range parsed.Footnote {
		if n == nil || isSeparatorNote(n) {
			continue
		}
		newID := strconv.Itoa(next)
		next++
		remap[n.Id] = newID
		n.Id = newID
		d.footnotes.Footnote = append(d.footnotes.Footnote, n)
	}
	if len(remap) > 0 {
		d.markFootnotesModified()
	}
	return remap, nil
}

// importEndnotes is the endnote counterpart of importFootnotes.
func (d *Document) importEndnotes(other *Document) (map[string]string, error) {
	remap := make(map[string]string)
	if other.endnotes == nil || len(other.endnotes.Endnote) == 0 {
		return remap, nil
	}
	data, err := marshalEndnotesXML(other.endnotes)
	if err != nil {
		return nil, err
	}
	var parsed oxml.CT_Endnotes
	if err := xmlb.UnmarshalWithSource(data, &parsed); err != nil {
		return nil, err
	}
	d.ensureEndnotes()
	next := d.endnotes.MaxID() + 1
	if next < 1 {
		next = 1
	}
	for _, n := range parsed.Endnote {
		if n == nil || isSeparatorNote(n) {
			continue
		}
		newID := strconv.Itoa(next)
		next++
		remap[n.Id] = newID
		n.Id = newID
		d.endnotes.Endnote = append(d.endnotes.Endnote, n)
	}
	if len(remap) > 0 {
		d.markEndnotesModified()
	}
	return remap, nil
}

// importComments copies the source's comment definitions into this document with
// fresh ids, returning a map from the source id to the assigned id. The comment
// threading metadata (commentsExtended, people) is not merged: it threads on
// w14:paraId, which is preserved unchanged, so the definitions and their body
// references stay consistent while the merge remains bounded.
func (d *Document) importComments(other *Document) (map[string]string, error) {
	remap := make(map[string]string)
	if other.comments == nil || len(other.comments.Comment) == 0 {
		return remap, nil
	}
	data, err := marshalCommentsXML(other.comments)
	if err != nil {
		return nil, err
	}
	var parsed oxml.CT_Comments
	if err := xmlb.UnmarshalWithSource(data, &parsed); err != nil {
		return nil, err
	}
	if d.comments == nil {
		d.comments = &oxml.CT_Comments{}
	}
	next := d.comments.MaxID() + 1
	if next < 1 {
		next = 1
	}
	// The imported comments' w14:paraId values are this document's keys once
	// they land here: commentsExtended is keyed on them, and the source's
	// threading metadata is deliberately not merged. A source paraId that
	// happens to match one of this document's commentsExtended keys would make
	// the foreign comment read back as resolved or threaded under a local
	// comment, so colliding ids are reassigned (C503).
	used := d.usedParaIDs()
	for _, cm := range parsed.Comment {
		if cm == nil {
			continue
		}
		newID := strconv.Itoa(next)
		next++
		remap[cm.Id] = newID
		cm.Id = newID
		for _, p := range cm.P {
			if p == nil || p.ParaId == "" {
				continue
			}
			for used[strings.ToUpper(p.ParaId)] {
				// nextParaID's own scan does not yet see the comments still in
				// flight in this loop, so re-check against the running set.
				p.ParaId = d.nextParaID()
			}
			used[strings.ToUpper(p.ParaId)] = true
		}
		d.comments.Comment = append(d.comments.Comment, cm)
	}
	if len(remap) > 0 {
		d.markCommentsModified()
	}
	return remap, nil
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
	if err := xmlb.Unmarshal(data, &typed); err != nil {
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
	used := usedHdrFtrRIDs(other.doc().Body)
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
	if err := xmlb.UnmarshalWithSource(data, hf); err != nil {
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
func rewriteReferences(data []byte, relRemap, styleRemap, numRemap, hdrFtrRemap map[string]string, noteRemap noteCommentRemap) []byte {
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
	data = replaceAttr(data, ftnRefRe, noteRemap.footnote)
	data = replaceAttr(data, ednRefRe, noteRemap.endnote)
	data = replaceAttr(data, commentRefRe, noteRemap.comment)
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

// carriedRelIDs returns the freshly assigned destination ids of every
// relationship the import carried across — the only relationship ids the
// rewritten body may legitimately still hold. An id left over from the source
// is not one of them: it is either a reference whose target could not be
// imported or one that already dangled in the source.
func carriedRelIDs(remaps ...map[string]string) map[string]bool {
	set := make(map[string]bool)
	for _, remap := range remaps {
		for _, newID := range remap {
			set[newID] = true
		}
	}
	return set
}

// stripUnresolvableRelRefs removes the r:*-namespaced relationship-id
// attributes of the merged body whose id is not among valid. Removing just the
// attribute — rather than the element that carries it — keeps the content: an
// a:blip with no r:embed and a w:hyperlink with no r:id are both schema-valid
// and render as an empty picture frame and as plain text respectively, whereas
// leaving the id in place would bind the copied content to an unrelated part of
// the destination.
func stripUnresolvableRelRefs(data []byte, valid map[string]bool) []byte {
	return relIDRefRe.ReplaceAllFunc(data, func(m []byte) []byte {
		sub := relIDRefRe.FindSubmatch(m)
		if sub == nil || valid[string(sub[2])] {
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
	//xmlguard:lenient deep copy by marshal-then-unmarshal of this library's own output, not a part read
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

// remapCopiedCrossRefs rewrites the style/numbering cross-references that live
// inside the copied style and numbering definitions themselves, which the
// serialized-body rewrite never sees: a paragraph style's w:pPr/w:numPr/w:numId
// (through numRemap) and an abstractNum level's w:pStyle plus the abstractNum's
// w:numStyleLink/w:styleLink (through styleRemap). Without this a copied style's
// numId keeps pointing at the source's numbering — now the destination's
// unrelated list after that numId was remapped.
func remapCopiedCrossRefs(styles []*oxml.CT_Style, absNums []*oxml.CT_AbstractNum, styleRemap, numRemap map[string]string) {
	for _, s := range styles {
		if s == nil || s.PPr == nil || s.PPr.NumPr == nil {
			continue
		}
		remapNumIDVal(s.PPr.NumPr.NumId, numRemap)
	}
	for _, a := range absNums {
		if a == nil {
			continue
		}
		remapStyleRef(&a.NumStyleLink, styleRemap)
		remapStyleRef(&a.StyleLink, styleRemap)
		for _, lvl := range a.Lvl {
			if lvl == nil {
				continue
			}
			remapStyleRef(&lvl.PStyle, styleRemap)
			if lvl.PPr != nil && lvl.PPr.NumPr != nil {
				remapNumIDVal(lvl.PPr.NumPr.NumId, numRemap)
			}
		}
	}
}

// remapNumIDVal rewrites a numId value through numRemap (whose keys and values
// are decimal strings), leaving it untouched when the id is absent or nil.
func remapNumIDVal(numID *oxml.CT_DecimalNumber, numRemap map[string]string) {
	if numID == nil {
		return
	}
	if mapped, ok := numRemap[strconv.Itoa(numID.Val)]; ok {
		if v, err := strconv.Atoi(mapped); err == nil {
			numID.Val = v
		}
	}
}
