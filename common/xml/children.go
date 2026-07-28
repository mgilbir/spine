package xml

import (
	"bytes"
	"encoding/xml"
	"reflect"
)

// This file provides per-instance child-order capture for reflection-marshaled
// structs, the child-element counterpart to the CapturedAttrs convention:
// record the exact child sequence the source wrote at unmarshal time — typed
// children by struct field, everything the model does not type (unknown
// namespaces such as w14:*, or duplicated singletons like <w:b/><w:b/>) as
// verbatim raw bytes — and replay it at marshal time. Post-parse edits stay
// authoritative: a mutated field is re-marshaled from its current value at its
// captured position, a field the caller nil-ed out is skipped, and a field set
// after parse (absent from the capture) is spliced in at its schema position —
// the declaration order of the struct's fields, which by convention mirrors the
// type's XSD content model. Captured children are never moved or re-sorted, so
// a source whose children arrived out of schema order round-trips verbatim.
//
// A struct opts in by carrying the conventional field
//
//	CapturedChildren *xml.ChildCapture `xml:"-"`
//
// and calling UnmarshalOrderedChildren from its UnmarshalXML. The reflection
// marshaler (marshalStructChildren) detects the field and replays the order.

// ChildRef is one entry of a captured child sequence. Field is the struct
// field index of a typed child, or -1 for a raw child stored in
// ChildCapture.Raw at Index. For slice fields Index is the element index.
type ChildRef struct {
	Field int
	Index int
}

// ChildCapture records the verbatim child sequence of one element instance.
type ChildCapture struct {
	Order []ChildRef
	// Raw holds children preserved as verbatim source bytes: elements the
	// struct does not model and duplicated occurrences of singleton fields.
	Raw [][]byte
}

// InsertTypedField records a single-valued typed child (fieldIndex is the
// struct field index) at its schema position in the captured order: immediately
// after the last typed child whose field index is smaller, and before every
// child that follows (including a trailing raw child such as an a:extLst). It is
// a no-op when the field is already present.
//
// marshalCapturedChildren applies the same rule to every field set after parse
// (see schemaInsertPos), so this is not needed to get schema-ordered output; it
// exists for callers that want the capture itself to reflect the insertion
// eagerly, before marshal — LstStyle.EnsureLevel records the level it allocates
// so anything reading the capture sees the final sequence.
func (cc *ChildCapture) InsertTypedField(fieldIndex int) {
	for _, ref := range cc.Order {
		if ref.Field == fieldIndex {
			return
		}
	}
	pos := schemaInsertPos(cc.Order, fieldIndex)
	cc.Order = append(cc.Order, ChildRef{})
	copy(cc.Order[pos+1:], cc.Order[pos:])
	cc.Order[pos] = ChildRef{Field: fieldIndex, Index: 0}
}

// schemaInsertPos returns where a child belonging to struct field fieldIndex
// belongs in an already-ordered child sequence: immediately after the last
// typed entry whose field index is less than or equal to fieldIndex, and
// therefore before every entry that follows — later-ranked siblings and any
// trailing raw child (an a:extLst, a w:sectPr the model types but the sequence
// puts last, an unmodeled extension element).
//
// The rank is the struct's field declaration order, which by convention in this
// module mirrors the type's XSD content model (see the child-order guard tests
// in each oxml package). "After the last smaller-or-equal" rather than "before
// the first greater" is deliberate: it never steps over a same-field entry, so
// a new element of a repeated field lands after the ones already captured, and
// it degrades gracefully on a source whose children were already out of order —
// no captured entry is ever moved.
func schemaInsertPos(order []ChildRef, fieldIndex int) int {
	pos := 0
	for i, ref := range order {
		if ref.Field >= 0 && ref.Field <= fieldIndex {
			pos = i + 1
		}
	}
	return pos
}

// childCaptureType identifies the conventional CapturedChildren field.
var childCaptureType = reflect.TypeOf((*ChildCapture)(nil))

// capturedChildrenOf returns the struct's CapturedChildren field value, or nil
// when the struct does not carry one (or it is unset).
func capturedChildrenOf(val reflect.Value) *ChildCapture {
	f, ok := val.Type().FieldByName("CapturedChildren")
	if !ok || f.Type != childCaptureType || len(f.Index) != 1 {
		return nil
	}
	cc, _ := val.Field(f.Index[0]).Interface().(*ChildCapture)
	return cc
}

// childSlot describes where a child element name decodes to.
type childSlot struct {
	field  int
	single bool
}

