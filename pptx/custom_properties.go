package pptx

import "github.com/mgilbir/spine/opc"

// CustomProperties returns the presentation's custom (user-defined) properties
// as a name→value map, or nil when the presentation has none. Values are one of
// string, int64, float64, bool, or time.Time. The returned map is a copy;
// mutate the properties through SetCustomProperty and RemoveCustomProperty.
func (p *Presentation) CustomProperties() map[string]any {
	return p.customProps.AsMap()
}

// SetCustomProperty adds or replaces a custom document property. The value must
// be a string, int/int32/int64, float32/float64, bool, or time.Time (integers
// are stored as int64 and 32-bit floats as float64). Setting a property on a
// presentation that has none creates the docProps/custom.xml part on save.
func (p *Presentation) SetCustomProperty(name string, value any) error {
	if p.customProps == nil {
		p.customProps = &opc.CustomProperties{}
	}
	if err := p.customProps.Set(name, value); err != nil {
		return err
	}
	// customPropertiesModified (a snapshot comparison) is what makes the save
	// regenerate the part, so this call is only about recording that the deck
	// changed: the comparison latches true and cannot distinguish a second edit
	// made after a save from the first one still being outstanding.
	p.markModelEdited()
	return nil
}

// RemoveCustomProperty removes the named custom property, reporting whether it
// existed.
func (p *Presentation) RemoveCustomProperty(name string) bool {
	if p.customProps == nil {
		return false
	}
	if !p.customProps.Remove(name) {
		return false
	}
	p.markModelEdited()
	return true
}

// customPropertiesModified reports whether the custom properties were edited or
// added since the presentation was opened, so the save path regenerates
// docProps/custom.xml instead of preserving the source bytes verbatim.
func (p *Presentation) customPropertiesModified() bool {
	if p.customProps == nil {
		return false
	}
	if p.customSnapshot == nil {
		return p.customProps.Len() > 0
	}
	return !p.customProps.Equal(p.customSnapshot)
}

// ensureCustomPropsPackageRelationship adds the custom-properties relationship
// to the package root _rels/.rels when a docProps/custom.xml part is created
// this session. It injects the element into the preserved raw bytes (keeping
// unrelated relationships byte-identical) and mirrors it into the parsed set so
// the save writes the augmented bytes verbatim. It is idempotent: a repeat call
// finds the relationship already present and does nothing.
func (p *Presentation) ensureCustomPropsPackageRelationship() {
	const rootKey = "/" // RelsPathToSourcePart("/_rels/.rels") == "/"
	raw, ok := p.rawRels[rootKey]
	if !ok {
		return
	}
	aug, rel, added := opc.EnsureRelationshipInRels(raw, opc.RelTypeCustom, "docProps/custom.xml")
	if !added {
		return
	}
	p.rawRels[rootKey] = aug
	p.relationships[rootKey] = append(p.relationships[rootKey], rel)
}
