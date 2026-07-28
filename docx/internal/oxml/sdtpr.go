package oxml

import (
	"bytes"
	"encoding/xml"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// NsW14 is the Word 2010 wordml extension namespace. The structured-document-tag
// checkbox control (w14:checkbox) lives here rather than in the main WML
// namespace.
const NsW14 = "http://schemas.microsoft.com/office/word/2010/wordml"

// sdtPrChildKind identifies a typed w:sdtPr child slot. Everything the model
// does not type (w:rPr, w:dataBinding, w:placeholder, w:showingPlcHdr, ...) is
// captured verbatim in the Raw slice under sdtPrChildRaw.
type sdtPrChildKind uint8

const (
	sdtPrChildAlias sdtPrChildKind = iota
	sdtPrChildTag
	sdtPrChildID
	sdtPrChildLock
	sdtPrChildControl
	sdtPrChildRaw
)

// sdtPrChildRef records one w:sdtPr child in source order. index is meaningful
// only for sdtPrChildRaw (the singleton typed slots always occupy index 0).
type sdtPrChildRef struct {
	kind  sdtPrChildKind
	index int
}

// sdtPrElementOrder maps each w:sdtPr child to its position in CT_SdtPr's
// xsd:sequence (ECMA-376 §17.5.2.38): rPr, alias, tag, id, lock, placeholder,
// temporary, showingPlcHdr, dataBinding, label, tabIndex, then the control-type
// choice. It ranks both the typed slots and the children preserved raw, so a
// property set after parse is inserted at a schema-valid position among them
// rather than appended after every captured child (C329) — SetAlias on a
// control parsed with a w:id and a w:docPartObj emitted the w:alias last, which
// the sequence forbids.
var sdtPrElementOrder = map[string]int{
	"rPr": 0, "alias": 1, "tag": 2, "id": 3, "lock": 4, "placeholder": 5,
	"temporary": 6, "showingPlcHdr": 7, "dataBinding": 8, "label": 9,
	"tabIndex": 10,
}

// sdtPrControlRank is the rank of the control-type choice, the last member of
// CT_SdtPr's sequence.
const sdtPrControlRank = 11

// sdtPrKindRank returns the schema rank of a typed child slot.
func sdtPrKindRank(kind sdtPrChildKind) int {
	switch kind {
	case sdtPrChildAlias:
		return sdtPrElementOrder["alias"]
	case sdtPrChildTag:
		return sdtPrElementOrder["tag"]
	case sdtPrChildID:
		return sdtPrElementOrder["id"]
	case sdtPrChildLock:
		return sdtPrElementOrder["lock"]
	default:
		return sdtPrControlRank
	}
}

// rankOf returns the schema rank of a recorded child, and whether it has one.
// Raw children the schema does not define (extension elements) are unranked and
// do not move an insertion point.
func (pr *CT_SdtPr) rankOf(ref sdtPrChildRef) (int, bool) {
	if ref.kind != sdtPrChildRaw {
		return sdtPrKindRank(ref.kind), true
	}
	if ref.index < len(pr.Raw) {
		if r, ok := sdtPrElementOrder[pr.Raw[ref.index].Local]; ok {
			return r, true
		}
	}
	return 0, false
}

// insertChild records a child added after parse at its schema position:
// immediately after the last recorded child that outranks it. Nothing already
// recorded moves, so an unmodified control still round-trips verbatim.
func (pr *CT_SdtPr) insertChild(ref sdtPrChildRef) {
	rank := sdtPrKindRank(ref.kind)
	pos := 0
	for i, existing := range pr.childOrder {
		if r, ok := pr.rankOf(existing); ok && r <= rank {
			pos = i + 1
		}
	}
	pr.childOrder = append(pr.childOrder, sdtPrChildRef{})
	copy(pr.childOrder[pos+1:], pr.childOrder[pos:])
	pr.childOrder[pos] = ref
}

// CT_SdtPr represents structured document tag properties (w:sdtPr). The tag,
// alias, id and lock properties plus the control-type child are typed for API
// access; all other children (run properties, data bindings, placeholders and
// any construct the model does not recognize) are captured verbatim so an
// unmodified content control round-trips byte-for-byte.
type CT_SdtPr struct {
	Alias   *CT_SdtValElem
	Tag     *CT_SdtValElem
	ID      *CT_SdtValElem
	Lock    *CT_SdtValElem
	Control *CT_SdtControl
	// Raw preserves w:sdtPr children the model does not type verbatim; they
	// carry data bindings and placeholders that must not be dropped on save.
	Raw []*CT_RawNamedElement
	// CapturedEmptyTag records how a childless w:sdtPr / w:sdtEndPr was written
	// (<w:sdtEndPr/> vs <w:sdtEndPr></w:sdtEndPr>).
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
	childOrder       []sdtPrChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_SdtPr, walking the child
// choice in source order.
func (pr *CT_SdtPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	pr.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := pr.unmarshalChild(d, t); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// unmarshalChild routes a single w:sdtPr child element to its typed slot or,
// failing that, to the verbatim Raw capture.
func (pr *CT_SdtPr) unmarshalChild(d *xml.Decoder, t xml.StartElement) error {
	if t.Name.Space == NsWml {
		switch t.Name.Local {
		case "alias":
			if pr.Alias == nil {
				return pr.decodeVal(d, t, &pr.Alias, sdtPrChildAlias)
			}
		case "tag":
			if pr.Tag == nil {
				return pr.decodeVal(d, t, &pr.Tag, sdtPrChildTag)
			}
		case "id":
			if pr.ID == nil {
				return pr.decodeVal(d, t, &pr.ID, sdtPrChildID)
			}
		case "lock":
			if pr.Lock == nil {
				return pr.decodeVal(d, t, &pr.Lock, sdtPrChildLock)
			}
		}
	}
	if pr.Control == nil && isSdtControlElem(t.Name) {
		c := &CT_SdtControl{}
		if err := c.unmarshal(d, t); err != nil {
			return err
		}
		pr.Control = c
		pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildControl, 0})
		return nil
	}
	rn := &CT_RawNamedElement{}
	if err := rn.UnmarshalXML(d, t); err != nil {
		return err
	}
	pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildRaw, len(pr.Raw)})
	pr.Raw = append(pr.Raw, rn)
	return nil
}

