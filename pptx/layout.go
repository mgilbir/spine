package pptx

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// SlideLayoutType represents the type of a slide layout.
type SlideLayoutType string

const (
	LayoutTitle             SlideLayoutType = "title"
	LayoutTitleAndContent   SlideLayoutType = "obj"
	LayoutSectionHeader     SlideLayoutType = "secHead"
	LayoutTwoContent        SlideLayoutType = "twoObj"
	LayoutComparison        SlideLayoutType = "twoTxTwoObj"
	LayoutTitleOnly         SlideLayoutType = "titleOnly"
	LayoutBlank             SlideLayoutType = "blank"
	LayoutContentWithCaption SlideLayoutType = "objTx"
	LayoutPictureWithCaption SlideLayoutType = "picTx"
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

// Placeholders returns the placeholders defined in this layout.
func (sl *SlideLayout) Placeholders() []*PlaceholderShape {
	// Placeholder implementation - would parse shapes from layoutXML
	return nil
}

// GetPlaceholder returns the placeholder with the specified type.
func (sl *SlideLayout) GetPlaceholder(phType PlaceholderType) *PlaceholderShape {
	for _, ph := range sl.Placeholders() {
		if ph.PlaceholderType() == phType {
			return ph
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

	output, err := xml.MarshalIndent(sl.layoutXML, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
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
func createDefaultLayout(layoutType SlideLayoutType, master *SlideMaster) *SlideLayout {
	layout := &SlideLayout{
		master:     master,
		layoutType: layoutType,
		name:       layoutType.DisplayName(),
		layoutXML:  newLayoutXML(layoutType),
	}

	// Add placeholders based on layout type
	var placeholders []*PlaceholderShape

	switch layoutType {
	case LayoutTitle:
		// Title slide: centered title and subtitle
		titlePh := DefaultCenteredTitlePlaceholder()
		titlePh.SetIndex(0)
		subtitlePh := DefaultSubtitlePlaceholder()
		subtitlePh.SetIndex(1)
		placeholders = append(placeholders, titlePh, subtitlePh)

	case LayoutTitleAndContent:
		// Title and content layout
		titlePh := DefaultTitlePlaceholder()
		titlePh.SetIndex(0)
		bodyPh := DefaultBodyPlaceholder()
		bodyPh.SetIndex(1)
		placeholders = append(placeholders, titlePh, bodyPh)

	case LayoutTitleOnly:
		// Title only layout
		titlePh := DefaultTitlePlaceholder()
		titlePh.SetIndex(0)
		placeholders = append(placeholders, titlePh)

	case LayoutBlank:
		// No placeholders

	case LayoutSectionHeader:
		// Section header: centered title and text
		titlePh := NewPlaceholderShape(PlaceholderTitle)
		titlePh.SetPosition(dml.Inches(0.5), dml.Inches(2.5))
		titlePh.SetSize(dml.Inches(12.33), dml.Inches(1.5))
		titlePh.SetIndex(0)

		textPh := NewPlaceholderShape(PlaceholderBody)
		textPh.SetPosition(dml.Inches(0.5), dml.Inches(4.5))
		textPh.SetSize(dml.Inches(12.33), dml.Inches(1.5))
		textPh.SetIndex(1)
		placeholders = append(placeholders, titlePh, textPh)

	case LayoutTwoContent:
		// Title and two content areas side by side
		titlePh := DefaultTitlePlaceholder()
		titlePh.SetIndex(0)

		leftPh := NewPlaceholderShape(PlaceholderBody)
		leftPh.SetPosition(dml.Inches(0.5), dml.Inches(1.6))
		leftPh.SetSize(dml.Inches(5.92), dml.Inches(5.1))
		leftPh.SetIndex(1)

		rightPh := NewPlaceholderShape(PlaceholderBody)
		rightPh.SetPosition(dml.Inches(6.92), dml.Inches(1.6))
		rightPh.SetSize(dml.Inches(5.92), dml.Inches(5.1))
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
