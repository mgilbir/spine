package oxml

import (
	"bytes"
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Tracked moves (w:moveFrom / w:moveTo) are the two halves of a Word "move"
// revision: the content Word cut from one location (moveFrom) and the copy it
// pasted at the destination (moveTo), each paired with range markers
// (moveFromRangeStart/End, moveToRangeStart/End) that carry a shared w:name.
//
// The content wrappers and their range markers are preserved verbatim as raw
// paragraph-content children (see isRawPChild), so an untouched document
// round-trips byte-for-byte. This file layers enumeration and accept/reject on
// top of that raw capture without changing the stored representation: a move is
// read by parsing the raw element, and a transform rewrites the container's
// child list — dropping the block or splicing its decoded content in place.

// MoveKind distinguishes the two halves of a tracked move.
type MoveKind int

const (
	// MoveFrom is the source half of a move (w:moveFrom): the content in the
	// location the text was moved away from.
	MoveFrom MoveKind = iota
	// MoveTo is the destination half of a move (w:moveTo): the content in the
	// location the text was moved to.
	MoveTo
)

// RawMove is one enumerated tracked-move content block reachable from a
// container. It carries the block's display metadata plus the container and raw
// element the transforms rewrite.
type RawMove struct {
	Kind      MoveKind
	Id        string
	Author    string
	Date      string
	Name      string // paired move name from the enclosing range marker, best-effort
	Text      string
	Container RevContainer
	Raw       *CT_RawNamedElement
}

// isMoveContent reports whether a raw element local name is a move content
// wrapper (not a range marker).
func isMoveContent(local string) (MoveKind, bool) {
	switch local {
	case "moveFrom":
		return MoveFrom, true
	case "moveTo":
		return MoveTo, true
	}
	return 0, false
}

// CollectContainerMoves appends every tracked-move content block reachable from
// c in document order, descending into hyperlinks, fields, structured document
// tags, and insertion/deletion blocks (the same containers CollectParagraph
// revisions walks). The move name is resolved best-effort from the nearest
// preceding range-start marker in the same container.
func CollectContainerMoves(dst []RawMove, c RevContainer) []RawMove {
	var lastFromName, lastToName string
	for _, it := range itemsOf(c) {
		switch it.kind {
		case pChildRaw:
			rn := it.val.(*CT_RawNamedElement)
			switch rn.Local {
			case "moveFromRangeStart":
				lastFromName = rn.Attr("name")
			case "moveToRangeStart":
				lastToName = rn.Attr("name")
			default:
				if kind, ok := isMoveContent(rn.Local); ok {
					name := lastFromName
					if kind == MoveTo {
						name = lastToName
					}
					dst = append(dst, RawMove{
						Kind:      kind,
						Id:        rn.Attr("id"),
						Author:    rn.Attr("author"),
						Date:      rn.Attr("date"),
						Name:      name,
						Text:      MoveBlockText(rn),
						Container: c,
						Raw:       rn,
					})
				}
			}
		case pChildHyperlink:
			dst = CollectContainerMoves(dst, it.val.(*CT_Hyperlink))
		case pChildFldSimple:
			dst = CollectContainerMoves(dst, it.val.(*CT_SimpleField))
		case pChildSdtRun:
			if s := it.val.(*CT_SdtRun); s.SdtContent != nil {
				dst = CollectContainerMoves(dst, s.SdtContent)
			}
		case pChildIns, pChildDel:
			dst = CollectContainerMoves(dst, it.val.(*CT_RunTrackChange))
		}
	}
	return dst
}

// isMoveElement reports whether a raw element local name is a move content
// wrapper or a move range marker (the elements that carry a revision id).
func isMoveElement(local string) bool {
	switch local {
	case "moveFrom", "moveTo",
		"moveFromRangeStart", "moveFromRangeEnd",
		"moveToRangeStart", "moveToRangeEnd":
		return true
	}
	return false
}

// MaxMoveID feeds the revision ids of every tracked-move element reachable from
// c (content wrappers and range markers, descending into nested containers) to
// consider, so authoring can allocate ids above any move already present.
func MaxMoveID(c RevContainer, consider func(string)) {
	for _, it := range itemsOf(c) {
		switch it.kind {
		case pChildRaw:
			if rn := it.val.(*CT_RawNamedElement); isMoveElement(rn.Local) {
				consider(rn.Attr("id"))
			}
		case pChildHyperlink:
			MaxMoveID(it.val.(*CT_Hyperlink), consider)
		case pChildFldSimple:
			MaxMoveID(it.val.(*CT_SimpleField), consider)
		case pChildSdtRun:
			if s := it.val.(*CT_SdtRun); s.SdtContent != nil {
				MaxMoveID(s.SdtContent, consider)
			}
		case pChildIns, pChildDel:
			MaxMoveID(it.val.(*CT_RunTrackChange), consider)
		}
	}
}

// moveWrapNS is the namespace prelude used to decode a move block's raw inner
// content. Go's decoder does not enforce namespace binding, so declaring the
// WordprocessingML namespace is enough for the run content Word records in a
// move; unbound prefixes (r:, w14:, ...) survive as literal spaces on the
// captured attributes and replay verbatim.
const moveWrapNS = ` xmlns:w="` + NsWml + `"`

// decodeMoveBlock parses a move content wrapper's raw inner XML into a typed
// tracked-change block, so its content can be read or spliced into a container.
// The wrapper element name does not matter (moveFrom and moveTo share the
// CT_RunTrackChange content model); it is decoded under w:moveTo purely to
// satisfy the decoder.
func decodeMoveBlock(rn *CT_RawNamedElement) *CT_RunTrackChange {
	block := &CT_RunTrackChange{}
	if len(rn.RawContent) == 0 {
		return block
	}
	var buf bytes.Buffer
	buf.WriteString("<w:moveTo")
	buf.WriteString(moveWrapNS)
	buf.WriteByte('>')
	buf.Write(rn.RawContent)
	buf.WriteString("</w:moveTo>")
	dec := xml.NewDecoder(bytes.NewReader(buf.Bytes()))
	if err := dec.Decode(block); err != nil {
		return &CT_RunTrackChange{}
	}
	return block
}

// MoveBlockText returns the concatenated visible text of a move content block.
func MoveBlockText(rn *CT_RawNamedElement) string {
	return BlockText(decodeMoveBlock(rn))
}

// DropMoveBlock removes a move content wrapper (and its content) from c,
// reporting whether it was a direct raw child of c. Accepting a moveFrom or
// rejecting a moveTo drops the block.
func DropMoveBlock(c RevContainer, rn *CT_RawNamedElement) bool {
	items := itemsOf(c)
	for i, it := range items {
		if it.kind == pChildRaw && it.val == any(rn) {
			out := make([]pItem, 0, len(items)-1)
			out = append(out, items[:i]...)
			out = append(out, items[i+1:]...)
			setItemsOf(c, out)
			return true
		}
	}
	return false
}

// UnwrapMoveBlock replaces a move content wrapper with its own content spliced
// in at the same position, so the moved text becomes normal runs. Accepting a
// moveTo or rejecting a moveFrom unwraps the block. It reports whether the block
// was a direct raw child of c.
func UnwrapMoveBlock(c RevContainer, rn *CT_RawNamedElement) bool {
	items := itemsOf(c)
	for i, it := range items {
		if it.kind == pChildRaw && it.val == any(rn) {
			inner := itemsOf(decodeMoveBlock(rn))
			out := make([]pItem, 0, len(items)-1+len(inner))
			out = append(out, items[:i]...)
			out = append(out, inner...)
			out = append(out, items[i+1:]...)
			setItemsOf(c, out)
			return true
		}
	}
	return false
}

// acceptMovesInItems maps a container's items for AcceptAllInContainer: a
// moveFrom block is dropped and a moveTo block is unwrapped (its content kept),
// which is the net effect of accepting a move. Other raw items pass through.
func acceptMoveRaw(rn *CT_RawNamedElement, out []pItem) ([]pItem, bool) {
	kind, ok := isMoveContent(rn.Local)
	if !ok {
		return out, false
	}
	if kind == MoveTo {
		out = append(out, itemsOf(decodeMoveBlock(rn))...)
	}
	// moveFrom content is dropped.
	return out, true
}

// rejectMoveRaw is the inverse of acceptMoveRaw: a moveTo block is dropped and a
// moveFrom block is unwrapped.
func rejectMoveRaw(rn *CT_RawNamedElement, out []pItem) ([]pItem, bool) {
	kind, ok := isMoveContent(rn.Local)
	if !ok {
		return out, false
	}
	if kind == MoveFrom {
		out = append(out, itemsOf(decodeMoveBlock(rn))...)
	}
	// moveTo content is dropped.
	return out, true
}

// NewMoveBlock builds a move content wrapper (w:moveFrom or w:moveTo) carrying
// id/author/date and wrapping run r, ready to append as a raw paragraph-content
// child. The run is serialized to the wrapper's raw inner XML so it replays
// through the raw-preservation path.
func NewMoveBlock(local, id, author, date string, r *CT_R) *CT_RawNamedElement {
	rn := &CT_RawNamedElement{Local: local, Space: NsWml}
	rn.Attrs = []xml.Attr{
		{Name: xml.Name{Space: NsWml, Local: "id"}, Value: id},
		{Name: xml.Name{Space: NsWml, Local: "author"}, Value: author},
		{Name: xml.Name{Space: NsWml, Local: "date"}, Value: date},
	}
	rn.RawContent = marshalRunBytes(r)
	return rn
}

// NewMoveRangeStart builds a move range-start marker (moveFromRangeStart or
// moveToRangeStart) carrying id/author/date/name.
func NewMoveRangeStart(local, id, author, date, name string) *CT_RawNamedElement {
	rn := &CT_RawNamedElement{Local: local, Space: NsWml}
	rn.Attrs = []xml.Attr{
		{Name: xml.Name{Space: NsWml, Local: "id"}, Value: id},
		{Name: xml.Name{Space: NsWml, Local: "author"}, Value: author},
		{Name: xml.Name{Space: NsWml, Local: "date"}, Value: date},
		{Name: xml.Name{Space: NsWml, Local: "name"}, Value: name},
	}
	return rn
}

// NewMoveRangeEnd builds a move range-end marker (moveFromRangeEnd or
// moveToRangeEnd) carrying the range id.
func NewMoveRangeEnd(local, id string) *CT_RawNamedElement {
	rn := &CT_RawNamedElement{Local: local, Space: NsWml}
	rn.Attrs = []xml.Attr{
		{Name: xml.Name{Space: NsWml, Local: "id"}, Value: id},
	}
	return rn
}

// marshalRunBytes serializes a run to its w:-prefixed inner XML, matching the
// bytes the paragraph marshaler produces elsewhere, so an authored move block's
// content is consistent with the rest of the document.
func marshalRunBytes(r *CT_R) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	r.MarshalToBuilder(b, NsWml, "r")
	return b.Bytes()
}

// AppendRaw appends a raw-preserved inline child (e.g. an authored move wrapper
// or range marker) to this paragraph, maintaining child order like AppendR.
func (p *CT_P) AppendRaw(rn *CT_RawNamedElement) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildRaw, len(p.Raw)})
	p.Raw = append(p.Raw, rn)
}