// decodeVal decodes a simple w:val-bearing property into slot and records it.
func (pr *CT_SdtPr) decodeVal(d *xml.Decoder, t xml.StartElement, slot **CT_SdtValElem, kind sdtPrChildKind) error {
	v := &CT_SdtValElem{}
	if err := v.unmarshal(d, t); err != nil {
		return err
	}
	*slot = v
	pr.childOrder = append(pr.childOrder, sdtPrChildRef{kind, 0})
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SdtPr.
func (pr *CT_SdtPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if pr.isEmpty() {
		b.EmptyElementStyled(pr.CapturedEmptyTag, ns, localName)
		return
	}
	b.StartElement(ns, localName)
	if len(pr.childOrder) > 0 {
		for _, ref := range pr.childOrder {
			pr.marshalChild(b, ns, ref)
		}
	} else {
		// Programmatic construction: emit in canonical Word order.
		if pr.Alias != nil {
			pr.Alias.marshalTo(b, ns, "alias")
		}
		if pr.Tag != nil {
			pr.Tag.marshalTo(b, ns, "tag")
		}
		if pr.ID != nil {
			pr.ID.marshalTo(b, ns, "id")
		}
		if pr.Lock != nil {
			pr.Lock.marshalTo(b, ns, "lock")
		}
		for _, rn := range pr.Raw {
			rn.MarshalNamed(b, ns)
		}
		if pr.Control != nil {
			pr.Control.marshalTo(b)
		}
	}
	b.EndElement(ns, localName)
}

// isEmpty reports whether the properties carry no children at all.
func (pr *CT_SdtPr) isEmpty() bool {
	return pr.Alias == nil && pr.Tag == nil && pr.ID == nil && pr.Lock == nil &&
		pr.Control == nil && len(pr.Raw) == 0
}

// marshalChild writes one child in its recorded source position.
func (pr *CT_SdtPr) marshalChild(b *xmlb.Builder, ns string, ref sdtPrChildRef) {
	switch ref.kind {
	case sdtPrChildAlias:
		if pr.Alias != nil {
			pr.Alias.marshalTo(b, ns, "alias")
		}
	case sdtPrChildTag:
		if pr.Tag != nil {
			pr.Tag.marshalTo(b, ns, "tag")
		}
	case sdtPrChildID:
		if pr.ID != nil {
			pr.ID.marshalTo(b, ns, "id")
		}
	case sdtPrChildLock:
		if pr.Lock != nil {
			pr.Lock.marshalTo(b, ns, "lock")
		}
	case sdtPrChildControl:
		if pr.Control != nil {
			pr.Control.marshalTo(b)
		}
	case sdtPrChildRaw:
		if ref.index < len(pr.Raw) {
			pr.Raw[ref.index].MarshalNamed(b, ns)
		}
	}
}

// TagValue returns the w:tag value, or "" when unset.
func (pr *CT_SdtPr) TagValue() string {
	if pr == nil || pr.Tag == nil {
		return ""
	}
	return pr.Tag.Val
}

// AliasValue returns the w:alias value, or "" when unset.
func (pr *CT_SdtPr) AliasValue() string {
	if pr == nil || pr.Alias == nil {
		return ""
	}
	return pr.Alias.Val
}

// IDValue returns the w:id value, or "" when unset.
func (pr *CT_SdtPr) IDValue() string {
	if pr == nil || pr.ID == nil {
		return ""
	}
	return pr.ID.Val
}

// SetTag sets (creating if necessary) the w:tag value. Passing "" removes it.
func (pr *CT_SdtPr) SetTag(v string) { pr.setVal(&pr.Tag, sdtPrChildTag, v) }

// SetAlias sets (creating if necessary) the w:alias value. Passing "" removes it.
func (pr *CT_SdtPr) SetAlias(v string) { pr.setVal(&pr.Alias, sdtPrChildAlias, v) }

// SetID sets (creating if necessary) the w:id value. Passing "" removes it.
func (pr *CT_SdtPr) SetID(v string) { pr.setVal(&pr.ID, sdtPrChildID, v) }

// SetControl sets (creating if necessary) the control-type child, keeping child
// order coherent so a control added programmatically still marshals.
func (pr *CT_SdtPr) SetControl(local, space string) {
	if pr.Control != nil {
		pr.Control.Local = local
		pr.Control.Space = space
		return
	}
	// backfillChildOrder must run while pr.Control is still nil so it does not
	// record the slot we are about to insert explicitly (double emission).
	pr.backfillChildOrder()
	pr.Control = &CT_SdtControl{Local: local, Space: space}
	pr.insertChild(sdtPrChildRef{sdtPrChildControl, 0})
}

// setVal sets or clears a simple w:val property, keeping child order coherent so
// a newly created property still marshals on a control parsed from a file.
func (pr *CT_SdtPr) setVal(slot **CT_SdtValElem, kind sdtPrChildKind, v string) {
	if v == "" {
		if *slot != nil {
			*slot = nil
			pr.dropChild(kind)
		}
		return
	}
	if *slot != nil {
		// Preserve position; the captured attrs replay keeps the modeled value
		// authoritative so the new value wins.
		(*slot).Val = v
		return
	}
	// backfillChildOrder must run while the slot is still nil so it does not
	// record the child we are about to insert explicitly (double emission).
	pr.backfillChildOrder()
	*slot = &CT_SdtValElem{Val: v}
	pr.insertChild(sdtPrChildRef{kind, 0})
}

// dropChild removes a typed child from childOrder after its slot was cleared.
func (pr *CT_SdtPr) dropChild(kind sdtPrChildKind) {
	if len(pr.childOrder) == 0 {
		return
	}
	out := pr.childOrder[:0]
	for _, ref := range pr.childOrder {
		if ref.kind == kind {
			continue
		}
		out = append(out, ref)
	}
	pr.childOrder = out
}

// backfillChildOrder records existing children in childOrder before the first
// tracked mutation, so appending a property does not flip marshaling to the
// childOrder path and drop the untracked children.
func (pr *CT_SdtPr) backfillChildOrder() {
	if len(pr.childOrder) > 0 {
		return
	}
	if pr.Alias != nil {
		pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildAlias, 0})
	}
	if pr.Tag != nil {
		pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildTag, 0})
	}
	if pr.ID != nil {
		pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildID, 0})
	}
	if pr.Lock != nil {
		pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildLock, 0})
	}
	for i := range pr.Raw {
		pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildRaw, i})
	}
	if pr.Control != nil {
		pr.childOrder = append(pr.childOrder, sdtPrChildRef{sdtPrChildControl, 0})
	}
}

