package oxml

// This file holds the single descent used by every walker over WML *paragraph*
// content — the run-level counterpart of blockvisit.go's descent over block
// content.
//
// EG_PContent nests: a w:r can sit directly in the w:p, or inside a
// w:hyperlink, a w:ins/w:del tracked-change wrapper, a w:fldSimple, or an
// inline w:sdt's content — and each of those containers can nest the others
// again. Every reader that wanted "the runs of this paragraph" hand-rolled its
// own subset of that descent, and each subset was different: the content-control
// collector read only p.SdtRun (C405), the merge-field and form-field scanners
// read only p.R plus p.Hyperlink[].R (C498), the image reader adds p.Ins[].R and
// stops there. The revision transforms already had the full descent, expressed
// over pContentRefs; this file lifts it out so read-only walkers share it
// instead of re-deriving it.
//
// Unlike itemsOf, the walk here never calls backfillChildOrder: a read must not
// mutate the model it reads. Containers with no recorded child order (built
// programmatically rather than parsed) fall back to declaration order, grouped
// by kind exactly as backfillChildOrder would have grouped them.

// ContentVisitor collects callbacks for the paragraph-content node kinds a
// run-level walk can reach. Every field is optional; a nil hook simply is not
// called, and the descent happens regardless.
type ContentVisitor struct {
	// Run is called for each w:r, in document order.
	Run func(*CT_R)
	// SdtRun is called for each inline structured document tag before its
	// content is visited.
	SdtRun func(*CT_SdtRun)
	// FldSimple is called for each w:fldSimple before its content is visited.
	FldSimple func(*CT_SimpleField)
	// TrackChange is called for each w:ins/w:del wrapper before its content is
	// visited.
	TrackChange func(*CT_RunTrackChange)
	// Hyperlink is called for each w:hyperlink before its content is visited.
	Hyperlink func(*CT_Hyperlink)
}

// VisitContent walks a paragraph-content container's children in document
// order, descending into hyperlinks, tracked-change wrappers, simple fields and
// inline SDT content. Nesting is unbounded in the schema, so the descent is
// recursive; the parsed model is a tree, so it terminates.
func VisitContent(c RevContainer, v ContentVisitor) {
	if c == nil {
		return
	}
	visitContentRefs(c.contentRefs(), v)
}

// visitContentRefs is VisitContent over an already-resolved ref bundle.
func visitContentRefs(refs pContentRefs, v ContentVisitor) {
	for _, ref := range refs.orderedChildren() {
		switch ref.kind {
		case pChildR:
			if r, ok := refs.valueAt(ref).(*CT_R); ok && v.Run != nil {
				v.Run(r)
			}
		case pChildHyperlink:
			if h, ok := refs.valueAt(ref).(*CT_Hyperlink); ok && h != nil {
				if v.Hyperlink != nil {
					v.Hyperlink(h)
				}
				visitContentRefs(h.contentRefs(), v)
			}
		case pChildIns, pChildDel:
			if tc, ok := refs.valueAt(ref).(*CT_RunTrackChange); ok && tc != nil {
				if v.TrackChange != nil {
					v.TrackChange(tc)
				}
				visitContentRefs(tc.contentRefs(), v)
			}
		case pChildFldSimple:
			if f, ok := refs.valueAt(ref).(*CT_SimpleField); ok && f != nil {
				if v.FldSimple != nil {
					v.FldSimple(f)
				}
				visitContentRefs(f.contentRefs(), v)
			}
		case pChildSdtRun:
			if s, ok := refs.valueAt(ref).(*CT_SdtRun); ok && s != nil {
				if v.SdtRun != nil {
					v.SdtRun(s)
				}
				if s.SdtContent != nil {
					visitContentRefs(s.SdtContent.contentRefs(), v)
				}
			}
		}
	}
}

// orderedChildren returns the container's child references in document order
// without mutating it. A container that recorded no order (built through the
// public API rather than parsed) yields its typed children grouped by kind in
// slice order, matching backfillChildOrder — but computed into a fresh slice so
// the read leaves the model untouched.
func (refs pContentRefs) orderedChildren() []pChildRef {
	if refs.childOrder != nil && len(*refs.childOrder) > 0 {
		return *refs.childOrder
	}
	var out []pChildRef
	add := func(kind pChildKind, n int) {
		for i := 0; i < n; i++ {
			out = append(out, pChildRef{kind, i})
		}
	}
	if refs.r != nil {
		add(pChildR, len(*refs.r))
	}
	if refs.hyperlink != nil {
		add(pChildHyperlink, len(*refs.hyperlink))
	}
	if refs.ins != nil {
		add(pChildIns, len(*refs.ins))
	}
	if refs.del != nil {
		add(pChildDel, len(*refs.del))
	}
	if refs.fldSimple != nil {
		add(pChildFldSimple, len(*refs.fldSimple))
	}
	if refs.sdtRun != nil {
		add(pChildSdtRun, len(*refs.sdtRun))
	}
	return out
}

// ContainerRuns returns every w:r reachable from a paragraph-content container
// in document order, descending into hyperlinks, tracked-change wrappers,
// simple fields and inline SDT content. Field state machines (merge fields,
// legacy form fields) run over this sequence so a complex field whose
// begin/instrText/end runs straddle one of those containers is still read as a
// single field.
func ContainerRuns(c RevContainer) []*CT_R {
	var out []*CT_R
	VisitContent(c, ContentVisitor{Run: func(r *CT_R) {
		if r != nil {
			out = append(out, r)
		}
	}})
	return out
}
