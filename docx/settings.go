package docx

import (
	"bytes"
	"encoding/xml"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// ensureSettings makes sure the document has a settings model, creating an
// empty one (which the save materializes as /word/settings.xml) when absent.
func (d *Document) ensureSettings() *oxml.CT_Settings {
	if d.settings == nil {
		d.settings = &oxml.CT_Settings{}
	}
	return d.settings
}

// DefaultTabStop returns the document's default tab-stop interval in points
// (w:defaultTabStop) and whether the setting is present. Word's built-in
// default is 36 points (720 twips) when the element is absent.
func (d *Document) DefaultTabStop() (float64, bool) {
	if d.settings == nil {
		return 0, false
	}
	c := d.settings.Child("defaultTabStop")
	if c == nil {
		return 0, false
	}
	return twipsToPoints(c.Attr("val")), true
}

// SetDefaultTabStop sets the document's default tab-stop interval in points
// (w:defaultTabStop), creating the settings part if necessary.
func (d *Document) SetDefaultTabStop(points float64) {
	s := d.ensureSettings()
	s.SetChild("defaultTabStop", []xml.Attr{wAttr("val", pointsToTwips(points))})
	d.settingsModified = true
}

// EvenAndOddHeaders reports whether the document declares distinct even-page
// headers and footers (w:evenAndOddHeaders).
func (d *Document) EvenAndOddHeaders() bool {
	return d.settings != nil && d.settings.Child("evenAndOddHeaders") != nil
}

// SetEvenAndOddHeaders enables or disables distinct even-page headers and
// footers (w:evenAndOddHeaders). Enabling creates the settings part if
// necessary; disabling removes the flag.
func (d *Document) SetEvenAndOddHeaders(on bool) {
	if on {
		s := d.ensureSettings()
		if s.EnsureEvenAndOddHeaders() {
			d.settingsModified = true
		}
		return
	}
	if d.settings != nil && d.settings.RemoveChild("evenAndOddHeaders") {
		d.settingsModified = true
	}
}

// Zoom returns the document's view magnification as a percentage
// (w:zoom/@w:percent) and whether a w:zoom element is present. A zoom element
// without an explicit percent (a named zoom such as "fullPage") reports 0.
func (d *Document) Zoom() (int, bool) {
	if d.settings == nil {
		return 0, false
	}
	c := d.settings.Child("zoom")
	if c == nil {
		return 0, false
	}
	pct, _ := strconv.Atoi(c.Attr("percent"))
	return pct, true
}

// SetZoom sets the document's view magnification to the given percentage
// (w:zoom w:percent), creating the settings part if necessary.
func (d *Document) SetZoom(percent int) {
	s := d.ensureSettings()
	s.SetChild("zoom", []xml.Attr{wAttr("percent", strconv.Itoa(percent))})
	d.settingsModified = true
}

// DocumentVariable is a single document variable (w:docVar): a name paired with
// a string value. Document variables are hidden name/value storage used by
// fields (DOCVARIABLE) and macros.
type DocumentVariable struct {
	Name  string
	Value string
}

// DocumentVariables returns the document variables (w:docVars/w:docVar) in
// document order, or nil when none are defined.
func (d *Document) DocumentVariables() []DocumentVariable {
	if d.settings == nil {
		return nil
	}
	vars := d.settings.DocVars()
	if len(vars) == 0 {
		return nil
	}
	out := make([]DocumentVariable, len(vars))
	for i, v := range vars {
		out[i] = DocumentVariable{Name: v.Name, Value: v.Val}
	}
	return out
}

// DocumentVariable returns the value of the named document variable and whether
// it is defined.
func (d *Document) DocumentVariable(name string) (string, bool) {
	if d.settings == nil {
		return "", false
	}
	for _, v := range d.settings.DocVars() {
		if v.Name == name {
			return v.Val, true
		}
	}
	return "", false
}

// SetDocumentVariable sets the named document variable, replacing an existing
// value or appending a new variable, and creating the settings part if
// necessary.
func (d *Document) SetDocumentVariable(name, value string) {
	s := d.ensureSettings()
	vars := s.DocVars()
	found := false
	for i := range vars {
		if vars[i].Name == name {
			vars[i].Val = value
			found = true
			break
		}
	}
	if !found {
		vars = append(vars, oxml.CT_DocVar{Name: name, Val: value})
	}
	s.SetDocVars(vars)
	d.settingsModified = true
}

// --- Document-level footnote / endnote numbering (w:settings/w:footnotePr,
// w:settings/w:endnotePr) ---
//
// These are the document-wide defaults, distinct from the per-section
// properties on Section.FootnoteProperties / Section.EndnoteProperties. The
// settings element also carries the separator footnote references (w:footnote);
// those are preserved when the numbering is changed.

// FootnoteProperties returns the document-level footnote numbering properties
// (w:settings/w:footnotePr) and whether the element is present. Only the
// numbering fields are reported; separator references are preserved but not
// exposed.
func (d *Document) FootnoteProperties() (NoteProperties, bool) {
	return d.settingsNoteProps("footnotePr")
}

// SetFootnoteProperties sets the document-level footnote numbering properties
// (w:settings/w:footnotePr), creating the settings part if necessary. Any
// separator footnote references (w:footnote children) already present are
// preserved; the numbering children are regenerated.
func (d *Document) SetFootnoteProperties(np NoteProperties) {
	d.setSettingsNoteProps("footnotePr", np)
}

// ClearFootnoteProperties removes the document-level w:footnotePr element and
// reports whether it was present.
func (d *Document) ClearFootnoteProperties() bool {
	return d.removeSettingsChild("footnotePr")
}

// EndnoteProperties returns the document-level endnote numbering properties
// (w:settings/w:endnotePr) and whether the element is present.
func (d *Document) EndnoteProperties() (NoteProperties, bool) {
	return d.settingsNoteProps("endnotePr")
}

// SetEndnoteProperties sets the document-level endnote numbering properties
// (w:settings/w:endnotePr), creating the settings part if necessary. Separator
// references are preserved (see SetFootnoteProperties).
func (d *Document) SetEndnoteProperties(np NoteProperties) {
	d.setSettingsNoteProps("endnotePr", np)
}

// ClearEndnoteProperties removes the document-level w:endnotePr element and
// reports whether it was present.
func (d *Document) ClearEndnoteProperties() bool {
	return d.removeSettingsChild("endnotePr")
}

// settingsNoteProps parses the numbering fields of a settings note-properties
// child (footnotePr/endnotePr).
func (d *Document) settingsNoteProps(local string) (NoteProperties, bool) {
	if d.settings == nil {
		return NoteProperties{}, false
	}
	c := d.settings.Child(local)
	if c == nil {
		return NoteProperties{}, false
	}
	return parseNoteProps(c.RawContent), true
}

// setSettingsNoteProps writes a settings note-properties child, regenerating the
// numbering children while preserving any non-numbering children (separator
// w:footnote references) already present.
func (d *Document) setSettingsNoteProps(local string, np NoteProperties) {
	s := d.ensureSettings()
	var existing []byte
	if c := s.Child(local); c != nil {
		existing = c.RawContent
	}
	s.SetRawChild(local, buildNotePropsContent(np, existing))
	d.settingsModified = true
}

// removeSettingsChild deletes a settings child by local name and marks the
// settings part modified when it was present.
func (d *Document) removeSettingsChild(local string) bool {
	if d.settings == nil {
		return false
	}
	if d.settings.RemoveChild(local) {
		d.settingsModified = true
		return true
	}
	return false
}

// noteNumberingChildren are the local names of the numbering elements a note
// properties element carries, in schema order. Any other child (a w:footnote
// separator reference) is preserved verbatim after them.
var noteNumberingChildren = map[string]bool{
	"pos": true, "numFmt": true, "numStart": true, "numRestart": true,
}

// parseNoteProps extracts the numbering fields from a note-properties element's
// raw inner XML, matching on local names so an unbound prefix is tolerated.
func parseNoteProps(raw []byte) NoteProperties {
	np := NoteProperties{}
	if len(raw) == 0 {
		return np
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		val := ""
		for _, a := range se.Attr {
			if a.Name.Local == "val" {
				val = a.Value
			}
		}
		switch se.Name.Local {
		case "pos":
			np.Position = val
		case "numFmt":
			np.NumberFormat = val
		case "numStart":
			if n, err := strconv.Atoi(val); err == nil {
				np.NumberStart = &n
			}
		case "numRestart":
			np.Restart = val
		}
	}
	return np
}

// buildNotePropsContent renders the numbering children for a settings note
// properties element (in schema order), then appends any non-numbering children
// (separator w:footnote references) preserved from existing.
func buildNotePropsContent(np NoteProperties, existing []byte) []byte {
	var buf bytes.Buffer
	writeNoteValElem(&buf, "pos", np.Position)
	writeNoteValElem(&buf, "numFmt", np.NumberFormat)
	if np.NumberStart != nil {
		writeNoteValElem(&buf, "numStart", strconv.Itoa(*np.NumberStart))
	}
	writeNoteValElem(&buf, "numRestart", np.Restart)
	buf.Write(preserveNoteExtraChildren(existing))
	return buf.Bytes()
}

// writeNoteValElem writes an empty w:<local w:val="v"/> element when v is
// non-empty.
func writeNoteValElem(buf *bytes.Buffer, local, v string) {
	if v == "" {
		return
	}
	buf.WriteString("<w:")
	buf.WriteString(local)
	buf.WriteString(` w:val="`)
	buf.WriteString(xmlb.EscapeAttrValue(v))
	buf.WriteString(`"/>`)
}

// preserveNoteExtraChildren re-emits every child of a note-properties element
// that is not a numbering element (i.e. separator w:footnote references),
// keeping their attributes so a numbering edit does not drop the separators.
func preserveNoteExtraChildren(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	var buf bytes.Buffer
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if noteNumberingChildren[se.Name.Local] {
			continue
		}
		buf.WriteString("<w:")
		buf.WriteString(se.Name.Local)
		for _, a := range se.Attr {
			if a.Name.Local == "xmlns" || a.Name.Space == "xmlns" {
				continue
			}
			buf.WriteString(` w:`)
			buf.WriteString(a.Name.Local)
			buf.WriteString(`="`)
			buf.WriteString(xmlb.EscapeAttrValue(a.Value))
			buf.WriteString(`"`)
		}
		buf.WriteString("/>")
	}
	return buf.Bytes()
}

// RemoveDocumentVariable deletes the named document variable and reports whether
// it was present.
func (d *Document) RemoveDocumentVariable(name string) bool {
	if d.settings == nil {
		return false
	}
	vars := d.settings.DocVars()
	kept := vars[:0]
	removed := false
	for _, v := range vars {
		if v.Name == name {
			removed = true
			continue
		}
		kept = append(kept, v)
	}
	if removed {
		d.settings.SetDocVars(kept)
		d.settingsModified = true
	}
	return removed
}
