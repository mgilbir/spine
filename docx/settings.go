package docx

import (
	"encoding/xml"
	"strconv"

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
