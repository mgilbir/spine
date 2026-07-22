package pptx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// mergeCtx threads the dedup maps of a single append/extract across the slides
// it copies, so a part, master, or layout shared by several source slides is
// carried exactly once.
type mergeCtx struct {
	parts   map[string]string             // source part name -> new part name
	masters map[*SlideMaster]*SlideMaster // source master -> destination master
	layouts map[*SlideLayout]*SlideLayout // source layout -> destination layout
}

func newMergeCtx() *mergeCtx {
	return &mergeCtx{
		parts:   make(map[string]string),
		masters: make(map[*SlideMaster]*SlideMaster),
		layouts: make(map[*SlideLayout]*SlideLayout),
	}
}

// ErrNilPresentation is returned when a merge/append operation is given a nil
// source presentation.
var ErrNilPresentation = errors.New("pptx: source presentation is nil")

// AppendSlidesFrom copies every slide of other into this presentation, in
// order, after the existing slides. Each copied slide brings the parts it
// references — media (images/audio/video), charts and their embedded
// workbooks, and any other auxiliary parts — under fresh, non-colliding part
// names, with the relationship targets and ids remapped so the combined
// package has no dangling references or duplicate part names.
//
// Each appended slide's own slide layout — and the slide master and theme that
// layout depends on — is carried over with remapped part names and
// relationships, so the slide keeps its original look instead of being forced
// onto a destination layout. Masters (and layouts) that are byte-for-byte
// identical to ones already in the destination are reused rather than
// duplicated, and a master or layout shared by several appended slides is
// imported once. Each slide's notes slide is carried too, re-wired to the new
// slide (and to the destination's notes master when it has one). The source's
// notes master and handout master — each its part, the theme it references, its
// relationships, and its presentation entry — are carried when the source has
// one and the destination does not (a deck carries at most one of each, so a
// second source does not add a duplicate). The notes master is imported before
// the slides so each imported notes slide can wire to it.
func (p *Presentation) AppendSlidesFrom(other *Presentation) error {
	if other == nil {
		return ErrNilPresentation
	}
	if other == p {
		return errors.New("pptx: cannot append a presentation to itself")
	}

	ctx := newMergeCtx()
	// Carry the notes master first so the notes slides imported below re-wire
	// their notesMaster reference to it instead of dropping it.
	p.importNotesMaster(other, ctx)
	for _, src := range other.slides {
		ns, err := p.importSlide(src, ctx)
		if err != nil {
			return err
		}
		ns.materializeShapes()
	}
	p.importHandoutMaster(other, ctx)
	return nil
}

// ExtractSlides returns a new presentation containing copies of the slides at
// the given indices, in the order requested. The referenced parts (media,
// charts, embeddings) are copied with fresh part names and remapped
// relationships, so the extracted deck opens without dangling references.
//
// The extracted deck starts from a fresh default master/layout set; each
// extracted slide carries its own source layout, master, and theme (reusing the
// fresh deck's identical default parts rather than duplicating them), along with
// its notes slide. The source's notes master and handout master are carried too
// when the source has one.
func (p *Presentation) ExtractSlides(indices []int) (*Presentation, error) {
	for _, idx := range indices {
		if idx < 0 || idx >= len(p.slides) {
			return nil, ErrSlideIndex
		}
	}

	out := Create()
	ctx := newMergeCtx()
	// Carry the notes master first so the extracted notes slides wire to it.
	out.importNotesMaster(p, ctx)
	for _, idx := range indices {
		ns, err := out.importSlide(p.slides[idx], ctx)
		if err != nil {
			return nil, err
		}
		ns.materializeShapes()
	}
	out.importHandoutMaster(p, ctx)
	return out, nil
}

