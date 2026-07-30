package docx

import "time"

// This file implements the "stamp dcterms:modified if and only if the content
// changed" rule.
//
// docx never touched Properties.Modified: a document that was edited beyond
// recognition kept whatever write time its producer had left in
// docProps/core.xml, and a document built by Create carried none at all. The
// obvious fix — stamp on every save — is the defect pptx spent three audits
// calling a flaky test: it makes SaveBytes non-idempotent, so an untouched
// document saved twice either side of a second boundary produces two different
// packages and defeats content-hash dedup and reproducible builds. The two
// requirements only conflict when nothing changed, which is why there is no
// option to choose between them.
//
// Detection reuses the flags the save path already trusts, rather than adding a
// second, independently wrong record of what happened:
//
//   - Paragraph.touch() (mutate.go) is the funnel every body, header and footer
//     mutator already reaches, because an edit into a preserved header is
//     dropped unless it flags the part. It was a no-op for the main document
//     part; it now records the edit there too.
//   - The metadata parts are gated on stylesModified, numberingModified and
//     friends. Their setters (mutate.go) record the edit as they raise the flag.
//   - The main document part has no flag of its own — it is regenerated whenever
//     the body was materialized — so the handles that reach it without touching
//     a paragraph (Section, Table, TableRow, TableCell, ContentControl) get a
//     touch() of their own, and the document-level body mutators call
//     markEdited directly.
//   - The two edits that reach no model at all — a custom-XML part queued for
//     writing, and the theme editor, which is a dml type mutated outside this
//     package — record themselves: AddCustomXMLPart calls markEdited, and
//     themeEdited reads the editor's own flag against a per-save baseline.
//
// If any of those is wrong the library already miscompiles the document, so the
// timestamp inherits their correctness instead of adding a second failure mode.
// TestMutationsFlagTheirPart derives the whole set from the source, so a mutator
// added tomorrow is covered the day it is written.
//
// The one thing detection must NOT key off is the regeneration decision.
// Reading anything in the body materializes docModel and so forces the main
// part to be regenerated from the model, but reading is not editing: keying the
// stamp off "will this part be regenerated" would bump the write time of a
// document the caller only read. See TestReadOnlyAccessDoesNotStampModified and
// TestZeroModificationRaisesNoFlag.

// markEdited records that the caller changed something this session.
//
// It is a counter, not a flag, because the flags it is fed from never reset:
// two edits either side of a save have to be distinguishable from one edit
// saved twice, and a boolean that has already latched cannot tell them apart.
// A missing call costs a stale timestamp, never a lost edit — persistence is
// the flags' job, not this counter's.
func (d *Document) markEdited() {
	if d == nil {
		return
	}
	d.modelEdits++
}

// stampModified records the save time in Properties.Modified when, and only
// when, this session changed the document's content since it was opened,
// created, or last saved. Called from the save path before either save strategy
// runs, so the stamped value is what saveRoundTrip's snapshot comparison
// observes — which is also what makes a content edit regenerate
// docProps/core.xml instead of preserving the source bytes.
func (d *Document) stampModified() {
	if !d.contentChanged() {
		return
	}
	// An explicit Properties.Modified assignment is itself a property edit, and
	// the caller's value wins over ours. stampedModified distinguishes that from
	// the value a previous save of this same Document wrote: after a save both
	// show up as Modified differing from the baseline, so without it a second
	// round of edits could never stamp again. A created document has no
	// snapshot, and its baseline is the zero time Create left behind.
	var baseline time.Time
	if d.propsSnapshot != nil {
		baseline = d.propsSnapshot.Modified
	}
	if !d.Properties.Modified.Equal(baseline) && !d.Properties.Modified.Equal(d.stampedModified) {
		return
	}
	// A package opened without a /docProps/core.xml part must not gain one:
	// System.IO.Packaging producers keep core properties in a *.psmdcp part and
	// others omit them entirely, and synthesizing the part would change the
	// package shape for a caller who only edited a paragraph. saveRoundTrip
	// writes the edited properties back into whichever part holds them.
	if d.reader != nil && !d.hasCoreProps {
		return
	}
	now := time.Now()
	d.Properties.Modified = now
	d.stampedModified = now
}

// noteSaved records the change state a completed save has just persisted, so a
// repeat save of an untouched document reproduces the same bytes instead of
// stamping again. Without it the edit counter — which nothing else resets —
// would report the same edit as outstanding forever.
func (d *Document) noteSaved() {
	d.savedModelEdits = d.modelEdits
	d.savedThemeEdit = d.themeEdited()
}

// themeEdited reports whether the theme handed out by Document.Theme carries an
// edit. The editor is a dml type mutated outside this package, so there is no
// call here to hook; its own Modified flag is the signal the save already uses
// to decide whether to re-serialize the part (see regeneratedThemePart).
//
// It latches — nothing resets it — which is why noteSaved records its value and
// contentChanged compares against that. A second theme edit made after a save
// is therefore not distinguishable from the first, and costs a stale timestamp
// rather than a lost edit; the part is re-serialized either way.
func (d *Document) themeEdited() bool {
	return d.theme != nil && d.theme.Modified()
}

// contentChanged reports whether this session changed anything that will be
// written to the package, relative to the last completed save (or to Open /
// Create when there has not been one). None of the signals it reads is set by
// merely reading the document.
//
// Deliberately not counted: an edit to Properties or to the custom properties.
// Those are already detected by comparing against the snapshot taken at open —
// which is what regenerates docProps/core.xml and docProps/custom.xml — and a
// caller authoring the document's metadata is stating what the metadata should
// say, not asking for a write time on top. pptx draws the line in the same
// place.
func (d *Document) contentChanged() bool {
	return d.modelEdits != d.savedModelEdits || d.themeEdited() != d.savedThemeEdit
}
