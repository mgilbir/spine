package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_OleObjects is the worksheet <oleObjects> element: the list of embedded OLE
// objects anchored on a sheet. Existing elements round-trip byte-for-byte via
// Raw; authoring one (AddOLEObject) sets Dirty, switching to the typed marshal.
type CT_OleObjects struct {
	OleObject []CT_OleObject
	// Raw is the verbatim reconstruction of a parsed element, emitted on a no-op
	// round trip. nil for an element authored from scratch.
	Raw []byte
	// Dirty forces the typed marshal instead of re-emitting Raw.
	Dirty bool
}

// CT_OleObject is one embedded OLE object reference. Authored objects carry the
// modern mc:AlternateContent-free minimal form: progId, shapeId and the r:id of
// the embedded object part.
type CT_OleObject struct {
	ProgID  string
	ShapeID uint32
	RID     string
}

// oleObjectsXML mirrors the on-disk shape for tolerant unmarshaling; the
// reconstructed Raw carries no namespace declarations, so names arrive in the
// empty namespace. The r:id lives in the relationships namespace, matched by
// local name here.
type oleObjectsXML struct {
	OleObject []struct {
		ProgID  string `xml:"progId,attr"`
		ShapeID uint32 `xml:"shapeId,attr"`
		RID     string `xml:"id,attr"`
	} `xml:"oleObject"`
}

// parse best-effort populates the typed model from the parsed start element and
// its raw inner content. A parse failure still round-trips via Raw.
func (o *CT_OleObjects) parse(start xml.StartElement, inner []byte) {
	// encoding/xml rather than the part-level xmlb.Unmarshal: this is one
	// element lifted out of a larger document, so the declarations its prefixes
	// resolve against are not in these bytes (see CT_Scenarios.parse).
	full := encodeUnknownElement(start, inner, nil)
	var x oleObjectsXML
	//xmlguard:lenient one element reconstructed from a captured start tag and its inner content; its prefixes resolve against a root that stayed behind
	if err := xml.Unmarshal(full, &x); err != nil {
		return
	}
	for _, e := range x.OleObject {
		o.OleObject = append(o.OleObject, CT_OleObject{ProgID: e.ProgID, ShapeID: e.ShapeID, RID: e.RID})
	}
}

// MarshalToBuilder emits the oleObjects element from the typed model (authored
// objects only; an untouched element is re-emitted from Raw by the worksheet
// marshaler).
func (o *CT_OleObjects) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if len(o.OleObject) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	for i := range o.OleObject {
		oo := &o.OleObject[i]
		attrs := []xmlb.Attr{xmlb.StrAttr("progId", oo.ProgID)}
		attrs = append(attrs, xmlb.UintAttr("shapeId", oo.ShapeID))
		if oo.RID != "" {
			attrs = append(attrs, xmlb.RelAttr("id", oo.RID))
		}
		b.EmptyElement(ns, "oleObject", attrs...)
	}
	b.EndElement(ns, localName)
}