// importSlide copies src (which belongs to another, or the same, presentation)
// into p as a new trailing slide, carrying its referenced parts and remapping
// their relationships. ctx tracks already-copied source parts, masters, and
// layouts so shared furniture is not duplicated.
func (p *Presentation) importSlide(src *Slide, ctx *mergeCtx) (*Slide, error) {
	// Marshal the source slide first: this flushes pending shape edits, embeds
	// pending media (pictures/video/audio) into the source package's parts and
	// relationships, and resolves media timing/animations — so the slide XML and
	// its relationship set are complete before we copy them. The returned bytes
	// are the authoritative snapshot we deep-copy from.
	data, err := src.marshal()
	if err != nil {
		return nil, err
	}

	ns := p.AddSlide()

	var copyXML oxml.Slide
	if err := xml.Unmarshal(data, &copyXML); err != nil {
		return nil, err
	}
	ns.sxModel = &copyXML
	ns.sxParsed = true

	srcPres := src.presentation
	var newRels []*opc.Relationship
	for _, rel := range srcPres.relationships[src.partName] {
		if rel == nil {
			continue
		}
		switch rel.Type {
		case opc.RelTypeSlideLayout:
			// The slide's layout is imported (with its master and theme) below and
			// re-wired from the ns.layout pointer at save; the slide body never
			// references the layout by r:id, so dropping this rel is safe.
			continue
		case opc.RelTypeNotesSlide:
			srcNotes := opc.ResolvePartName(src.partName, rel.Target)
			newNotes := p.importNotesSlide(srcPres, srcNotes, ns.partName, ctx)
			if newNotes == "" {
				continue
			}
			c := *rel
			c.Target = relativeTarget(ns.partName, newNotes)
			newRels = append(newRels, &c)
			continue
		}
		if rel.TargetMode == opc.TargetModeExternal {
			c := *rel
			newRels = append(newRels, &c)
			continue
		}
		srcTarget := opc.ResolvePartName(src.partName, rel.Target)
		newTarget := p.importPart(srcPres, srcTarget, ctx.parts)
		if newTarget == "" {
			// Referenced part is furniture we do not carry (or is missing);
			// drop the relationship rather than leave it dangling.
			continue
		}
		c := *rel
		c.Target = relativeTarget(ns.partName, newTarget)
		newRels = append(newRels, &c)
	}
	if len(newRels) > 0 {
		p.relationships[ns.partName] = newRels
	}

	// Carry the slide's own layout (and the master + theme it depends on),
	// falling back to a type-matched destination layout when the source layout
	// cannot be resolved.
	if layout := p.importLayout(srcPres, src.layout, ctx); layout != nil {
		ns.layout = layout
	} else {
		ns.layout = p.matchLayout(src.layout)
	}
	return ns, nil
}

// importPart copies the part named srcName from srcPres into p (recursively
// copying the parts it references), returning the new part name. Already-copied
// parts are returned from partMap. Returns "" when the source part is not a
// carryable auxiliary part (e.g. it lives in themeData or is absent).
func (p *Presentation) importPart(srcPres *Presentation, srcName string, partMap map[string]string) string {
	if srcName == "" {
		return ""
	}
	if mapped, ok := partMap[srcName]; ok {
		return mapped
	}
	part, ok := srcPres.otherParts[srcName]
	if !ok {
		// Not an auxiliary part we carry (theme/master/layout/notes furniture).
		return ""
	}

	newName := p.freePartNameLike(srcName)
	copied := *part
	p.otherParts[newName] = &copied
	partMap[srcName] = newName

	var rels []*opc.Relationship
	for _, rel := range srcPres.relationships[srcName] {
		if rel == nil {
			continue
		}
		if rel.TargetMode == opc.TargetModeExternal {
			c := *rel
			rels = append(rels, &c)
			continue
		}
		st := opc.ResolvePartName(srcName, rel.Target)
		nt := p.importPart(srcPres, st, partMap)
		if nt == "" {
			// Uncarried furniture reference; drop rather than dangle.
			continue
		}
		c := *rel
		c.Target = relativeTarget(newName, nt)
		rels = append(rels, &c)
	}
	if len(rels) > 0 {
		p.relationships[newName] = rels
	}
	return newName
}

// freePartNameLike returns a part name in the same directory and with the same
// extension as src that is not already taken in p. The source name is reused
// verbatim when free (the common case when copying into a fresh deck).
func (p *Presentation) freePartNameLike(src string) string {
	if !p.partNameTaken(src) {
		return src
	}
	slash := strings.LastIndex(src, "/")
	dir := src[:slash+1]
	file := src[slash+1:]
	ext := path.Ext(file)
	stem := strings.TrimSuffix(file, ext)
	for i := 1; ; i++ {
		cand := dir + stem + "_" + strconv.Itoa(i) + ext
		if !p.partNameTaken(cand) {
			return cand
		}
	}
}

