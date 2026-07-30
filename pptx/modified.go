package pptx

import "time"

// This file implements the "stamp dcterms:modified if and only if the content
// changed" rule.
//
// The rule exists because the two obvious alternatives are both wrong. Stamping
// on every save (what saveNew used to do) makes SaveBytes non-idempotent: an
// unchanged deck saved twice either side of a second boundary produces different
// docProps/core.xml, which is what made TestFurnitureDeterministic fail about
// once in 300 runs and get written off as flaky for three audits. Never stamping
// means a deck that really was edited records a stale write time.
//
// Detection has two halves, split along how the two halves of the package
// already work.
//
// Slide content reuses the flags marshal() itself trusts to decide whether the
// domain shapes need flushing into the slide XML (slide.go: shapesModified ||
// hasDirtyShapes) plus the deferred-edit queues it drains — removals, image
// replacements, authored animations. Those are load-bearing: if they are wrong
// the library already miscompiles the slide, so the timestamp inherits their
// correctness rather than introducing a second, independent way to be wrong.
// They also reset themselves, because the save clears them once it has flushed.
//
// Everything above the shape layer had no signal at all. Masters, layouts,
// presentation.xml and the preserved raw parts (notes, comments, VBA, embedded
// fonts) are regenerated or rewritten unconditionally, so a mutator there
// persists its edit without needing to flag anything — which is exactly why
// nothing recorded that an edit happened. markModelEdited is that record. It is
// one hook, not a flag per mutator, and TestEveryMutatorMarksTheDeck drives the
// package's exported mutators by reflection to keep it complete.
//
// The one thing detection must NOT key off is the regeneration decision itself.
// Accessing a slide materializes its model (sxModel != nil) and so forces the
// part to be regenerated, but reading is not editing: keying the stamp off "will
// this part be regenerated" would bump the timestamp for a caller who only read
// the deck. See TestReadOnlyAccessDoesNotStampModified.

// markModelEdited records that the caller changed content this package writes
// out unconditionally — a master, a layout, presentation.xml, or one of the
// preserved raw parts — and which therefore needs no flag to persist.
//
// It is a counter, not a flag, because these edits are never reset: two edits
// either side of a save have to be distinguishable from one edit saved twice,
// and a boolean that has already latched cannot tell them apart. Missing a call
// costs a stale timestamp, never a lost edit; persistence does not depend on it.
func (p *Presentation) markModelEdited() {
	if p == nil {
		return
	}
	p.modelEdits++
}

// stampModified records the save time in Properties.Modified when, and only
// when, this session changed the deck's content since it was opened, created,
// or last saved. Called from the save path before either save strategy runs, so
// the stamped value is what saveRoundTrip's propsEdited comparison and saveNew's
// writer both observe — which is also what makes a content edit regenerate
// docProps/core.xml instead of preserving the source bytes.
func (p *Presentation) stampModified() {
	if !p.contentChanged() {
		return
	}
	// An explicit Properties.Modified assignment is itself a property edit, and
	// the caller's value wins over ours. stampedModified distinguishes that from
	// the value a previous save of this same Presentation wrote: after a save
	// both show up as Modified differing from propsSnapshot, so without it a
	// second round of edits could never stamp again.
	if p.propsSnapshot != nil &&
		!p.Properties.Modified.Equal(p.propsSnapshot.Modified) &&
		!p.Properties.Modified.Equal(p.stampedModified) {
		return
	}
	// A package opened without a /docProps/core.xml part must not gain one:
	// several producers keep core properties in a *.psmdcp part or omit them
	// entirely, and synthesizing the part would change the package shape for a
	// caller who only edited a slide. saveRoundTrip makes the same call for
	// unmodified properties; this keeps the two consistent.
	if p.reader != nil && !p.hasCorePart {
		return
	}
	now := time.Now()
	p.Properties.Modified = now
	p.stampedModified = now
}

// noteSaved records the change state a completed save has just persisted, so a
// repeat save of an untouched deck reproduces the same bytes instead of
// stamping again. Without it the model-edit counter — which nothing else resets
// — would report the same edit as outstanding forever.
func (p *Presentation) noteSaved() {
	p.savedModelEdits = p.modelEdits
}

// contentChanged reports whether this session changed anything that will be
// written to the package, relative to the last completed save (or to open /
// Create when there has not been one). None of the signals it reads is set by
// merely reading the deck.
func (p *Presentation) contentChanged() bool {
	// Slide content: the shape-level flags marshal() itself consults, plus the
	// deferred edits it flushes. The save clears them, so they need no baseline.
	for _, slide := range p.slides {
		if slide.contentEdited() {
			return true
		}
	}
	// Everything above the shape layer, counted rather than flagged.
	return p.modelEdits != p.savedModelEdits
}

// contentEdited reports whether this slide's content was changed through the
// API. It is the condition marshal() uses to decide a shape sync is needed
// (slide.go: shapesModified || hasDirtyShapes) widened to the edits marshal
// flushes through other paths — removals, image replacements, authored
// animations — each already load-bearing for the save.
//
// Deliberately absent: sxModel != nil. That records only that the slide was
// materialized, which happens on the first read of any property, so including
// it would make reading a deck bump its modification time.
func (s *Slide) contentEdited() bool {
	if s == nil {
		return false
	}
	if s.shapesModified || s.forceShapeRebuild || len(s.removedRefs) > 0 || len(s.pendingAnims) > 0 {
		return true
	}
	if s.hasDirtyShapes() {
		return true
	}
	// A pending image replacement is held on the shape until marshal processes
	// it, and neither PlaceholderShape.SetImage* nor Picture.SetImage* marks the
	// shape dirty. Mirror exactly what processPendingImages looks for, so the
	// two cannot disagree about whether an image swap is outstanding.
	for _, shape := range s.shapeCache {
		switch sh := shape.(type) {
		case *PlaceholderShape:
			if sh.hasPendingImage() {
				return true
			}
		case *Picture:
			if sh.hasPendingImage() {
				return true
			}
		}
	}
	return false
}
