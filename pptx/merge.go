package pptx

import (
	"encoding/xml"
	"errors"
	"path"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

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
// Appended slides adopt the destination deck's layout whose type matches the
// source slide's layout (falling back to the first layout); the source deck's
// slide masters, layouts, and themes are not imported. Notes slides are not
// carried over. Full slide-master/theme reconciliation is deferred.
func (p *Presentation) AppendSlidesFrom(other *Presentation) error {
	if other == nil {
		return ErrNilPresentation
	}
	if other == p {
		return errors.New("pptx: cannot append a presentation to itself")
	}

	// partMap deduplicates shared parts (e.g. the same image referenced by two
	// slides) across the whole append so they are copied once.
	partMap := make(map[string]string)
	for _, src := range other.slides {
		ns, err := p.importSlide(src, partMap)
		if err != nil {
			return err
		}
		ns.materializeShapes()
	}
	return nil
}

// ExtractSlides returns a new presentation containing copies of the slides at
// the given indices, in the order requested. The referenced parts (media,
// charts, embeddings) are copied with fresh part names and remapped
// relationships, so the extracted deck opens without dangling references.
//
// The extracted deck uses a fresh default master/layout set; slides adopt the
// default layout matching their source layout type. Source master/theme
// styling is not imported (deferred).
func (p *Presentation) ExtractSlides(indices []int) (*Presentation, error) {
	for _, idx := range indices {
		if idx < 0 || idx >= len(p.slides) {
			return nil, ErrSlideIndex
		}
	}

	out := Create()
	partMap := make(map[string]string)
	for _, idx := range indices {
		ns, err := out.importSlide(p.slides[idx], partMap)
		if err != nil {
			return nil, err
		}
		ns.materializeShapes()
	}
	return out, nil
}

// importSlide copies src (which belongs to another, or the same, presentation)
// into p as a new trailing slide, carrying its referenced parts and remapping
// their relationships. partMap tracks already-copied source parts so shared
// media is not duplicated.
func (p *Presentation) importSlide(src *Slide, partMap map[string]string) (*Slide, error) {
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
	ns.slideXML = &copyXML

	srcPres := src.presentation
	var newRels []*opc.Relationship
	for _, rel := range srcPres.relationships[src.partName] {
		if rel == nil {
			continue
		}
		switch rel.Type {
		case opc.RelTypeSlideLayout:
			// Remapped onto a destination layout below; the slide body never
			// references the layout by r:id, so dropping the rel is safe.
			continue
		case opc.RelTypeNotesSlide:
			// Notes slides pull in the notes master/theme chain; not carried.
			continue
		}
		if rel.TargetMode == opc.TargetModeExternal {
			c := *rel
			newRels = append(newRels, &c)
			continue
		}
		srcTarget := opc.ResolvePartName(src.partName, rel.Target)
		newTarget := p.importPart(srcPres, srcTarget, partMap)
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

	ns.layout = p.matchLayout(src.layout)
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
