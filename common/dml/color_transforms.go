package dml

// ColorTransformStep is one EG_ColorTransform child of a color element.
type ColorTransformStep struct {
	// Name is the element local name, such as "lumMod".
	Name string
	// Val is the transform argument; it is zero for the arg-less kinds (comp,
	// inv, gray, gamma and invGamma).
	Val Percentage
}

// Transforms returns the color's transforms in the order a save writes them,
// which is the order they apply: source order for a parsed color, field order
// for one built in code. ok is false when the color also holds children kept
// only as raw bytes (an unknown or repeated transform), whose effect a caller
// cannot evaluate from the steps. Those bytes are captured only when the color
// was parsed with its source (xmlb.UnmarshalWithSource); other decoders drop
// such children before this method can see them.
func (c *SrgbClr) Transforms() (steps []ColorTransformStep, ok bool) {
	if c == nil {
		return nil, true
	}
	return slotTransforms(c.srgbSlots(), c.xfOrder, c.xfRaws)
}

// Transforms returns the color's transforms in application order; see
// SrgbClr.Transforms.
func (c *SystemClr) Transforms() (steps []ColorTransformStep, ok bool) {
	if c == nil {
		return nil, true
	}
	return slotTransforms(c.sysSlots(), c.xfOrder, c.xfRaws)
}

// Transforms returns the color's transforms in application order; see
// SrgbClr.Transforms. Parsing drops unknown children of a:schemeClr, so ok is
// always true here.
func (s *SchemeClrTransform) Transforms() (steps []ColorTransformStep, ok bool) {
	if s == nil {
		return nil, true
	}
	step := func(kind clrTransformKind, ct *ColorTransform) {
		v := ColorTransformStep{Name: clrTransformKindName[kind]}
		if ct != nil && !clrTransformArgless[kind] {
			v.Val = ct.Val
		}
		steps = append(steps, v)
	}
	if len(s.xfOrder) > 0 {
		for _, ref := range s.xfOrder {
			if slice := *s.sliceForKind(ref.kind); ref.index < len(slice) {
				step(ref.kind, slice[ref.index])
			}
		}
		return steps, true
	}
	for _, kind := range clrTransformAllKinds {
		for _, ct := range *s.sliceForKind(kind) {
			step(kind, ct)
		}
	}
	return steps, true
}

// slotTransforms mirrors marshalClrColor's write order.
func slotTransforms(slots []clrXfSlot, order []clrTransformKind, raws [][]byte) ([]ColorTransformStep, bool) {
	var steps []ColorTransformStep
	step := func(s clrXfSlot) {
		if !s.isSet() {
			return
		}
		v := ColorTransformStep{Name: clrTransformKindName[s.kind]}
		if s.val != nil {
			v.Val = (*s.val).Val
		}
		steps = append(steps, v)
	}
	if len(order) == 0 {
		for _, s := range slots {
			step(s)
		}
		return steps, len(raws) == 0
	}
	byKind := make(map[clrTransformKind]clrXfSlot, len(slots))
	for _, s := range slots {
		byKind[s.kind] = s
	}
	for _, kind := range order {
		if kind >= clrRawKindBase {
			return nil, false
		}
		if s, found := byKind[kind]; found {
			step(s)
		}
	}
	return steps, len(raws) == 0
}
