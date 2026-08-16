package pptx

// Notes slides and modern (threaded) comment parts are edited through parsed
// models rather than through their serialized bytes.
//
// The motivation is that serializing can fail. A part read off a wild package
// can carry a name no namespace-aware consumer accepts, and the Builder refuses
// to write one rather than emit a part nothing can parse. When every setter
// serialized on the spot, that error had nowhere to go but the setter's
// signature, and SetNotes/AddComment/Reply/Resolve/SetResolved all grew an
// error return for a failure none of them causes.
//
// So the setters mutate a model and mark it dirty, and flushPendingParts turns
// the dirty models back into bytes at save, where there has always been an
// error to return. The cost is that between a setter and the next flush the
// bytes in otherParts are stale — which is why every in-session reader of these
// two part kinds goes through the accessors here instead of reading
// otherParts directly. Getting that wrong does not fail loudly: an unflushed
// comment part reads back as a deck with no comments.
//
// Only these two part kinds are modeled. The rest of otherParts (media, OLE,
// diagram data, ink, ...) is opaque bytes this library never re-serializes, so
// there is nothing to defer.

import (
	"fmt"

	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// notesEntry is a parsed notes slide and whether it has diverged from the bytes
// in otherParts. A nil model means the part does not parse; the entry is still
// cached so the failure is not retried on every Notes() call.
type notesEntry struct {
	model *oxml.NotesSlide
	dirty bool
}

// commentEntry is a parsed modern comment part, with the same nil-means-
// unparseable convention as notesEntry.
type commentEntry struct {
	model *oxml.ModernCommentPart
	dirty bool
}

// notesModel returns the parsed notes slide at partName, parsing and caching it
// on first use. It returns nil when the part is absent or does not parse.
//
// Every caller gets the same instance: that is what makes a SetNotes edit
// visible to the Notes() call after it, and to the save, without either going
// near the serialized form.
func (p *Presentation) notesModel(partName string) *oxml.NotesSlide {
	if e, ok := p.notesModels[partName]; ok {
		return e.model
	}
	e := &notesEntry{}
	if data := p.rawPartData(partName); data != nil {
		var ns oxml.NotesSlide
		// UnmarshalWithSource registers the raw bytes so the root attribute
		// capture (and the capture kit below it) can recover the producer's
		// verbatim rendering rather than a re-synthesized one.
		if err := xmlb.UnmarshalWithSource(data, &ns); err == nil {
			e.model = &ns
		}
	}
	if p.notesModels == nil {
		p.notesModels = make(map[string]*notesEntry)
	}
	p.notesModels[partName] = e
	return e.model
}

// putNotesModel installs a freshly built notes slide at partName and marks it
// for serialization. The otherParts entry is created here so part-name
// allocation and the notes-part count see it immediately; its bytes are filled
// in by the flush.
func (p *Presentation) putNotesModel(partName string, ns *oxml.NotesSlide) {
	if p.notesModels == nil {
		p.notesModels = make(map[string]*notesEntry)
	}
	p.notesModels[partName] = &notesEntry{model: ns, dirty: true}
	if _, ok := p.otherParts[partName]; !ok {
		p.otherParts[partName] = &coxml.RawPart{ContentType: opc.ContentTypeNotesSlide}
	}
}

// markNotesDirty records that the cached notes model at partName has been
// edited and must be re-serialized at the next flush.
func (p *Presentation) markNotesDirty(partName string) {
	if e, ok := p.notesModels[partName]; ok {
		e.dirty = true
	}
}

// commentModel returns the parsed modern comment part at partName, parsing and
// caching it on first use. It returns nil when the part is absent or does not
// parse — which is the same thing a reader sees for a part that is not a
// comment part at all, and is why Comments() reports no comments rather than an
// error.
func (p *Presentation) commentModel(partName string) *oxml.ModernCommentPart {
	if e, ok := p.commentModels[partName]; ok {
		return e.model
	}
	e := &commentEntry{}
	if data := p.rawPartData(partName); data != nil {
		if part, err := oxml.ParseModernCommentPart(data); err == nil {
			e.model = part
		}
	}
	if p.commentModels == nil {
		p.commentModels = make(map[string]*commentEntry)
	}
	p.commentModels[partName] = e
	return e.model
}

// putCommentModel installs a freshly built comment thread at partName and marks
// it for serialization (see putNotesModel for why the otherParts entry is
// created now).
func (p *Presentation) putCommentModel(partName string, part *oxml.ModernCommentPart) {
	if p.commentModels == nil {
		p.commentModels = make(map[string]*commentEntry)
	}
	p.commentModels[partName] = &commentEntry{model: part, dirty: true}
	if _, ok := p.otherParts[partName]; !ok {
		p.otherParts[partName] = &coxml.RawPart{ContentType: opc.ContentTypeModernComments}
	}
}

// markCommentDirty records that the cached comment thread at partName has been
// edited (a reply added, a status changed) and must be re-serialized.
func (p *Presentation) markCommentDirty(partName string) {
	if e, ok := p.commentModels[partName]; ok {
		e.dirty = true
	}
}

// markAuthorsDirty records that the shared modern author list has been edited.
// Unlike the per-part models the author list already had a cache
// (modernAuthors); all this adds is the deferral of its serialization.
func (p *Presentation) markAuthorsDirty() {
	p.modernAuthorsDirty = true
	if _, ok := p.otherParts[modernAuthorsPart]; !ok {
		p.otherParts[modernAuthorsPart] = &coxml.RawPart{ContentType: opc.ContentTypeAuthors}
	}
}

// forgetPartModel drops any cached model for a part that has been deleted, so a
// later flush does not resurrect the part it was just asked to remove, and a
// part name reused for different content is not read back through the previous
// occupant's model.
func (p *Presentation) forgetPartModel(partName string) {
	delete(p.notesModels, partName)
	delete(p.commentModels, partName)
	if partName == modernAuthorsPart {
		p.modernAuthors = nil
		p.modernAuthorsLoaded = false
		p.modernAuthorsDirty = false
	}
}

// flushPendingParts serializes every notes slide, comment thread and author list
// this session edited back into otherParts. It is idempotent, and it is the only
// place these three part kinds are serialized.
//
// It must run before anything reads the serialized form: both save paths, the
// validation pass, and the merge/duplicate paths that copy a part by its bytes.
//
// A part that failed to serialize keeps its dirty flag. That is what lets the
// callers with nowhere to report an error — Duplicate is the one that matters —
// treat the flush as best-effort and still have the failure surface, because the
// save's own flush hits it again and does return it.
func (p *Presentation) flushPendingParts() error {
	for _, name := range sortedKeys(p.notesModels) {
		e := p.notesModels[name]
		if !e.dirty || e.model == nil {
			continue
		}
		data, err := marshalNotesSlide(e.model)
		if err != nil {
			return fmt.Errorf("pptx: serializing the notes slide %s: %w", name, err)
		}
		p.storePartData(name, opc.ContentTypeNotesSlide, data)
		e.dirty = false
	}

	for _, name := range sortedKeys(p.commentModels) {
		e := p.commentModels[name]
		if !e.dirty || e.model == nil {
			continue
		}
		data, err := e.model.Marshal()
		if err != nil {
			return fmt.Errorf("pptx: serializing the comment thread %s: %w", name, err)
		}
		p.storePartData(name, opc.ContentTypeModernComments, data)
		e.dirty = false
	}

	if p.modernAuthorsDirty && p.modernAuthors != nil {
		data, err := p.modernAuthors.Marshal()
		if err != nil {
			return fmt.Errorf("pptx: serializing %s: %w", modernAuthorsPart, err)
		}
		p.storePartData(modernAuthorsPart, opc.ContentTypeAuthors, data)
		p.modernAuthorsDirty = false
	}
	return nil
}

// storePartData writes serialized bytes into otherParts, creating the entry when
// the flush is materializing a part this session invented and leaving an
// existing entry's content type alone (a source may type a part more specifically
// than the constant here).
func (p *Presentation) storePartData(partName, contentType string, data []byte) {
	if part, ok := p.otherParts[partName]; ok && part != nil {
		part.Data = data
		if part.ContentType == "" {
			part.ContentType = contentType
		}
		return
	}
	p.otherParts[partName] = &coxml.RawPart{ContentType: contentType, Data: data}
}

// flushBestEffort serializes the pending models on a path that has no way to
// report the failure. See flushPendingParts: a failure leaves the model dirty,
// so the save that follows reports it and refuses to write the package.
func (p *Presentation) flushBestEffort() {
	_ = p.flushPendingParts()
}
