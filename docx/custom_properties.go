package docx

import "github.com/mgilbir/spine/opc"

// CustomProperties returns the document's custom (user-defined) properties as a
// name→value map, or nil when the document has none. Values are one of string,
// int64, float64, bool, or time.Time. The returned map is a copy; mutate the
// properties through SetCustomProperty and RemoveCustomProperty.
func (d *Document) CustomProperties() map[string]any {
	return d.customProps.AsMap()
}

// SetCustomProperty adds or replaces a custom document property. The value must
// be a string, int/int32/int64, float32/float64, bool, or time.Time (integers
// are stored as int64 and 32-bit floats as float64). Setting a property on a
// document that has none creates the docProps/custom.xml part on save.
func (d *Document) SetCustomProperty(name string, value any) error {
	if d.customProps == nil {
		d.customProps = &opc.CustomProperties{}
	}
	return d.customProps.Set(name, value)
}

// RemoveCustomProperty removes the named custom property, reporting whether it
// existed.
func (d *Document) RemoveCustomProperty(name string) bool {
	if d.customProps == nil {
		return false
	}
	return d.customProps.Remove(name)
}

// customPropertiesModified reports whether the custom properties were edited or
// added since the document was opened, so the save path regenerates
// docProps/custom.xml instead of preserving the source bytes verbatim.
func (d *Document) customPropertiesModified() bool {
	if d.customProps == nil {
		return false
	}
	if d.customSnapshot == nil {
		return d.customProps.Len() > 0
	}
	return !d.customProps.Equal(d.customSnapshot)
}
