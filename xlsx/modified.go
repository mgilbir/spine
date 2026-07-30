package xlsx

import (
	"bytes"
	"time"
)

// This file implements the "stamp dcterms:modified if and only if the content
// changed" rule, matching pptx (see pptx/modified.go).
//
// The rule exists because the two obvious alternatives are both wrong. Stamping
// on every save makes SaveBytes non-idempotent: an unchanged workbook saved
// twice either side of a second boundary produces different docProps/core.xml —
// dcterms:modified is written at RFC3339 (one second) resolution — and the
// byte-identical round-trip the corpus gate depends on is lost. Never stamping
// (what xlsx did) means a workbook that really was edited keeps whatever write
// time it was opened with, or none at all for one built with Create.
//
// Detection is a counter with a per-save baseline, never a latch. Latching
// booleans cannot tell "one edit saved twice" from "two edits either side of a
// save": the flags this package already keeps (Sheet.dirty, Workbook.stylesDirty,
// …) are never cleared, so after the first save every one of them still reads
// "changed" forever. A monotonic count compared against the value recorded by
// the last completed save answers both questions.
//
// What increments the counter is deliberately not a new parallel flag. It is
// bumped from inside the regeneration flags that are already load-bearing —
// Sheet.markDirty (which refuses opaque sheets, C241/C423), the StyleManager
// onModify hook behind Workbook.stylesDirty, Workbook.sheetsDirty,
// Workbook.vbaModified, Workbook.personsDirty — so if those are wrong the
// library already miscompiles the workbook and the timestamp inherits their
// correctness instead of adding a second, independent way to be wrong. The
// remaining mutators are the ones that need no flag because they write
// workbook.xml, which is regenerated from the model on every save: those call
// markContentEdited directly, and TestExportedMutatorsRecordAnEdit drives the
// package's exported mutators by AST to keep the set complete.
//
// The one thing detection must NOT key off is the regeneration decision itself.
// xlsx has two traps here. Sheet.Cell is a materializing accessor by design
// (C425): looking up a reference creates the <row>/<c> for it, so a pure read
// mutates the model — and Workbook.Styles materialized a default stylesheet
// (and set stylesDirty) until PR #257, so reading styles grew the saved package
// by three parts. Neither reaches the counter. Nor does "was this part
// regenerated": xlsx ALWAYS regenerates workbook.xml, which makes that proxy
// even worse here than in pptx. See TestReadOnlyAccessDoesNotStampModified.

// markContentEdited records that the caller changed content that will be
// written to the package.
//
// It is a counter, not a flag, because none of this package's regeneration
// flags is ever reset: two edits either side of a save have to be
// distinguishable from one edit saved twice, and a boolean that has already
// latched cannot tell them apart. Missing a call costs a stale timestamp, never
// a lost edit; persistence depends on the regeneration flags, not on this.
func (w *Workbook) markContentEdited() {
	if w == nil {
		return
	}
	w.contentEdits++
}

// contentChanged reports whether this session changed anything that will be
// written to the package, relative to the last completed save (or to Open /
// Create when there has not been one). None of the signals it reads is set by
// merely reading the workbook.
func (w *Workbook) contentChanged() bool {
	return w.contentEdits != w.savedContentEdits
}

// noteSaved records the change state a completed save has just persisted, so a
// repeat save of an untouched workbook reproduces the same bytes instead of
// stamping again. Without it the counter — which nothing else resets — would
// report the same edit as outstanding forever.
//
// It runs after the save, so edits the save path itself makes (attaching a
// drawing reference marks its sheet dirty) are part of the baseline rather than
// outstanding work for the next save.
func (w *Workbook) noteSaved() {
	w.savedContentEdits = w.contentEdits
}

// applyThemeEdits records a theme edit made through Workbook.Theme.
//
// dml.ThemeEditor exposes only a latching Modified bit, so keying off it would
// report the same theme edit as outstanding on every subsequent save and
// re-stamp dcterms:modified each time. Comparing the re-serialized theme
// against the bytes the part currently holds answers "changed since the last
// save" instead, and storing the result makes those bytes the new baseline —
// the same shape pptx's applyThemeEdits uses.
func (w *Workbook) applyThemeEdits() {
	if w.theme == nil || !w.theme.Modified() || w.themePartName == "" {
		return
	}
	part := w.preservedParts[w.themePartName]
	if part == nil {
		return
	}
	data, err := w.theme.Marshal()
	if err != nil {
		return
	}
	if !bytes.Equal(part.Data, data) {
		w.markContentEdited()
	}
	part.Data = data
}

// stampModified records the save time in Properties.Modified when, and only
// when, this session changed the workbook's content since it was opened,
// created, or last saved. Called from the save path before either save strategy
// runs, so the stamped value is what saveRoundTrip's snapshot comparison and
// saveNew's writer both observe — which is also what makes a content edit
// regenerate docProps/core.xml instead of preserving the source bytes.
func (w *Workbook) stampModified() {
	if !w.contentChanged() {
		return
	}
	// An explicit Properties.Modified assignment is itself a property edit, and
	// the caller's value wins over ours. stampedModified distinguishes that from
	// the value a previous save of this same Workbook wrote: after a save both
	// show up as Modified differing from propsSnapshot, so without it a second
	// round of edits could never stamp again.
	if w.propsSnapshot != nil &&
		!w.Properties.Modified.Equal(w.propsSnapshot.Modified) &&
		!w.Properties.Modified.Equal(w.stampedModified) {
		return
	}
	// A package opened without a /docProps/core.xml part must not gain one:
	// some producers (System.IO.Packaging) keep core properties in a *.psmdcp
	// part, others omit them entirely or reach them through a malformed
	// relationship type this package leaves unparsed. Synthesizing the part
	// would change the package shape for a caller who only edited a cell.
	// saveRoundTrip makes the same call for unmodified properties; this keeps
	// the two consistent.
	if w.opened {
		if _, ok := w.preservedParts["/docProps/core.xml"]; !ok || !w.hasCoreProps {
			return
		}
	}
	now := time.Now()
	w.Properties.Modified = now
	w.stampedModified = now
}