// importLayout resolves the destination layout for a source slide's layout,
// importing the master (and all its layouts and theme) it depends on. When the
// master is reused from the destination (deduplicated), the byte-identical
// layout under it is reused; a fresh master brings every layout with it. Returns
// nil when src is nil, its master cannot be imported, or no matching layout is
// found under a deduplicated master (the caller then falls back to matchLayout).
func (p *Presentation) importLayout(srcPres *Presentation, src *SlideLayout, ctx *mergeCtx) *SlideLayout {
	if src == nil {
		return nil
	}
	if l, ok := ctx.layouts[src]; ok {
		return l
	}
	master := p.importMaster(srcPres, src.master, ctx)
	if master == nil {
		return nil
	}
	// A fresh master import populates ctx.layouts for all its layouts, so a hit
	// here means the master was newly imported; a miss means it was reused from
	// the destination, where an identical layout is looked up (or the caller
	// falls back).
	if l, ok := ctx.layouts[src]; ok {
		return l
	}
	return findEquivalentLayout(master, src)
}

// importMaster carries the source master — its theme and every layout — into p,
// reusing a destination master that is byte-for-byte identical (same master XML
// and theme) rather than duplicating it. On a fresh import ctx.layouts is
// populated for each of the master's layouts. Returns nil when src is nil.
func (p *Presentation) importMaster(srcPres *Presentation, src *SlideMaster, ctx *mergeCtx) *SlideMaster {
	if src == nil {
		return nil
	}
	if m, ok := ctx.masters[src]; ok {
		return m
	}
	if dm := p.findEquivalentMaster(srcPres, src); dm != nil {
		ctx.masters[src] = dm
		return dm
	}

	nm := &SlideMaster{presentation: p, numericID: 0}
	if data, err := src.marshal(); err == nil {
		clone := &oxml.SlideMaster{}
		if xml.Unmarshal(data, clone) == nil {
			nm.masterXML = clone
		}
	}
	nm.partName = p.nextAvailableMasterPartName()
	p.slideMasters = append(p.slideMasters, nm)
	ctx.masters[src] = nm

	// Carry every layout of the master, preserving each layout's relationship id
	// so the master's verbatim sldLayoutIdLst (which references layouts by r:id)
	// stays consistent with the master's relationships.
	for _, srcLayout := range src.layouts {
		if srcLayout == nil {
			continue
		}
		nl := p.addImportedLayout(nm, srcLayout, srcLayout.relID)
		if nl != nil {
			ctx.layouts[srcLayout] = nl
		}
	}

	// Import the master's theme (opened source only; a created source keeps its
	// theme hardcoded at save and has none in themeData). The theme part is
	// stored under a free name and referenced by both save paths. Its rel id is
	// allocated past the layout rels so it never collides with a sldLayoutId.
	if themeName := srcPres.masterThemePartName(src); themeName != "" {
		if data, ok := srcPres.themeData[themeName]; ok {
			newTheme := p.freeThemePartName()
			p.themeData[newTheme] = bytes.Clone(data)
			nm.themePartName = newTheme
			masterRels := p.relationships[nm.partName]
			p.relationships[nm.partName] = append(masterRels, &opc.Relationship{
				ID:         fmt.Sprintf("rId%d", nextRelationshipID(masterRels)),
				Type:       opc.RelTypeTheme,
				Target:     partNameToRelTarget(newTheme, path.Dir(nm.partName)+"/"),
				TargetMode: opc.TargetModeInternal,
			})
		}
	}

	// The round-trip save path does not synthesize a presentation->master rel for
	// a newly added master (it only appends new-slide rels), so register it here.
	// The from-scratch save path rebuilds these rels and reassigns relID, so this
	// is harmless there.
	presRels := p.relationships["/ppt/presentation.xml"]
	nm.relID = fmt.Sprintf("rId%d", nextRelationshipID(presRels))
	p.relationships["/ppt/presentation.xml"] = append(presRels, &opc.Relationship{
		ID:         nm.relID,
		Type:       opc.RelTypeSlideMaster,
		Target:     partNameToRelTarget(nm.partName, "/ppt/"),
		TargetMode: opc.TargetModeInternal,
	})
	return nm
}

