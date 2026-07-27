package dml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// This file gives CT_FillStyleList (a:fillStyleLst) and
// CT_BackgroundFillStyleList (a:bgFillStyleLst) cross-kind document-order
// fidelity, the same treatment Duotone has.
//
// Both types are a repeated EG_FillProperties choice, and their children are
// POSITIONAL: a shape's <a:fillRef idx="n"/> selects the n-th entry of the
// theme's fill style list (ECMA-376 §20.1.4.1.13/14 with §20.1.4.2.10's
// ST_StyleMatrixColumnIndex). Modeling them as six per-kind slices with no
// order capture meant a theme that interleaves kinds against field order
// re-emitted regrouped: a source [gradFill, solidFill, gradFill] came back as
// [solidFill, gradFill, gradFill], so every shape in the document resolved
// fillRef idx="1" to a different fill. The per-kind slices stay (they are the
// public shape of the model); parsing records the cross-kind order and
// marshaling replays it, falling back to grouped order for programmatically
// built lists.

// fillChoiceKind identifies an EG_FillProperties element kind.
type fillChoiceKind int

const (
	fcNoFill fillChoiceKind = iota
	fcSolidFill
	fcGradFill
	fcBlipFill
	fcPattFill
	fcGrpFill
)

// fillChoiceKindName maps a fill choice kind to its element local name.
var fillChoiceKindName = map[fillChoiceKind]string{
	fcNoFill: "noFill", fcSolidFill: "solidFill", fcGradFill: "gradFill",
	fcBlipFill: "blipFill", fcPattFill: "pattFill", fcGrpFill: "grpFill",
}

// fillChoiceNameKind is the reverse of fillChoiceKindName.
var fillChoiceNameKind = map[string]fillChoiceKind{
	"noFill": fcNoFill, "solidFill": fcSolidFill, "gradFill": fcGradFill,
	"blipFill": fcBlipFill, "pattFill": fcPattFill, "grpFill": fcGrpFill,
}

// fillChoiceRef references a fill by kind and index within its per-kind slice.
type fillChoiceRef struct {
	kind  fillChoiceKind
	index int
}

// fillListSlots gives kind-indexed access to the six per-kind slices of a fill
// style list, so the two identical content models share one implementation.
type fillListSlots struct {
	noFill    *[]*NoFillXML
	solidFill *[]*SolidFill
	gradFill  *[]*GradFill
	blipFill  *[]*BlipFillXML
	pattFill  *[]*PattFill
	grpFill   *[]*GrpFill
}

// slots returns the fill list's per-kind slices.
func (v *FillStyleLst) slots() fillListSlots {
	return fillListSlots{&v.NoFill, &v.SolidFill, &v.GradFill, &v.BlipFill, &v.PattFill, &v.GrpFill}
}

// slots returns the background fill list's per-kind slices.
func (v *BgFillStyleLst) slots() fillListSlots {
	return fillListSlots{&v.NoFill, &v.SolidFill, &v.GradFill, &v.BlipFill, &v.PattFill, &v.GrpFill}
}

// length returns the number of entries currently held for a kind.
func (s fillListSlots) length(kind fillChoiceKind) int {
	switch kind {
	case fcNoFill:
		return len(*s.noFill)
	case fcSolidFill:
		return len(*s.solidFill)
	case fcGradFill:
		return len(*s.gradFill)
	case fcBlipFill:
		return len(*s.blipFill)
	case fcPattFill:
		return len(*s.pattFill)
	case fcGrpFill:
		return len(*s.grpFill)
	}
	return 0
}

// at returns the fill referenced by ref, or nil when out of range.
func (s fillListSlots) at(ref fillChoiceRef) interface{} {
	switch ref.kind {
	case fcNoFill:
		if ref.index < len(*s.noFill) {
			return (*s.noFill)[ref.index]
		}
	case fcSolidFill:
		if ref.index < len(*s.solidFill) {
			return (*s.solidFill)[ref.index]
		}
	case fcGradFill:
		if ref.index < len(*s.gradFill) {
			return (*s.gradFill)[ref.index]
		}
	case fcBlipFill:
		if ref.index < len(*s.blipFill) {
			return (*s.blipFill)[ref.index]
		}
	case fcPattFill:
		if ref.index < len(*s.pattFill) {
			return (*s.pattFill)[ref.index]
		}
	case fcGrpFill:
		if ref.index < len(*s.grpFill) {
			return (*s.grpFill)[ref.index]
		}
	}
	return nil
}

