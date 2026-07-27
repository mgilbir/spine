package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Numbering is the root element of the numbering definitions part.
//
// Parsed parts follow the house round-trip pattern: every child element is
// preserved verbatim in Raw (so numPicBullet entries, extension attributes
// like w15:restartNumberingAfterBreak, and unknown children survive
// regeneration byte-for-byte), while only the IDs of the parsed definitions
// are lifted into ParsedAbstractNumIDs/ParsedNumIDs for allocation. The typed
// AbstractNum/Num slices hold only definitions added through the API in this
// session; MarshalContent emits the raw originals first and appends the new
// definitions in schema position.
type CT_Numbering struct {
	XMLName xml.Name `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numbering"`
	// OriginalNSDecls and Ignorable preserve the root attributes of a parsed
	// part so regeneration keeps its namespace declarations.
	OriginalNSDecls []xmlb.NSDecl `xml:"-"`
	Ignorable       string        `xml:"-"`
	// Raw holds every child of a parsed part verbatim, in document order.
	Raw []*CT_RawNamedElement `xml:"-"`
	// ParsedAbstractNumIDs / ParsedNumIDs are the w:abstractNumId / w:numId
	// values of the parsed definitions kept in Raw.
	ParsedAbstractNumIDs []string `xml:"-"`
	ParsedNumIDs         []string `xml:"-"`
	// Session-added definitions.
	AbstractNum       []*CT_AbstractNum `xml:"-"`
	Num               []*CT_Num         `xml:"-"`
	NumIdMacAtCleanup *CT_DecimalNumber `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Numbering.
func (n *CT_Numbering) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.XMLName = start.Name
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			n.OriginalNSDecls = append(n.OriginalNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			n.OriginalNSDecls = append(n.OriginalNSDecls, xmlb.NSDecl{Prefix: "", URI: attr.Value})
		case attr.Name.Local == "Ignorable":
			n.Ignorable = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "abstractNum":
				for _, attr := range t.Attr {
					if attr.Name.Local == "abstractNumId" {
						n.ParsedAbstractNumIDs = append(n.ParsedAbstractNumIDs, attr.Value)
					}
				}
			case "num":
				for _, attr := range t.Attr {
					if attr.Name.Local == "numId" {
						n.ParsedNumIDs = append(n.ParsedNumIDs, attr.Value)
					}
				}
			}
			v := &CT_RawNamedElement{}
			if err := d.DecodeElement(v, &t); err != nil {
				return err
			}
			n.Raw = append(n.Raw, v)
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Numbering.
func (n *CT_Numbering) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	n.MarshalContent(b, ns)
	b.EndElement(ns, localName)
}

// MarshalContent writes the numbering children: the raw-preserved originals
// verbatim in their source document order, with session-added abstractNum/num
// definitions inserted at their schema boundary (session abstractNum elements
// after the last raw abstractNum, session num elements after the last raw num).
// Emitting the originals in document order keeps a zero-modification save
// byte-identical even when a producer interleaves the child kinds, which the
// earlier grouped emit reordered.
func (n *CT_Numbering) MarshalContent(b *xmlb.Builder, ns string) {
	lastAbstract, lastNum := -1, -1
	for i, rc := range n.Raw {
		switch rc.Local {
		case "abstractNum":
			lastAbstract = i
		case "num":
			lastNum = i
		}
	}
	abstractDone, numDone := false, false
	emitAbstract := func() {
		for _, v := range n.AbstractNum {
			b.MarshalElement(ns, "abstractNum", v)
		}
		abstractDone = true
	}
	emitNum := func() {
		for _, v := range n.Num {
			b.MarshalElement(ns, "num", v)
		}
		numDone = true
	}
	for i, rc := range n.Raw {
		// With no raw definition of a kind to anchor them, the session-added
		// ones must still precede every child the schema places after them:
		// abstractNum before num and numIdMacAtCleanup, num before
		// numIdMacAtCleanup (CT_Numbering's sequence is numPicBullet*,
		// abstractNum*, num*, numIdMacAtCleanup?). Anchoring only on num left
		// session definitions trailing a raw numIdMacAtCleanup (C506).
		if !abstractDone && lastAbstract < 0 &&
			(rc.Local == "num" || rc.Local == "numIdMacAtCleanup") {
			emitAbstract()
		}
		if !numDone && lastNum < 0 && rc.Local == "numIdMacAtCleanup" {
			emitNum()
		}
		rc.MarshalNamed(b, ns)
		if i == lastAbstract {
			emitAbstract()
		}
		if i == lastNum {
			emitNum()
		}
	}
	if !abstractDone {
		emitAbstract()
	}
	if !numDone {
		emitNum()
	}
	if n.NumIdMacAtCleanup != nil {
		b.MarshalElement(ns, "numIdMacAtCleanup", n.NumIdMacAtCleanup)
	}
}

// CT_AbstractNum represents an abstract numbering definition.
type CT_AbstractNum struct {
	AbstractNumId string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main abstractNumId,attr"`
	Nsid          *CT_LongHexNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main nsid,omitempty"`
	MultiLevelType *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main multiLevelType,omitempty"`
	Tmpl          *CT_LongHexNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tmpl,omitempty"`
	Name          *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	StyleLink     *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main styleLink,omitempty"`
	NumStyleLink  *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numStyleLink,omitempty"`
	Lvl           []*CT_Lvl  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvl"`
}

// CT_Lvl represents a single numbering level.
type CT_Lvl struct {
	Ilvl          string         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ilvl,attr"`
	Tplc          string         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tplc,attr,omitempty"`
	Tentative     string         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tentative,attr,omitempty"`
	Start         *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main start,omitempty"`
	NumFmt        *CT_NumFmt     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numFmt,omitempty"`
	LvlRestart    *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlRestart,omitempty"`
	PStyle        *CT_String     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pStyle,omitempty"`
	IsLgl         *CT_OnOff      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main isLgl,omitempty"`
	Suff          *CT_String     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suff,omitempty"`
	LvlText       *CT_LvlText    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlText,omitempty"`
	LvlPicBulletId *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlPicBulletId,omitempty"`
	LvlJc         *CT_Jc         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlJc,omitempty"`
	PPr           *CT_PPr        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPr,omitempty"`
	RPr           *CT_RPr        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
}

// CT_LvlText represents the level text template.
type CT_LvlText struct {
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Null string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main null,attr,omitempty"`
}

// CT_Num represents a numbering definition instance.
type CT_Num struct {
	NumId         string        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numId,attr"`
	AbstractNumId *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main abstractNumId"`
	LvlOverride   []*CT_NumLvl  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlOverride,omitempty"`
}

// CT_NumLvl represents a level override in a numbering instance.
type CT_NumLvl struct {
	Ilvl      string            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ilvl,attr"`
	StartOverride *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main startOverride,omitempty"`
	Lvl       *CT_Lvl           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvl,omitempty"`
}