// CT_SdtValElem is a simple w:val-bearing empty property element (w:alias,
// w:tag, w:id, w:lock). The verbatim attribute list and empty-tag style are
// captured so an unmodified property round-trips byte-for-byte.
type CT_SdtValElem struct {
	Val              string
	CapturedAttrs    []xmlb.RootAttr    `xml:"-"`
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

func (v *CT_SdtValElem) unmarshal(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, a := range start.Attr {
		if a.Name.Local == "val" && (a.Name.Space == NsWml || a.Name.Space == "") {
			v.Val = a.Value
		}
	}
	return d.Skip()
}

func (v *CT_SdtValElem) marshalTo(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{{Namespace: NsWml, Name: "val", Value: v.Val}}
	if v.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(v.CapturedAttrs, attrs)
	}
	b.EmptyElementStyled(v.CapturedEmptyTag, ns, localName, attrs...)
}

// CT_SdtControl is the control-type child of w:sdtPr (w:text, w:richText,
// w:dropDownList, w:comboBox, w14:checkbox, w:date, w:picture). It is preserved
// verbatim for byte-identical round-trip while exposing typed views (control
// kind, drop-down/combo options, date format, checkbox state) for the API.
type CT_SdtControl struct {
	// Local is the control element's local name (e.g. "dropDownList"); it
	// doubles as the public control-kind discriminator.
	Local string
	// Space is the control element's namespace URI (NsWml, or NsW14 for the
	// checkbox).
	Space string
	raw   CT_RawElement
}