// addImportedLayout appends a deep copy of src as a new layout under master with
// the given relationship id, wiring the master<->layout relationships. It
// mirrors SlideMaster.AddLayout but preserves the source layout's XML and rel id
// instead of building a default layout with a freshly allocated id.
func (p *Presentation) addImportedLayout(master *SlideMaster, src *SlideLayout, relID string) *SlideLayout {
	nl := &SlideLayout{
		presentation: p,
		master:       master,
		layoutType:   src.layoutType,
		name:         src.name,
	}
	if data, err := src.marshal(); err == nil {
		clone := &oxml.SlideLayout{}
		if xml.Unmarshal(data, clone) == nil {
			nl.layoutXML = clone
		}
	}
	if nl.layoutXML == nil {
		return nil
	}
	if relID == "" {
		relID = fmt.Sprintf("rId%d", master.nextLayoutRelIDNum())
	}
	nl.relID = relID
	nl.partName = p.nextAvailableLayoutPartName()
	master.layouts = append(master.layouts, nl)
	p.slideLayouts = append(p.slideLayouts, nl)
	master.registerLayoutRelationships(nl)
	return nl
}

// findEquivalentMaster returns a destination master byte-for-byte identical to
// src — same marshaled master XML and same theme bytes — or nil when none match.
func (p *Presentation) findEquivalentMaster(srcPres *Presentation, src *SlideMaster) *SlideMaster {
	srcXML, err := src.marshal()
	if err != nil {
		return nil
	}
	srcTheme := srcPres.masterThemeBytes(src)
	for _, dm := range p.slideMasters {
		dmXML, err := dm.marshal()
		if err != nil {
			continue
		}
		if !bytes.Equal(srcXML, dmXML) {
			continue
		}
		if !bytes.Equal(srcTheme, p.masterThemeBytes(dm)) {
			continue
		}
		return dm
	}
	return nil
}

// findEquivalentLayout returns a layout under master byte-for-byte identical to
// src, or nil when none match.
func findEquivalentLayout(master *SlideMaster, src *SlideLayout) *SlideLayout {
	srcXML, err := src.marshal()
	if err != nil {
		return nil
	}
	for _, l := range master.layouts {
		lXML, err := l.marshal()
		if err != nil {
			continue
		}
		if bytes.Equal(srcXML, lXML) {
			return l
		}
	}
	return nil
}

// masterThemePartName resolves the theme part a master references, or "" when it
// has no (internal) theme relationship.
func (p *Presentation) masterThemePartName(m *SlideMaster) string {
	if m == nil || m.partName == "" {
		return ""
	}
	for _, rel := range p.relationships[m.partName] {
		if rel != nil && rel.Type == opc.RelTypeTheme && rel.TargetMode != opc.TargetModeExternal {
			return opc.ResolvePartName(m.partName, rel.Target)
		}
	}
	return ""
}

// masterThemeBytes returns the theme bytes a master references, or nil. A
// created deck keeps its theme hardcoded at save (not in themeData), so this
// returns nil for it — which makes two default masters compare as theme-equal.
func (p *Presentation) masterThemeBytes(m *SlideMaster) []byte {
	name := p.masterThemePartName(m)
	if name == "" {
		return nil
	}
	return p.themeData[name]
}

// nextAvailableMasterPartName returns a slideMaster part name not already used
// by a master or another part.
func (p *Presentation) nextAvailableMasterPartName() string {
	used := make(map[string]bool, len(p.slideMasters)+len(p.otherParts))
	for _, m := range p.slideMasters {
		if m.partName != "" {
			used[m.partName] = true
		}
	}
	for name := range p.otherParts {
		used[name] = true
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/slideMasters/slideMaster%d.xml", i)
		if !used[name] && !p.partNameTaken(name) {
			return name
		}
	}
}

// freeThemePartName returns a theme part name not already present in themeData or
// the package. theme1.xml is reserved for the deck's default/presentation theme
// (the from-scratch save path always writes it), so allocation starts at 2.
func (p *Presentation) freeThemePartName() string {
	for i := 2; ; i++ {
		name := fmt.Sprintf("/ppt/theme/theme%d.xml", i)
		if _, ok := p.themeData[name]; ok {
			continue
		}
		if p.partNameTaken(name) {
			continue
		}
		return name
	}
}

