package oxml

import (
	"encoding/xml"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Scenarios is the worksheet <scenarios> element: the what-if Scenario
// Manager for a sheet. Existing scenarios round-trip byte-for-byte through Raw
// (the verbatim reconstruction captured at parse time); authoring one sets
// Dirty, which switches marshaling to the typed model.
type CT_Scenarios struct {
	Current  *uint32
	Show     *uint32
	SqRef    string
	Scenario []CT_Scenario
	// Raw is the verbatim reconstruction of the source element, emitted on a
	// no-op round trip. nil for a scenarios element authored from scratch.
	Raw []byte
	// Dirty is set when the typed model was mutated (AddScenario); it forces the
	// typed marshal instead of re-emitting Raw.
	Dirty bool
}

// CT_Scenario is a single named scenario: a set of substitute values for a
// group of changing (input) cells.
type CT_Scenario struct {
	Name       string
	Locked     *bool
	Hidden     *bool
	Count      *uint32
	User       string
	Comment    string
	InputCells []CT_InputCells
}

// CT_InputCells is one changing cell within a scenario and the value the
// scenario substitutes for it.
type CT_InputCells struct {
	R        string
	Deleted  *bool
	Undone   *bool
	Val      string
	NumFmtId *uint32
}

// scenariosXML mirrors the on-disk shape for tolerant unmarshaling. Local names
// only (no namespace qualifier): the reconstructed Raw carries no namespace
// declarations, so every element and attribute arrives in the empty namespace.
type scenariosXML struct {
	Current  *uint32       `xml:"current,attr"`
	Show     *uint32       `xml:"show,attr"`
	SqRef    string        `xml:"sqref,attr"`
	Scenario []scenarioXML `xml:"scenario"`
}

type scenarioXML struct {
	Name       string          `xml:"name,attr"`
	Locked     *bool           `xml:"locked,attr"`
	Hidden     *bool           `xml:"hidden,attr"`
	Count      *uint32         `xml:"count,attr"`
	User       string          `xml:"user,attr"`
	Comment    string          `xml:"comment,attr"`
	InputCells []inputCellsXML `xml:"inputCells"`
}

type inputCellsXML struct {
	R        string  `xml:"r,attr"`
	Deleted  *bool   `xml:"deleted,attr"`
	Undone   *bool   `xml:"undone,attr"`
	Val      string  `xml:"val,attr"`
	NumFmtId *uint32 `xml:"numFmtId,attr"`
}

// parse populates the typed model from the parsed start element and its raw
// inner content. It is best-effort: a scenarios element that fails to reparse
// still round-trips via Raw.
func (sc *CT_Scenarios) parse(start xml.StartElement, inner []byte) {
	// Reconstruct the full element bytes and decode. Reusing the same bare
	// reconstruction the marshaler emits keeps parse and emit symmetric.
	//
	// encoding/xml, not xmlb.Unmarshal, and deliberately: the part-level entry
	// point additionally requires the input to bind every prefix it uses, which
	// is right for a part read off the package and wrong here. This is one
	// element lifted out of a larger document, and the declarations its prefixes
	// resolve against stayed behind on that document's root.
	full := encodeUnknownElement(start, inner, nil)
	var x scenariosXML
	if err := xml.Unmarshal(full, &x); err != nil {
		return
	}
	sc.Current = x.Current
	sc.Show = x.Show
	sc.SqRef = x.SqRef
	for _, s := range x.Scenario {
		cs := CT_Scenario{
			Name:    s.Name,
			Locked:  s.Locked,
			Hidden:  s.Hidden,
			Count:   s.Count,
			User:    s.User,
			Comment: s.Comment,
		}
		for _, ic := range s.InputCells {
			cs.InputCells = append(cs.InputCells, CT_InputCells(ic))
		}
		sc.Scenario = append(sc.Scenario, cs)
	}
}

// MarshalToBuilder emits the scenarios element from the typed model. It is only
// used for an authored/modified element; an untouched one is re-emitted from Raw
// by the worksheet marshaler.
func (sc *CT_Scenarios) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if sc.Current != nil {
		attrs = append(attrs, xmlb.UintAttr("current", *sc.Current))
	}
	if sc.Show != nil {
		attrs = append(attrs, xmlb.UintAttr("show", *sc.Show))
	}
	if sc.SqRef != "" {
		attrs = append(attrs, xmlb.StrAttr("sqref", sc.SqRef))
	}
	if len(sc.Scenario) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	for i := range sc.Scenario {
		sc.Scenario[i].marshal(b, ns)
	}
	b.EndElement(ns, localName)
}

func (s *CT_Scenario) marshal(b *xmlb.Builder, ns string) {
	var attrs []xmlb.Attr
	attrs = append(attrs, xmlb.StrAttr("name", s.Name))
	if s.Locked != nil {
		attrs = append(attrs, xmlb.BoolAttr("locked", *s.Locked))
	}
	if s.Hidden != nil {
		attrs = append(attrs, xmlb.BoolAttr("hidden", *s.Hidden))
	}
	count := uint32(len(s.InputCells))
	if s.Count != nil {
		count = *s.Count
	}
	attrs = append(attrs, xmlb.Attr{Name: "count", Value: strconv.FormatUint(uint64(count), 10)})
	if s.User != "" {
		// user is optional; emitting user="" on a scenario whose source had no
		// author invented an attribute the producer never wrote (C556).
		attrs = append(attrs, xmlb.StrAttr("user", s.User))
	}
	if s.Comment != "" {
		attrs = append(attrs, xmlb.StrAttr("comment", s.Comment))
	}
	if len(s.InputCells) == 0 {
		b.EmptyElement(ns, "scenario", attrs...)
		return
	}
	b.StartElement(ns, "scenario", attrs...)
	for i := range s.InputCells {
		s.InputCells[i].marshal(b, ns)
	}
	b.EndElement(ns, "scenario")
}

func (ic *CT_InputCells) marshal(b *xmlb.Builder, ns string) {
	var attrs []xmlb.Attr
	attrs = append(attrs, xmlb.StrAttr("r", ic.R))
	if ic.Deleted != nil {
		attrs = append(attrs, xmlb.BoolAttr("deleted", *ic.Deleted))
	}
	if ic.Undone != nil {
		attrs = append(attrs, xmlb.BoolAttr("undone", *ic.Undone))
	}
	attrs = append(attrs, xmlb.StrAttr("val", ic.Val))
	if ic.NumFmtId != nil {
		attrs = append(attrs, xmlb.UintAttr("numFmtId", *ic.NumFmtId))
	}
	b.EmptyElement(ns, "inputCells", attrs...)
}