func (c *CT_SdtControl) unmarshal(d *xml.Decoder, start xml.StartElement) error {
	c.Local = start.Name.Local
	c.Space = start.Name.Space
	return c.raw.UnmarshalXML(d, start)
}

func (c *CT_SdtControl) marshalTo(b *xmlb.Builder) {
	ns := c.Space
	if ns == "" {
		ns = NsWml
	}
	c.raw.MarshalToBuilder(b, ns, c.Local)
}

// SdtListItem is one w:listItem of a drop-down or combo-box control.
type SdtListItem struct {
	DisplayText string
	Value       string
}

// ListItems parses the drop-down / combo-box options from the preserved raw
// content. It returns nil for other control kinds.
func (c *CT_SdtControl) ListItems() []SdtListItem {
	if c == nil {
		return nil
	}
	var items []SdtListItem
	c.forEachChild("listItem", func(attrs []xml.Attr) {
		var it SdtListItem
		for _, a := range attrs {
			switch a.Name.Local {
			case "displayText":
				it.DisplayText = a.Value
			case "value":
				it.Value = a.Value
			}
		}
		items = append(items, it)
	})
	return items
}

// DateFormat parses the w:dateFormat value of a date control, or "" when absent.
func (c *CT_SdtControl) DateFormat() string {
	return c.firstChildVal("dateFormat")
}

// Checked reports the w14:checkbox checked state (and whether it was present).
func (c *CT_SdtControl) Checked() (checked, ok bool) {
	v := c.firstChildVal("checked")
	if v == "" {
		return false, false
	}
	return v == "1" || v == "true", true
}

// firstChildVal returns the w:val attribute of the first raw child with the
// given local name.
func (c *CT_SdtControl) firstChildVal(local string) string {
	var val string
	found := false
	c.forEachChild(local, func(attrs []xml.Attr) {
		if found {
			return
		}
		for _, a := range attrs {
			if a.Name.Local == "val" {
				val = a.Value
			}
		}
		found = true
	})
	return val
}

// forEachChild invokes fn with the attributes of every direct child element of
// the preserved control whose local name matches. The raw inner bytes carry
// undeclared namespace prefixes (e.g. "w:listItem"); Go's decoder leaves the
// prefix in Name.Space without erroring, so matching is by local name.
func (c *CT_SdtControl) forEachChild(local string, fn func(attrs []xml.Attr)) {
	if c == nil || len(c.raw.RawContent) == 0 {
		return
	}
	dec := xml.NewDecoder(bytes.NewReader(c.raw.RawContent))
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 && t.Name.Local == local {
				fn(t.Attr)
			}
		case xml.EndElement:
			depth--
		}
	}
}

// isSdtControlElem reports whether an element is one of the modeled content
// control types.
func isSdtControlElem(n xml.Name) bool {
	switch n.Space {
	case NsWml:
		switch n.Local {
		case "text", "richText", "dropDownList", "comboBox", "date", "picture":
			return true
		}
	case NsW14:
		return n.Local == "checkbox"
	}
	return false
}