// importNotesSlide copies the source notes slide part into p, re-wiring its
// relationships: the back-reference points at the new slide, the notes-master
// reference is repointed at the destination's notes master (or dropped when it
// has none), and any media the notes slide embeds is carried. Relationship ids
// are preserved so the notes XML's own r:id references stay valid. Returns the
// new notes part name, or "" when the source part is missing.
func (p *Presentation) importNotesSlide(srcPres *Presentation, srcNotes, newSlidePart string, ctx *mergeCtx) string {
	part, ok := srcPres.otherParts[srcNotes]
	if !ok {
		return ""
	}
	newName := p.nextAvailableNotesName()
	copied := *part
	copied.Data = bytes.Clone(part.Data)
	p.otherParts[newName] = &copied
	ctx.parts[srcNotes] = newName

	var notesRels []*opc.Relationship
	for _, rel := range srcPres.relationships[srcNotes] {
		if rel == nil {
			continue
		}
		c := *rel
		switch rel.Type {
		case opc.RelTypeSlide:
			c.Target = relativeTarget(newName, newSlidePart)
		case opc.RelTypeNotesMaster:
			master := p.notesMasterPartName()
			if master == "" {
				// The destination has no notes master; drop the reference rather
				// than carry the source's (deferred).
				continue
			}
			c.Target = relativeTarget(newName, master)
		default:
			if rel.TargetMode == opc.TargetModeExternal {
				notesRels = append(notesRels, &c)
				continue
			}
			st := opc.ResolvePartName(srcNotes, rel.Target)
			nt := p.importPart(srcPres, st, ctx.parts)
			if nt == "" {
				continue
			}
			c.Target = relativeTarget(newName, nt)
		}
		notesRels = append(notesRels, &c)
	}
	if len(notesRels) > 0 {
		p.relationships[newName] = notesRels
	}
	return newName
}

// importHandoutMaster carries the source deck's handout master — its part, the
// theme it references, its own relationships, the presentation relationship, and
// the handoutMasterIdLst entry — into p, but only when the source has a handout
// master and p does not (a deck has at most one). Slides never reference the
// handout master, so this runs once per merge rather than per slide.
func (p *Presentation) importHandoutMaster(srcPres *Presentation, ctx *mergeCtx) {
	if p.handoutMasterPartName() != "" {
		return // destination already has one; do not duplicate.
	}
	srcName := srcPres.handoutMasterPartName()
	if srcName == "" {
		return
	}
	part, ok := srcPres.otherParts[srcName]
	if !ok {
		return
	}

	newName := p.nextAvailableHandoutMasterName()
	copied := *part
	copied.Data = bytes.Clone(part.Data)
	p.otherParts[newName] = &copied
	ctx.parts[srcName] = newName

	// Carry the handout master's own relationships, remapping the theme to a fresh
	// theme part and any other internal targets to carried auxiliary parts.
	var rels []*opc.Relationship
	for _, rel := range srcPres.relationships[srcName] {
		if rel == nil {
			continue
		}
		c := *rel
		if rel.TargetMode == opc.TargetModeExternal {
			rels = append(rels, &c)
			continue
		}
		st := opc.ResolvePartName(srcName, rel.Target)
		var nt string
		if rel.Type == opc.RelTypeTheme {
			nt = p.importThemePart(srcPres, st)
		} else {
			nt = p.importPart(srcPres, st, ctx.parts)
		}
		if nt == "" {
			continue
		}
		c.Target = relativeTarget(newName, nt)
		rels = append(rels, &c)
	}
	if len(rels) > 0 {
		p.relationships[newName] = rels
	}

	// presentation -> handoutMaster relationship + handoutMasterIdLst entry.
	presRels := p.relationships[presentationPartName]
	relID := fmt.Sprintf("rId%d", nextRelationshipID(presRels))
	p.relationships[presentationPartName] = append(presRels, &opc.Relationship{
		ID:         relID,
		Type:       opc.RelTypeHandoutMaster,
		Target:     partNameToRelTarget(newName, "/ppt/"),
		TargetMode: opc.TargetModeInternal,
	})
	if p.presentation.HandoutMasterIDs == nil {
		p.presentation.HandoutMasterIDs = &oxml.HandoutMasterIDs{}
	}
	p.presentation.HandoutMasterIDs.HandoutMasterID = append(
		p.presentation.HandoutMasterIDs.HandoutMasterID,
		oxml.HandoutMasterID{RID: relID},
	)
}

// handoutMasterPartName returns the deck's handout master part name, or "" when
// it has none. Keys are scanned in sorted order so the choice is deterministic.
func (p *Presentation) handoutMasterPartName() string {
	for _, name := range sortedKeys(p.otherParts) {
		if strings.HasPrefix(name, "/ppt/handoutMasters/") && strings.HasSuffix(name, ".xml") {
			return name
		}
	}
	return ""
}