// UnmarshalOrderedChildren decodes the children of the element the decoder is
// positioned inside (start has been consumed) into v's element fields, and
// records their source order in v's CapturedChildren field. v must be a
// pointer to a struct whose element children are pointer or slice fields with
// namespaced xml tags; the struct's attributes are not touched (capture them
// separately with CaptureAttrs before calling this).
//
// Children the struct does not model — and second occurrences of singleton
// fields — are preserved as verbatim raw bytes when the decoder has a
// registered source (UnmarshalWithSource); without one they are dropped, and
// nothing records that they were there. Two consequences are worth stating
// because they are *not* what encoding/xml would do:
//
//   - A duplicated singleton is first-wins here (the first occurrence decodes
//     into the field, later ones are kept as raw children so the source
//     sequence replays); encoding/xml is last-wins and keeps no trace of the
//     earlier ones. A struct whose schema permits repeats should model the
//     child as a slice rather than rely on either behaviour.
//   - Character data between children is captured verbatim (whitespace and,
//     with a source registered, non-whitespace text), so an element with mixed
//     content keeps its text. Without a registered source that text is lost —
//     the same inert-capture failure mode as unknown children. Structs with a
//     `,chardata` field must not use this decoder: it never populates one.
func UnmarshalOrderedChildren(d *xml.Decoder, v interface{}) error {
	val := reflect.ValueOf(v).Elem()
	typ := val.Type()

	slots := make(map[xml.Name]childSlot)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() || f.Type == xmlNameType {
			continue
		}
		info := parseTag(f.Tag.Get("xml"), f.Name)
		if info.skip || info.attr || info.chardata || info.innerxml || info.isAny {
			continue
		}
		single := f.Type.Kind() != reflect.Slice || f.Type.Elem().Kind() == reflect.Uint8
		slots[xml.Name{Space: info.ns, Local: info.name}] = childSlot{field: i, single: single}
	}

	var src []byte
	if s, ok := decoderSources.Load(d); ok {
		src = s.([]byte)
	}

	cap := &ChildCapture{}
	seen := make(map[int]bool)
	for {
		pre := d.InputOffset()
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			slot, ok := slots[t.Name]
			if !ok {
				// A tag without a namespace matches any namespace, mirroring
				// encoding/xml (xlsx models rely on the parent default ns).
				slot, ok = slots[xml.Name{Local: t.Name.Local}]
			}
			if ok && (!slot.single || !seen[slot.field]) {
				fv := val.Field(slot.field)
				if slot.single {
					if err := decodeChildInto(d, &t, fv); err != nil {
						return err
					}
					seen[slot.field] = true
					cap.Order = append(cap.Order, ChildRef{Field: slot.field, Index: 0})
					continue
				}
				et := fv.Type().Elem()
				nv := reflect.New(et)
				if et.Kind() == reflect.Pointer {
					nv.Elem().Set(reflect.New(et.Elem()))
					if err := d.DecodeElement(nv.Elem().Interface(), &t); err != nil {
						return err
					}
				} else if err := d.DecodeElement(nv.Interface(), &t); err != nil {
					return err
				}
				cap.Order = append(cap.Order, ChildRef{Field: slot.field, Index: fv.Len()})
				fv.Set(reflect.Append(fv, nv.Elem()))
				continue
			}
			// Unknown child or duplicated singleton: keep the source bytes.
			if err := d.Skip(); err != nil {
				return err
			}
			if src != nil {
				post := d.InputOffset()
				if pre >= 0 && post <= int64(len(src)) && pre < post {
					cap.Order = append(cap.Order, ChildRef{Field: -1, Index: len(cap.Raw)})
					// Clone: retaining a sub-slice of src would pin the whole
					// part in memory for the model's lifetime (see C282).
					cap.Raw = append(cap.Raw, bytes.Clone(src[pre:post]))
				}
			}
		case xml.Comment, xml.ProcInst:
			// Comments and processing instructions between children are part of
			// the source the capture kit exists to preserve; without this they
			// fall through and are silently dropped even on a zero-mod save.
			// Keep them as verbatim raw children in document order.
			if src == nil {
				continue
			}
			post := d.InputOffset()
			if pre < 0 || post > int64(len(src)) || pre >= post {
				continue
			}
			cap.Order = append(cap.Order, ChildRef{Field: -1, Index: len(cap.Raw)})
			cap.Raw = append(cap.Raw, bytes.Clone(src[pre:post]))
		case xml.CharData:
			// Character data between children is captured verbatim from the
			// raw source — the token itself is EOL-normalized by the decoder —
			// and replayed as a raw child. This covers both the whitespace of
			// pretty-printed property bags and genuine mixed content: dropping
			// non-whitespace text here would silently delete user content the
			// moment a struct that opted into ordered capture gained a
			// text-bearing child, with no guard and no error.
			if src == nil {
				continue
			}
			post := d.InputOffset()
			if pre < 0 || post > int64(len(src)) || pre >= post {
				continue
			}
			raw := src[pre:post]
			cap.Order = append(cap.Order, ChildRef{Field: -1, Index: len(cap.Raw)})
			// Clone: retaining a sub-slice of src would pin the whole part in
			// memory for the model's lifetime (see C282).
			cap.Raw = append(cap.Raw, bytes.Clone(raw))
		case xml.EndElement:
			if len(cap.Order) > 0 {
				setCapturedChildren(val, cap)
			}
			return nil
		}
	}
}