// AppendSdtRun appends an inline structured document tag to this paragraph,
// keeping child order coherent like AppendR.
func (p *CT_P) AppendSdtRun(s *CT_SdtRun) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildSdtRun, len(p.SdtRun)})
	p.SdtRun = append(p.SdtRun, s)
}

// EnsurePr returns the SDT properties, creating them if absent.
func (s *CT_SdtBlock) EnsurePr() *CT_SdtPr {
	if s.SdtPr == nil {
		s.SdtPr = &CT_SdtPr{}
	}
	return s.SdtPr
}

// EnsurePr returns the SDT properties, creating them if absent.
func (sr *CT_SdtRun) EnsurePr() *CT_SdtPr {
	if sr.SdtPr == nil {
		sr.SdtPr = &CT_SdtPr{}
	}
	return sr.SdtPr
}

// ContentText returns the concatenated text of a block SDT's content
// paragraphs, joined by newlines.
func (s *CT_SdtBlock) ContentText() string {
	var sb strings.Builder
	for i, p := range s.contentParagraphs() {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(p.Text())
	}
	return sb.String()
}

// SetContentText replaces the block SDT content with a single paragraph holding
// text. Existing content (and its formatting) is discarded, which is the
// intended behavior for setting a content control's value.
func (s *CT_SdtBlock) SetContentText(text string) {
	r := &CT_R{}
	r.SetTexts([]*CT_Text{{Space: "preserve", Text: text}})
	p := &CT_P{}
	p.AppendR(r)
	sc := &CT_SdtContentBlock{}
	sc.AppendP(p)
	s.SdtContent = sc
}

// ContentText returns the concatenated text of an inline SDT's content runs.
func (sr *CT_SdtRun) ContentText() string {
	if sr.SdtContent == nil {
		return ""
	}
	var sb strings.Builder
	writeRunsText(&sb, sr.SdtContent.R)
	return sb.String()
}

// SetContentText replaces the inline SDT content with a single run holding text.
func (sr *CT_SdtRun) SetContentText(text string) {
	r := &CT_R{}
	r.SetTexts([]*CT_Text{{Space: "preserve", Text: text}})
	if sr.SdtContent == nil {
		sr.SdtContent = &CT_SdtContentRun{}
	}
	sr.SdtContent.setRuns([]*CT_R{r})
}

// setRuns replaces the inline content's top-level runs, resetting child order so
// the single value run is the only content emitted.
func (sc *CT_SdtContentRun) setRuns(rs []*CT_R) {
	sc.R = rs
	sc.Hyperlink = nil
	sc.childOrder = sc.childOrder[:0]
	for i := range rs {
		sc.childOrder = append(sc.childOrder, pChildRef{pChildR, i})
	}
}

// SdtRef identifies one content control located during a document walk; exactly
// one of Block or Run is non-nil.
type SdtRef struct {
	Block *CT_SdtBlock
	Run   *CT_SdtRun
}

// ContentControls returns every structured document tag in the body in document
// order, descending into nested SDTs, tables, and paragraphs through the shared
// block visitor.
func (body *CT_Body) ContentControls() []SdtRef {
	var out []SdtRef
	visitBlockContent(body.childOrder, body.P, body.Tbl, body.SdtBlock, sdtCollector(&out))
	return out
}

// sdtCollector is the block visitor that records every block SDT it reaches and
// every inline SDT of every paragraph it reaches.
func sdtCollector(out *[]SdtRef) blockVisitor {
	return blockVisitor{
		Sdt:  func(s *CT_SdtBlock) { *out = append(*out, SdtRef{Block: s}) },
		Para: func(p *CT_P) { collectParaSdt(out, p) },
	}
}

// collectParaSdt records a paragraph's inline SDTs in document order.
//
// It reads p.SdtRun *and* the inline SDTs nested inside the paragraph's
// hyperlinks, tracked-change wrappers (w:ins/w:del) and simple fields, all of
// which EG_PContent allows and the model carries. Reading only p.SdtRun made
// ContentControls() miss every control a producer put inside one of them —
// silently skipping, for a consumer templating by tag, controls that are
// perfectly ordinary in form documents (C405). The shared paragraph-content
// descent does the nesting, so a container added to the model later is taught
// once (see pcontent.go).
func collectParaSdt(out *[]SdtRef, p *CT_P) {
	if p == nil {
		return
	}
	VisitContent(p, ContentVisitor{
		SdtRun: func(sr *CT_SdtRun) { *out = append(*out, SdtRef{Run: sr}) },
	})
}