// decodeInto decodes one element of the given kind into its slice.
func (s fillListSlots) decodeInto(d *xml.Decoder, kind fillChoiceKind, start *xml.StartElement) error {
	switch kind {
	case fcNoFill:
		v := &NoFillXML{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		*s.noFill = append(*s.noFill, v)
	case fcSolidFill:
		v := &SolidFill{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		*s.solidFill = append(*s.solidFill, v)
	case fcGradFill:
		v := &GradFill{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		*s.gradFill = append(*s.gradFill, v)
	case fcBlipFill:
		v := &BlipFillXML{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		*s.blipFill = append(*s.blipFill, v)
	case fcPattFill:
		v := &PattFill{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		*s.pattFill = append(*s.pattFill, v)
	case fcGrpFill:
		v := &GrpFill{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		*s.grpFill = append(*s.grpFill, v)
	}
	return nil
}

// unmarshalFillList decodes a repeated EG_FillProperties choice into its
// per-kind slices, recording the cross-kind document order in *order.
func unmarshalFillList(d *xml.Decoder, s fillListSlots, order *[]fillChoiceRef) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			kind, ok := fillChoiceNameKind[t.Name.Local]
			if !ok {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			*order = append(*order, fillChoiceRef{kind, s.length(kind)})
			if err := s.decodeInto(d, kind, &t); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// groupedFillRefs builds refs in grouped XSD order, the fallback when no
// document order was captured (a programmatically built list).
func groupedFillRefs(s fillListSlots) []fillChoiceRef {
	var refs []fillChoiceRef
	for _, kind := range []fillChoiceKind{fcNoFill, fcSolidFill, fcGradFill, fcBlipFill, fcPattFill, fcGrpFill} {
		for i := 0; i < s.length(kind); i++ {
			refs = append(refs, fillChoiceRef{kind, i})
		}
	}
	return refs
}

// orderedFillRefs returns the captured document order, extended with any
// entries appended after parse (which the capture never saw) so a
// programmatic addition is not silently dropped.
func orderedFillRefs(s fillListSlots, order []fillChoiceRef) []fillChoiceRef {
	if len(order) == 0 {
		return groupedFillRefs(s)
	}
	seen := make(map[fillChoiceRef]bool, len(order))
	refs := make([]fillChoiceRef, 0, len(order))
	for _, ref := range order {
		if ref.index < s.length(ref.kind) {
			refs = append(refs, ref)
			seen[ref] = true
		}
	}
	for _, ref := range groupedFillRefs(s) {
		if !seen[ref] {
			refs = append(refs, ref)
		}
	}
	return refs
}

// marshalFillList writes a fill style list, replaying the captured cross-kind
// document order.
func marshalFillList(b *xmlb.Builder, ns, localName string, s fillListSlots, order []fillChoiceRef) {
	refs := orderedFillRefs(s, order)
	if len(refs) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	for _, ref := range refs {
		if f := s.at(ref); f != nil {
			b.MarshalElement(ns, fillChoiceKindName[ref.kind], f)
		}
	}
	b.EndElement(ns, localName)
}

// encodeFillList is the encoding/xml counterpart of marshalFillList; without
// it the stdlib path would silently regroup by kind (see C482).
func encodeFillList(e *xml.Encoder, start xml.StartElement, s fillListSlots, order []fillChoiceRef) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, ref := range orderedFillRefs(s, order) {
		f := s.at(ref)
		if f == nil {
			continue
		}
		name := xml.Name{Space: NsDrawingML, Local: fillChoiceKindName[ref.kind]}
		if err := e.EncodeElement(f, xml.StartElement{Name: name}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// UnmarshalXML decodes the fill list, preserving cross-kind document order.
func (v *FillStyleLst) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalFillList(d, v.slots(), &v.fillOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying the source's
// positional fill order (a:fillRef/@idx points into it).
func (v *FillStyleLst) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalFillList(b, ns, localName, v.slots(), v.fillOrder)
}

// MarshalXML implements xml.Marshaler for the encoding/xml path.
func (v *FillStyleLst) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return encodeFillList(e, start, v.slots(), v.fillOrder)
}

// UnmarshalXML decodes the background fill list, preserving cross-kind
// document order.
func (v *BgFillStyleLst) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalFillList(d, v.slots(), &v.fillOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying the source's
// positional fill order.
func (v *BgFillStyleLst) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalFillList(b, ns, localName, v.slots(), v.fillOrder)
}

// MarshalXML implements xml.Marshaler for the encoding/xml path.
func (v *BgFillStyleLst) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return encodeFillList(e, start, v.slots(), v.fillOrder)
}