// decodeChildInto decodes one element into a singleton field (pointer or
// value).
func decodeChildInto(d *xml.Decoder, t *xml.StartElement, fv reflect.Value) error {
	if fv.Kind() == reflect.Pointer {
		nv := reflect.New(fv.Type().Elem())
		if err := d.DecodeElement(nv.Interface(), t); err != nil {
			return err
		}
		fv.Set(nv)
		return nil
	}
	return d.DecodeElement(fv.Addr().Interface(), t)
}

// setCapturedChildren stores the capture in the struct's conventional field,
// when present.
func setCapturedChildren(val reflect.Value, cc *ChildCapture) {
	f, ok := val.Type().FieldByName("CapturedChildren")
	if !ok || f.Type != childCaptureType || len(f.Index) != 1 {
		return
	}
	val.Field(f.Index[0]).Set(reflect.ValueOf(cc))
}

// marshalCapturedChildren replays a captured child sequence: typed entries are
// re-marshaled from the field's current value (post-parse edits win), raw
// entries are written verbatim, and fields the capture never saw (set after
// parse) are spliced in at their schema position by planCapturedChildren.
func (b *Builder) marshalCapturedChildren(parentNS string, val reflect.Value, cc *ChildCapture) {
	typ := val.Type()
	emittedSingle := make(map[int]bool)

	for _, ref := range planCapturedChildren(typ, val, cc) {
		if ref.Field < 0 {
			if ref.Index < len(cc.Raw) {
				b.WriteRaw(cc.Raw[ref.Index])
			}
			continue
		}
		if ref.Field >= typ.NumField() {
			continue
		}
		field := typ.Field(ref.Field)
		info := parseTag(field.Tag.Get("xml"), field.Name)
		if info.skip || info.attr || info.chardata || info.innerxml || info.isAny {
			continue
		}
		fv := val.Field(ref.Field)
		elemNS := info.ns
		if elemNS == "" {
			elemNS = parentNS
		}
		if fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() != reflect.Uint8 {
			if ref.Index < fv.Len() {
				b.marshalReflect(elemNS, info.name, fv.Index(ref.Index))
			}
			continue
		}
		if emittedSingle[ref.Field] {
			continue
		}
		emittedSingle[ref.Field] = true
		if fv.Kind() == reflect.Pointer && fv.IsNil() {
			continue // removed after parse
		}
		if info.omitempty && isZeroValue(fv) {
			continue
		}
		b.marshalReflect(elemNS, info.name, fv)
	}
}

// planCapturedChildren returns the child sequence to emit for one instance: the
// captured order, with every child set after parse spliced in at its schema
// position (schemaInsertPos) rather than appended after everything.
//
// Appending was C329: a parsed w:pPr carrying a w:sectPr re-emitted a
// post-parse SetAlignment as <w:sectPr/><w:jc/>, which CT_PPr's xsd:sequence
// forbids; likewise a w:b added to a w:rPr that already carried a w:rPrChange,
// which the schema pins last.
//
// The captured entries themselves are never moved or re-sorted. A document
// whose children arrived out of schema order — real producers emit them, and
// Word tolerates it — replays exactly as it came in, so a zero-modification
// save stays byte-identical; the ordering rule applies only to what this
// library adds. The returned slice aliases cc.Order until something is
// inserted, and the capture itself is left untouched.
func planCapturedChildren(typ reflect.Type, val reflect.Value, cc *ChildCapture) []ChildRef {
	captured := make(map[ChildRef]bool, len(cc.Order))
	capturedField := make(map[int]bool, len(cc.Order))
	for _, ref := range cc.Order {
		if ref.Field < 0 {
			continue
		}
		captured[ref] = true
		capturedField[ref.Field] = true
	}

	plan := cc.Order
	cloned := false
	insert := func(ref ChildRef) {
		pos := schemaInsertPos(plan, ref.Field)
		if !cloned {
			plan = append([]ChildRef(nil), plan...)
			cloned = true
		}
		plan = append(plan, ChildRef{})
		copy(plan[pos+1:], plan[pos:])
		plan[pos] = ref
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() || field.Type == xmlNameType {
			continue
		}
		info := parseTag(field.Tag.Get("xml"), field.Name)
		if info.skip || info.attr || info.chardata || info.innerxml || info.isAny {
			continue
		}
		fv := val.Field(i)
		if info.omitempty && isZeroValue(fv) {
			continue
		}
		if fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() != reflect.Uint8 {
			for j := 0; j < fv.Len(); j++ {
				if !captured[ChildRef{Field: i, Index: j}] {
					insert(ChildRef{Field: i, Index: j})
				}
			}
			continue
		}
		if capturedField[i] {
			continue
		}
		if fv.Kind() == reflect.Pointer && fv.IsNil() {
			continue
		}
		insert(ChildRef{Field: i})
	}
	return plan
}

// hasCapturedRawChildren reports whether the capture will emit raw children,
// so an element whose typed fields are all empty still opens and closes.
func hasCapturedRawChildren(val reflect.Value) bool {
	cc := capturedChildrenOf(val)
	if cc == nil {
		return false
	}
	for _, ref := range cc.Order {
		if ref.Field < 0 && ref.Index < len(cc.Raw) {
			return true
		}
	}
	return false
}
