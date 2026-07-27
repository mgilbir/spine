package oxml

import (
	"encoding/xml"
)

// This file holds the family-wide fallback for children a WML container does
// not model.
//
// Historically each block/inline container carried its own hand-maintained
// whitelist of element names worth preserving (isRawPChild, isRawBodyChild,
// isRawRowChild, plus CT_Tbl's inline list). The four lists drifted apart —
// commentRangeStart/End were in none of them although EG_RunLevelElts is a
// member of EG_BlockLevelElts, EG_ContentRowContent and EG_ContentCellContent,
// so Word's whole-row and whole-cell comment anchors were deleted while the
// comment definition survived (C371). The only two containers with no such
// hole (CT_R, CT_SectPr) are precisely the ones that captured *everything*
// unknown, so that is now the default: an unmodeled child is preserved
// verbatim, never skipped.
//
// The same rule closes the nil-slot variant (C373): a shared content dispatcher
// given a nil slot for a typed child used to call d.Skip(), silently deleting
// content that is schema-valid in that container. Those branches now fall
// through here too, so a container can decline to *type* a child without
// thereby deleting it.

// captureRawChild decodes the element the decoder is positioned on into a
// verbatim CT_RawNamedElement and appends it to raw, reporting the index it
// landed at. When raw is nil the container cannot preserve anything, so the
// element is skipped and ok is false.
func captureRawChild(d *xml.Decoder, t *xml.StartElement, raw *[]*CT_RawNamedElement) (idx int, ok bool, err error) {
	if raw == nil {
		return 0, false, d.Skip()
	}
	v := &CT_RawNamedElement{}
	if err := d.DecodeElement(v, t); err != nil {
		return 0, false, err
	}
	idx = len(*raw)
	*raw = append(*raw, v)
	return idx, true, nil
}

// captureRawPChild is captureRawChild for paragraph-content containers, also
// recording the child's position in the container's order.
func captureRawPChild(d *xml.Decoder, t *xml.StartElement, raw *[]*CT_RawNamedElement, childOrder *[]pChildRef) error {
	idx, ok, err := captureRawChild(d, t, raw)
	if err != nil || !ok {
		return err
	}
	*childOrder = append(*childOrder, pChildRef{pChildRaw, idx})
	return nil
}

// captureRawBodyChild is captureRawChild for block-level containers (body,
// header/footer, table cell, block SDT content).
func captureRawBodyChild(d *xml.Decoder, t *xml.StartElement, raw *[]*CT_RawNamedElement, childOrder *[]bodyChildRef) error {
	idx, ok, err := captureRawChild(d, t, raw)
	if err != nil || !ok {
		return err
	}
	*childOrder = append(*childOrder, bodyChildRef{bodyChildRaw, idx})
	return nil
}
