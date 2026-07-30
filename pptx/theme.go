package pptx

// Theme access for PresentationML.
//
// pptx used to expose a package-local read-only *pptx.Theme here while docx and
// xlsx both handed back the shared *dml.ThemeEditor — the largest
// same-name-different-type collision in the public surface, and one that made a
// theme routine written for a Word document simply not compile against a deck
// (C571). Both now return *dml.ThemeEditor.
//
// The reason the collision survived the previous wave was C374: regenerating
// the theme part from the narrow model would have deleted custClrLst and the
// extLst carrying thm15:themeFamily, so the pptx setters were removed instead
// of wired. dml.Theme models both (and the extLst of five nested types), and
// ThemeEditor.Marshal replays the source root attribute list verbatim, so
// regeneration is lossless — pinned by TestThemeEditPreservesExtensions in
// common/dml and TestThemeEditKeepsExtensionsDocx in docx. That is what makes
// converging safe now and unsafe then.
//
// The round-trip contract matches docx and xlsx exactly: an untouched editor
// leaves the source part bytes in place, so a deck whose theme is merely read
// still saves byte-for-byte.

import (
	"bytes"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
)

// themeEditorFor parses a theme part into an editor, caching it per part name
// so two masters sharing one theme part share one editor (and one modified
// flag) rather than racing to regenerate the same bytes from divergent models.
func (p *Presentation) themeEditorFor(partName string) *dml.ThemeEditor {
	if ed, ok := p.themeEditors[partName]; ok {
		return ed
	}
	data, ok := p.themeData[partName]
	if !ok {
		p.themeEditors[partName] = nil
		return nil
	}
	var t dml.Theme
	if err := xmlb.Unmarshal(data, &t); err != nil {
		p.themeEditors[partName] = nil
		return nil
	}
	ed := dml.NewThemeEditor(&t, data)
	p.themeEditors[partName] = ed
	return ed
}

// resolveThemes binds each slide master to its theme part. The raw theme bytes
// stay preserved for round-trip; a theme part that fails to parse just leaves
// the master's Theme nil.
func (p *Presentation) resolveThemes() {
	for _, master := range p.slideMasters {
		for _, rel := range p.relationships[master.partName] {
			if rel == nil || rel.Type != opc.RelTypeTheme {
				continue
			}
			themeName := opc.ResolvePartName(master.partName, rel.Target)
			if _, ok := p.themeData[themeName]; !ok {
				break
			}
			master.resolvedThemePart = themeName
			break
		}
	}
}

// applyThemeEdits replaces the preserved bytes of every theme part whose editor
// was modified this session with its re-serialization, so the save writes the
// edit. Untouched theme parts are left exactly as they were read, which is what
// keeps an unmodified deck byte-identical.
//
// A marshal failure leaves the source bytes in place: dropping a theme part
// would break every master that references it, and writing the original is the
// only safe fallback. It is unreachable in practice — the same model was
// serialized on the way in.
func (p *Presentation) applyThemeEdits() {
	for name, ed := range p.themeEditors {
		if ed == nil || !ed.Modified() {
			continue
		}
		data, err := ed.Marshal()
		if err != nil {
			continue
		}
		// Record the edit only when the bytes actually move. dml.ThemeEditor's
		// modified bit never resets, so keying off it would report the same theme
		// edit as outstanding on every subsequent save and re-stamp
		// dcterms:modified each time; comparing the serialization against what is
		// already stored answers "changed since the last save" instead.
		if !bytes.Equal(p.themeData[name], data) {
			p.markModelEdited()
		}
		p.themeData[name] = data
	}
}

// Theme returns a read/write handle to the presentation theme: the theme part
// of the first slide master, as the shared DrawingML a:theme model exposed by
// dml.ThemeEditor — the same type docx.Document.Theme and xlsx.Workbook.Theme
// return (C571).
//
// Color-scheme and font-scheme edits made through the handle are written back
// to the theme part on save; an untouched theme round-trips byte-for-byte from
// its preserved source bytes. A deck whose masters share one theme part shares
// one handle, so an edit made through any of them is written once.
//
// It returns nil for presentations created programmatically (their default
// theme part is not modeled) and for masters whose theme part is missing or
// unparseable.
func (p *Presentation) Theme() *dml.ThemeEditor {
	if len(p.slideMasters) == 0 {
		return nil
	}
	return p.slideMasters[0].Theme()
}
