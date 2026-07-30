package pptx

import (
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// SlideLayoutType represents the type of a slide layout.
type SlideLayoutType string

const (
	LayoutTitle                SlideLayoutType = "title"
	LayoutTitleAndContent      SlideLayoutType = "obj"
	LayoutSectionHeader        SlideLayoutType = "secHead"
	LayoutTwoContent           SlideLayoutType = "twoObj"
	LayoutComparison           SlideLayoutType = "twoTxTwoObj"
	LayoutTitleOnly            SlideLayoutType = "titleOnly"
	LayoutBlank                SlideLayoutType = "blank"
	LayoutContentWithCaption   SlideLayoutType = "objTx"
	LayoutPictureWithCaption   SlideLayoutType = "picTx"
	LayoutTitleAndVerticalText SlideLayoutType = "vertTx"
	LayoutVerticalTitleAndText SlideLayoutType = "vertTitleAndTx"
)

// SlideLayout represents a slide layout.
type SlideLayout struct {
	presentation *Presentation
	master       *SlideMaster
	partName     string
	layoutXML    *oxml.SlideLayout
	layoutType   SlideLayoutType
	name         string
	relID        string
}

// Name returns the name of the layout.
func (sl *SlideLayout) Name() string {
	if sl.name != "" {
		return sl.name
	}
	if sl.layoutXML != nil {
		return sl.layoutXML.MatchingName
	}
	return ""
}

// SetName sets the name of the layout.
func (sl *SlideLayout) SetName(name string) {
	sl.presentation.markModelEdited()
	sl.name = name
	if sl.layoutXML != nil {
		sl.layoutXML.MatchingName = name
	}
}

// Type returns the layout type.
func (sl *SlideLayout) Type() SlideLayoutType {
	return sl.layoutType
}

// Master returns the slide master this layout is based on.
func (sl *SlideLayout) Master() *SlideMaster {
	return sl.master
}

// Placeholders returns the placeholder shapes defined in this layout's shape
// tree, materialized read-only from the parsed XML: mutating the returned
// shapes does not modify the layout part, which is written from its parsed
// tree on save.
func (sl *SlideLayout) Placeholders() []*PlaceholderShape {
	if sl.layoutXML == nil || sl.layoutXML.CSld == nil {
		return nil
	}
	return placeholdersFromSpTree(sl.layoutXML.CSld.SpTree)
}

// Placeholder returns the layout's placeholder of the given type, or
// ErrPlaceholderNotFound when it has none. See Slide.Placeholder (C565).
func (sl *SlideLayout) Placeholder(phType PlaceholderType) (*PlaceholderShape, error) {
	if ph := sl.GetPlaceholder(phType); ph != nil {
		return ph, nil
	}
	return nil, fmt.Errorf("%w: type %v on layout %q", ErrPlaceholderNotFound, phType, sl.Name())
}

// GetPlaceholder returns the placeholder with the specified type.
//
// Deprecated: use Placeholder, which reports a miss as an error (C565).
func (sl *SlideLayout) GetPlaceholder(phType PlaceholderType) *PlaceholderShape {
	for _, ph := range sl.Placeholders() {
		if ph.PlaceholderType() == phType {
			return ph
		}
	}
	return nil
}

// EditablePlaceholder is a mutable handle to a placeholder shape (a p:sp
// carrying a p:ph) in a master's or layout's shape tree. Its geometry setters
// write the a:off/a:ext of the shape's a:xfrm in place, so an unedited
// placeholder — and every unmodeled property of an edited one — round-trips
// byte-for-byte.
type EditablePlaceholder struct {
	sp *oxml.Shape
	// owner is the deck the wrapped node belongs to, so an edit through this
	// handle can be recorded. Master and layout parts are regenerated on every
	// save, so the edit persists without a flag and nothing else would notice
	// the deck changed.
	owner *Presentation
}

// editablePlaceholdersFromSpTree wraps every placeholder shape of a shape tree.
func editablePlaceholdersFromSpTree(owner *Presentation, spTree *oxml.ShapeTree) []*EditablePlaceholder {
	if spTree == nil {
		return nil
	}
	var out []*EditablePlaceholder
	for _, sp := range spTree.Sp {
		if sp == nil || sp.NvSpPr == nil || sp.NvSpPr.NvPr == nil || sp.NvSpPr.NvPr.Ph == nil {
			continue
		}
		out = append(out, &EditablePlaceholder{sp: sp, owner: owner})
	}
	return out
}

func (ep *EditablePlaceholder) ph() *oxml.Placeholder {
	if ep.sp.NvSpPr == nil || ep.sp.NvSpPr.NvPr == nil {
		return nil
	}
	return ep.sp.NvSpPr.NvPr.Ph
}

// Type returns the placeholder type.
func (ep *EditablePlaceholder) Type() PlaceholderType {
	if ph := ep.ph(); ph != nil {
		return PlaceholderType(ph.Type)
	}
	return ""
}

// Index returns the placeholder index.
func (ep *EditablePlaceholder) Index() uint32 {
	if ph := ep.ph(); ph != nil {
		return ph.Idx
	}
	return 0
}

// ensureXfrm returns the placeholder's transform, allocating it as needed. Only
// the setters call it, so it also records the edit (see EditablePlaceholder.owner).
func (ep *EditablePlaceholder) ensureXfrm() *dml.Xfrm {
	ep.owner.markModelEdited()
	if ep.sp.SpPr == nil {
		ep.sp.SpPr = &dml.SpPr{}
	}
	if ep.sp.SpPr.Xfrm == nil {
		ep.sp.SpPr.Xfrm = &dml.Xfrm{}
	}
	return ep.sp.SpPr.Xfrm
}

// Position returns the placeholder's explicit position (a:off) and true, or a
// zero position and false when the placeholder sets no transform (its geometry
// is then inherited).
func (ep *EditablePlaceholder) Position() (x, y dml.EMU, ok bool) {
	if ep.sp.SpPr == nil || ep.sp.SpPr.Xfrm == nil || ep.sp.SpPr.Xfrm.Off == nil {
		return 0, 0, false
	}
	off := ep.sp.SpPr.Xfrm.Off
	return dml.EMU(off.X), dml.EMU(off.Y), true
}

// SetPosition sets the placeholder's position (a:off), allocating the transform
// if the shape had none.
func (ep *EditablePlaceholder) SetPosition(x, y dml.EMU) {
	xf := ep.ensureXfrm()
	if xf.Off == nil {
		xf.Off = &dml.OffXML{}
	}
	xf.Off.X = int64(x)
	xf.Off.Y = int64(y)
}

// Size returns the placeholder's explicit size (a:ext) and true, or a zero size
// and false when the placeholder sets no transform.
func (ep *EditablePlaceholder) Size() (width, height dml.EMU, ok bool) {
	if ep.sp.SpPr == nil || ep.sp.SpPr.Xfrm == nil || ep.sp.SpPr.Xfrm.Ext == nil {
		return 0, 0, false
	}
	ext := ep.sp.SpPr.Xfrm.Ext
	return dml.EMU(ext.Cx), dml.EMU(ext.Cy), true
}

// SetSize sets the placeholder's size (a:ext), allocating the transform if the
// shape had none.
func (ep *EditablePlaceholder) SetSize(width, height dml.EMU) {
	xf := ep.ensureXfrm()
	if xf.Ext == nil {
		xf.Ext = &dml.ExtXML{}
	}
	xf.Ext.Cx = int64(width)
	xf.Ext.Cy = int64(height)
}

// EditablePlaceholders returns mutable handles to every placeholder in the
// layout's shape tree, in document order.
func (sl *SlideLayout) EditablePlaceholders() []*EditablePlaceholder {
	if sl.layoutXML == nil || sl.layoutXML.CSld == nil {
		return nil
	}
	return editablePlaceholdersFromSpTree(sl.presentation, sl.layoutXML.CSld.SpTree)
}

// EditablePlaceholder returns a mutable handle to the first layout placeholder
// of the given type, or nil when none matches.
func (sl *SlideLayout) EditablePlaceholder(phType PlaceholderType) *EditablePlaceholder {
	for _, ep := range sl.EditablePlaceholders() {
		if ep.Type() == phType {
			return ep
		}
	}
	return nil
}

// LayoutTypeFromString converts a string to a SlideLayoutType.
func LayoutTypeFromString(s string) SlideLayoutType {
	switch s {
	case "title":
		return LayoutTitle
	case "obj":
		return LayoutTitleAndContent
	case "secHead":
		return LayoutSectionHeader
	case "twoObj":
		return LayoutTwoContent
	case "twoTxTwoObj":
		return LayoutComparison
	case "titleOnly":
		return LayoutTitleOnly
	case "blank":
		return LayoutBlank
	case "objTx":
		return LayoutContentWithCaption
	case "picTx":
		return LayoutPictureWithCaption
	case "vertTx":
		return LayoutTitleAndVerticalText
	case "vertTitleAndTx":
		return LayoutVerticalTitleAndText
	default:
		return SlideLayoutType(s)
	}
}

// String returns the string representation of the layout type.
func (lt SlideLayoutType) String() string {
	return string(lt)
}

// DisplayName returns a human-readable name for the layout type.
func (lt SlideLayoutType) DisplayName() string {
	switch lt {
	case LayoutTitle:
		return "Title Slide"
	case LayoutTitleAndContent:
		return "Title and Content"
	case LayoutSectionHeader:
		return "Section Header"
	case LayoutTwoContent:
		return "Two Content"
	case LayoutComparison:
		return "Comparison"
	case LayoutTitleOnly:
		return "Title Only"
	case LayoutBlank:
		return "Blank"
	case LayoutContentWithCaption:
		return "Content with Caption"
	case LayoutPictureWithCaption:
		return "Picture with Caption"
	case LayoutTitleAndVerticalText:
		return "Title and Vertical Text"
	case LayoutVerticalTitleAndText:
		return "Vertical Title and Text"
	default:
		return string(lt)
	}
}

// marshal converts the slide layout to XML bytes.
func (sl *SlideLayout) marshal() ([]byte, error) {
	if sl.layoutXML == nil {
		sl.layoutXML = newLayoutXML(sl.layoutType)
	}

	// Use the namespace-aware marshaler for PowerPoint compatibility
	return marshalSlideLayout(sl.layoutXML)
}

// newLayoutXML creates a new slide layout XML structure.
func newLayoutXML(layoutType SlideLayoutType) *oxml.SlideLayout {
	return &oxml.SlideLayout{
		Type:         string(layoutType),
		Preserve:     true,
		MatchingName: layoutType.DisplayName(),
		CSld: &oxml.CommonSlideData{
			SpTree: newShapeTree(),
		},
		ClrMapOvr: &oxml.ColorMapOverride{
			MasterClrMapping: &oxml.MasterColorMapping{},
		},
	}
}

// createDefaultLayout creates a slide layout with default placeholders.
func createDefaultLayout(layoutType SlideLayoutType, master *SlideMaster, w, h dml.EMU) *SlideLayout {
	layout := &SlideLayout{
		master:     master,
		layoutType: layoutType,
		name:       layoutType.DisplayName(),
		layoutXML:  newLayoutXML(layoutType),
	}

	// Add placeholders based on layout type. All geometry is derived from the
	// slide size (w, h) so a created deck is internally consistent (C139).
	var placeholders []*PlaceholderShape

	switch layoutType {
	case LayoutTitle:
		// Title slide: centered title and subtitle
		titlePh := NewPlaceholderShape(PlaceholderCenteredTitle)
		centeredTitleRect(w, h).apply(titlePh)
		titlePh.SetIndex(0)
		subtitlePh := NewPlaceholderShape(PlaceholderSubtitle)
		subtitleRect(w, h).apply(subtitlePh)
		subtitlePh.SetIndex(1)
		placeholders = append(placeholders, titlePh, subtitlePh)

	case LayoutTitleAndContent:
		// Title and content layout
		titlePh := NewPlaceholderShape(PlaceholderTitle)
		titleRect(w, h).apply(titlePh)
		titlePh.SetIndex(0)
		bodyPh := NewPlaceholderShape(PlaceholderBody)
		bodyRect(w, h).apply(bodyPh)
		bodyPh.SetIndex(1)
		placeholders = append(placeholders, titlePh, bodyPh)

	case LayoutTitleOnly:
		// Title only layout
		titlePh := NewPlaceholderShape(PlaceholderTitle)
		titleRect(w, h).apply(titlePh)
		titlePh.SetIndex(0)
		placeholders = append(placeholders, titlePh)

	case LayoutBlank:
		// No placeholders

	case LayoutSectionHeader:
		// Section header: centered title and text
		titlePh := NewPlaceholderShape(PlaceholderTitle)
		sectionTitleRect(w, h).apply(titlePh)
		titlePh.SetIndex(0)

		textPh := NewPlaceholderShape(PlaceholderBody)
		sectionTextRect(w, h).apply(textPh)
		textPh.SetIndex(1)
		placeholders = append(placeholders, titlePh, textPh)

	case LayoutTwoContent:
		// Title and two content areas side by side
		titlePh := NewPlaceholderShape(PlaceholderTitle)
		titleRect(w, h).apply(titlePh)
		titlePh.SetIndex(0)

		leftPh := NewPlaceholderShape(PlaceholderBody)
		leftContentRect(w, h).apply(leftPh)
		leftPh.SetIndex(1)

		rightPh := NewPlaceholderShape(PlaceholderBody)
		rightContentRect(w, h).apply(rightPh)
		rightPh.SetIndex(2)
		placeholders = append(placeholders, titlePh, leftPh, rightPh)
	}

	// Add placeholders to layout XML
	if len(placeholders) > 0 && layout.layoutXML.CSld.SpTree != nil {
		var shapeID uint32 = 2
		for _, ph := range placeholders {
			sp := placeholderToOxml(ph, shapeID)
			layout.layoutXML.CSld.SpTree.Sp = append(layout.layoutXML.CSld.SpTree.Sp, sp)
			shapeID++
		}
	}

	return layout
}