// importThemePart copies the theme part a source master (handout or notes)
// references into p.themeData under a free name, returning that name (or ""
// when the theme is absent from the source).
func (p *Presentation) importThemePart(srcPres *Presentation, srcTheme string) string {
	data, ok := srcPres.themeData[srcTheme]
	if !ok {
		return ""
	}
	newTheme := p.freeThemePartName()
	p.themeData[newTheme] = bytes.Clone(data)
	return newTheme
}

// nextAvailableHandoutMasterName returns a handout master part name not already
// used in the package.
func (p *Presentation) nextAvailableHandoutMasterName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/handoutMasters/handoutMaster%d.xml", i)
		if !p.partNameTaken(name) {
			return name
		}
	}
}

// importNotesMaster carries the source deck's notes master — its part, the
// theme it references, its own relationships, the presentation relationship, and
// the notesMasterIdLst entry — into p, but only when the source has a notes
// master and p does not (a deck has at most one). It mirrors importHandoutMaster;
// unlike the handout master, notes slides reference the notes master, so this is
// run before the slides are imported (see AppendSlidesFrom) and their notesMaster
// references are re-wired to it in importNotesSlide.
func (p *Presentation) importNotesMaster(srcPres *Presentation, ctx *mergeCtx) {
	if p.notesMasterPartName() != "" {
		return // destination already has one; do not duplicate.
	}
	srcName := srcPres.notesMasterPartName()
	if srcName == "" {
		return
	}
	part, ok := srcPres.otherParts[srcName]
	if !ok {
		return
	}

	newName := p.nextAvailableNotesMasterName()
	copied := *part
	copied.Data = bytes.Clone(part.Data)
	p.otherParts[newName] = &copied
	ctx.parts[srcName] = newName

	// Carry the notes master's own relationships, remapping the theme to a fresh
	// theme part and any other internal targets to carried auxiliary parts.
	var rels []*opc.Relationship
	for _, rel := range srcPres.relationships[srcName] {
		if rel == nil {
			continue
		}
		c := *rel
		if rel.TargetMode == opc.TargetModeExternal {
			rels = append(rels, &c)
			continue
		}
		st := opc.ResolvePartName(srcName, rel.Target)
		var nt string
		if rel.Type == opc.RelTypeTheme {
			nt = p.importThemePart(srcPres, st)
		} else {
			nt = p.importPart(srcPres, st, ctx.parts)
		}
		if nt == "" {
			continue
		}
		c.Target = relativeTarget(newName, nt)
		rels = append(rels, &c)
	}
	if len(rels) > 0 {
		p.relationships[newName] = rels
	}

	// presentation -> notesMaster relationship + notesMasterIdLst entry.
	presRels := p.relationships[presentationPartName]
	relID := fmt.Sprintf("rId%d", nextRelationshipID(presRels))
	p.relationships[presentationPartName] = append(presRels, &opc.Relationship{
		ID:         relID,
		Type:       opc.RelTypeNotesMaster,
		Target:     partNameToRelTarget(newName, "/ppt/"),
		TargetMode: opc.TargetModeInternal,
	})
	if p.presentation.NotesMasterIDs == nil {
		p.presentation.NotesMasterIDs = &oxml.NotesMasterIDs{}
	}
	p.presentation.NotesMasterIDs.NotesMasterID = append(
		p.presentation.NotesMasterIDs.NotesMasterID,
		oxml.NotesMasterID{RID: relID},
	)
}

// nextAvailableNotesMasterName returns a notes master part name not already
// used in the package.
func (p *Presentation) nextAvailableNotesMasterName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/notesMasters/notesMaster%d.xml", i)
		if !p.partNameTaken(name) {
			return name
		}
	}
}

// matchLayout returns the destination layout whose type matches src, falling
// back to the first layout. It returns nil only when the deck has no layouts.
func (p *Presentation) matchLayout(src *SlideLayout) *SlideLayout {
	if len(p.slideLayouts) == 0 {
		return nil
	}
	if src != nil {
		for _, l := range p.slideLayouts {
			if l.layoutType == src.layoutType {
				return l
			}
		}
	}
	return p.slideLayouts[0]
}
