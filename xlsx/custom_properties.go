package xlsx

import "github.com/mgilbir/spine/opc"

// CustomProperties returns the workbook's custom (user-defined) properties as a
// name→value map, or nil when the workbook has none. Values are one of string,
// int64, float64, bool, or time.Time. The returned map is a copy; mutate the
// properties through SetCustomProperty and RemoveCustomProperty.
func (w *Workbook) CustomProperties() map[string]any {
	return w.customProps.AsMap()
}

// SetCustomProperty adds or replaces a custom document property. The value must
// be a string, int/int32/int64, float32/float64, bool, or time.Time (integers
// are stored as int64 and 32-bit floats as float64). Setting a property on a
// workbook that has none creates the docProps/custom.xml part on save.
func (w *Workbook) SetCustomProperty(name string, value any) error {
	if w.customProps == nil {
		w.customProps = &opc.CustomProperties{}
	}
	return w.customProps.Set(name, value)
}

// RemoveCustomProperty removes the named custom property, reporting whether it
// existed.
func (w *Workbook) RemoveCustomProperty(name string) bool {
	if w.customProps == nil {
		return false
	}
	return w.customProps.Remove(name)
}

// customPropertiesModified reports whether the custom properties were edited or
// added since the workbook was opened, so the save path regenerates
// docProps/custom.xml instead of preserving the source bytes verbatim.
func (w *Workbook) customPropertiesModified() bool {
	if w.customProps == nil {
		return false
	}
	if w.customSnapshot == nil {
		return w.customProps.Len() > 0
	}
	return !w.customProps.Equal(w.customSnapshot)
}
