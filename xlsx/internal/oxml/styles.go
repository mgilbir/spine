// This file contains the XML schema types for XLSX documents.

package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Stylesheet is the root element of styles.xml.
type CT_Stylesheet struct {
	XMLName      xml.Name          `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main styleSheet"`
	NumFmts      *CT_NumFmts       `xml:"numFmts,omitempty"`
	Fonts        *CT_Fonts         `xml:"fonts,omitempty"`
	Fills        *CT_Fills         `xml:"fills,omitempty"`
	Borders      *CT_Borders       `xml:"borders,omitempty"`
	CellStyleXfs *CT_CellStyleXfs  `xml:"cellStyleXfs,omitempty"`
	CellXfs      *CT_CellXfs       `xml:"cellXfs,omitempty"`
	CellStyles   *CT_CellStyles    `xml:"cellStyles,omitempty"`
	Dxfs         *CT_Dxfs          `xml:"dxfs,omitempty"`
	TableStyles  *CT_TableStyles   `xml:"tableStyles,omitempty"`
	Colors       *CT_Colors        `xml:"colors,omitempty"`
	ExtLst       *CT_ExtensionList `xml:"extLst,omitempty"`
	// OriginalNSDecls preserves the namespace declarations from the original XML
	// for byte-identical round-trip of styles.xml.
	OriginalNSDecls []xmlb.NSDecl `xml:"-"`
	// OriginalRootAttrs preserves all root-element attributes (namespace
	// declarations and regular attributes like mc:Ignorable) in their original
	// order, so regenerating a dirty styles.xml does not drop them (C199).
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Stylesheet.
func (ss *CT_Stylesheet) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ss.XMLName = start.Name
	// Capture all root-element attributes in order for round-trip preservation,
	// distinguishing namespace declarations from regular attributes (the same
	// treatment CT_Worksheet got for C74).
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			ss.OriginalNSDecls = append(ss.OriginalNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
			ss.OriginalRootAttrs = append(ss.OriginalRootAttrs, xmlb.RootAttr{IsNS: true, Prefix: attr.Name.Local, Value: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			// Default namespace: xmlns="URI"
			ss.OriginalNSDecls = append([]xmlb.NSDecl{{Prefix: "", URI: attr.Value}}, ss.OriginalNSDecls...)
			ss.OriginalRootAttrs = append(ss.OriginalRootAttrs, xmlb.RootAttr{IsNS: true, Prefix: "", Value: attr.Value})
		default:
			prefix := ""
			switch attr.Name.Space {
			case xmlb.NSMarkupCompatibility:
				prefix = xmlb.PrefixMarkupCompatibility
			case nsR:
				prefix = "r"
			case "":
				// no prefix
			default:
				for _, ra := range ss.OriginalRootAttrs {
					if ra.IsNS && ra.Value == attr.Name.Space {
						prefix = ra.Prefix
						break
					}
				}
			}
			ss.OriginalRootAttrs = append(ss.OriginalRootAttrs, xmlb.RootAttr{IsNS: false, Prefix: prefix, LocalName: attr.Name.Local, Value: attr.Value})
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
			case "numFmts":
				ss.NumFmts = &CT_NumFmts{}
				if err := d.DecodeElement(ss.NumFmts, &t); err != nil {
					return err
				}
			case "fonts":
				ss.Fonts = &CT_Fonts{}
				if err := d.DecodeElement(ss.Fonts, &t); err != nil {
					return err
				}
			case "fills":
				ss.Fills = &CT_Fills{}
				if err := d.DecodeElement(ss.Fills, &t); err != nil {
					return err
				}
			case "borders":
				ss.Borders = &CT_Borders{}
				if err := d.DecodeElement(ss.Borders, &t); err != nil {
					return err
				}
			case "cellStyleXfs":
				ss.CellStyleXfs = &CT_CellStyleXfs{}
				if err := d.DecodeElement(ss.CellStyleXfs, &t); err != nil {
					return err
				}
			case "cellXfs":
				ss.CellXfs = &CT_CellXfs{}
				if err := d.DecodeElement(ss.CellXfs, &t); err != nil {
					return err
				}
			case "cellStyles":
				ss.CellStyles = &CT_CellStyles{}
				if err := d.DecodeElement(ss.CellStyles, &t); err != nil {
					return err
				}
			case "dxfs":
				ss.Dxfs = &CT_Dxfs{}
				if err := d.DecodeElement(ss.Dxfs, &t); err != nil {
					return err
				}
			case "tableStyles":
				ss.TableStyles = &CT_TableStyles{}
				if err := d.DecodeElement(ss.TableStyles, &t); err != nil {
					return err
				}
			case "colors":
				ss.Colors = &CT_Colors{}
				if err := d.DecodeElement(ss.Colors, &t); err != nil {
					return err
				}
			case "extLst":
				ss.ExtLst = &CT_ExtensionList{}
				if err := d.DecodeElement(ss.ExtLst, &t); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// --- Number Formats ---

// CT_NumFmts represents the numFmts element containing custom number format definitions.
type CT_NumFmts struct {
	Count  *uint32      `xml:"count,attr,omitempty"`
	NumFmt []CT_NumFmt  `xml:"numFmt"`
}

// CT_NumFmt represents a single number format definition.
type CT_NumFmt struct {
	NumFmtId   uint32 `xml:"numFmtId,attr"`
	FormatCode string `xml:"formatCode,attr"`
}

// --- Fonts ---

// CT_Fonts represents the fonts element containing font definitions.
type CT_Fonts struct {
	Count      *uint32 `xml:"count,attr,omitempty"`
	KnownFonts *bool   `xml:"knownFonts,attr,omitempty"`
	Font       []CT_Font `xml:"font"`
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Fonts. knownFonts
// is not a CT_Fonts attribute in the sml schema — it is the x14ac extension
// attribute x14ac:knownFonts — so it must be emitted with the x14ac prefix
// (the way CT_SheetFormatPr emits dyDescent), not as a bare attribute (C199).
func (f *CT_Fonts) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if f.Count != nil {
		attrs = append(attrs, xmlb.UintAttr("count", *f.Count))
	}
	if f.KnownFonts != nil {
		v := "0"
		if *f.KnownFonts {
			v = "1"
		}
		attrs = append(attrs, xmlb.Attr{Namespace: nsX14AC, Name: "knownFonts", Value: v})
	}
	if len(f.Font) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	for i := range f.Font {
		b.MarshalElement(ns, "font", &f.Font[i])
	}
	b.EndElement(ns, localName)
}

// CT_Font represents a single font definition with optional property child elements.
type CT_Font struct {
	Name      *CT_FontName                  `xml:"name,omitempty"`
	Charset   *CT_IntProperty               `xml:"charset,omitempty"`
	Family    *CT_IntProperty               `xml:"family,omitempty"`
	B         *CT_BooleanProperty           `xml:"b,omitempty"`
	I         *CT_BooleanProperty           `xml:"i,omitempty"`
	Strike    *CT_BooleanProperty           `xml:"strike,omitempty"`
	Outline   *CT_BooleanProperty           `xml:"outline,omitempty"`
	Shadow    *CT_BooleanProperty           `xml:"shadow,omitempty"`
	Condense  *CT_BooleanProperty           `xml:"condense,omitempty"`
	Extend    *CT_BooleanProperty           `xml:"extend,omitempty"`
	Color     *CT_Color                     `xml:"color,omitempty"`
	Sz        *CT_FontSize                  `xml:"sz,omitempty"`
	U         *CT_UnderlineProperty         `xml:"u,omitempty"`
	VertAlign *CT_VerticalAlignFontProperty `xml:"vertAlign,omitempty"`
	Scheme    *CT_FontScheme                `xml:"scheme,omitempty"`
}

// --- Fills ---

// CT_Fills represents the fills element containing fill definitions.
type CT_Fills struct {
	Count *uint32   `xml:"count,attr,omitempty"`
	Fill  []CT_Fill `xml:"fill"`
}

// CT_Fill represents a single fill definition, either a pattern fill or gradient fill.
type CT_Fill struct {
	PatternFill  *CT_PatternFill  `xml:"patternFill,omitempty"`
	GradientFill *CT_GradientFill `xml:"gradientFill,omitempty"`
}

// CT_PatternFill represents a pattern fill with optional foreground and background colors.
type CT_PatternFill struct {
	PatternType string   `xml:"patternType,attr,omitempty"`
	FgColor     *CT_Color `xml:"fgColor,omitempty"`
	BgColor     *CT_Color `xml:"bgColor,omitempty"`
}

// CT_GradientFill represents a gradient fill with optional gradient stops.
type CT_GradientFill struct {
	Type   string            `xml:"type,attr,omitempty"`
	Degree *float64          `xml:"degree,attr,omitempty"`
	Left   *float64          `xml:"left,attr,omitempty"`
	Right  *float64          `xml:"right,attr,omitempty"`
	Top    *float64          `xml:"top,attr,omitempty"`
	Bottom *float64          `xml:"bottom,attr,omitempty"`
	Stop   []CT_GradientStop `xml:"stop"`
}

// CT_GradientStop represents a single gradient stop with a position and color.
type CT_GradientStop struct {
	Position float64  `xml:"position,attr"`
	Color    CT_Color `xml:"color"`
}

// --- Borders ---

// CT_Borders represents the borders element containing border definitions.
type CT_Borders struct {
	Count  *uint32     `xml:"count,attr,omitempty"`
	Border []CT_Border `xml:"border"`
}

// CT_Border represents a single border definition with optional edge properties.
type CT_Border struct {
	DiagonalUp   *bool        `xml:"diagonalUp,attr,omitempty"`
	DiagonalDown *bool        `xml:"diagonalDown,attr,omitempty"`
	Outline      *bool        `xml:"outline,attr,omitempty"`
	Left         *CT_BorderPr `xml:"left,omitempty"`
	Right        *CT_BorderPr `xml:"right,omitempty"`
	Top          *CT_BorderPr `xml:"top,omitempty"`
	Bottom       *CT_BorderPr `xml:"bottom,omitempty"`
	Diagonal     *CT_BorderPr `xml:"diagonal,omitempty"`
}

// CT_BorderPr represents a single border edge property with style and color.
type CT_BorderPr struct {
	Style string   `xml:"style,attr,omitempty"`
	Color *CT_Color `xml:"color,omitempty"`
}

// --- Cell Format Records ---

// CT_CellStyleXfs represents the cellStyleXfs element containing base cell format records.
type CT_CellStyleXfs struct {
	Count *uint32 `xml:"count,attr,omitempty"`
	Xf    []CT_Xf `xml:"xf"`
}

// CT_CellXfs represents the cellXfs element containing cell format records.
type CT_CellXfs struct {
	Count *uint32 `xml:"count,attr,omitempty"`
	Xf    []CT_Xf `xml:"xf"`
}

// CT_Xf represents a single cell format record (xf element).
type CT_Xf struct {
	NumFmtId          *uint32          `xml:"numFmtId,attr,omitempty"`
	FontId            *uint32          `xml:"fontId,attr,omitempty"`
	FillId            *uint32          `xml:"fillId,attr,omitempty"`
	BorderId          *uint32          `xml:"borderId,attr,omitempty"`
	XfId              *uint32          `xml:"xfId,attr,omitempty"`
	QuotePrefix       *bool            `xml:"quotePrefix,attr,omitempty"`
	PivotButton       *bool            `xml:"pivotButton,attr,omitempty"`
	ApplyNumberFormat *bool            `xml:"applyNumberFormat,attr,omitempty"`
	ApplyFont         *bool            `xml:"applyFont,attr,omitempty"`
	ApplyFill         *bool            `xml:"applyFill,attr,omitempty"`
	ApplyBorder       *bool            `xml:"applyBorder,attr,omitempty"`
	ApplyAlignment    *bool            `xml:"applyAlignment,attr,omitempty"`
	ApplyProtection   *bool            `xml:"applyProtection,attr,omitempty"`
	Alignment         *CT_CellAlignment  `xml:"alignment,omitempty"`
	Protection        *CT_CellProtection `xml:"protection,omitempty"`
}

// CT_CellAlignment represents cell alignment properties.
type CT_CellAlignment struct {
	Horizontal      string  `xml:"horizontal,attr,omitempty"`
	Vertical        string  `xml:"vertical,attr,omitempty"`
	TextRotation    *uint32 `xml:"textRotation,attr,omitempty"`
	WrapText        *bool   `xml:"wrapText,attr,omitempty"`
	Indent          *uint32 `xml:"indent,attr,omitempty"`
	RelativeIndent  *int32  `xml:"relativeIndent,attr,omitempty"`
	JustifyLastLine *bool   `xml:"justifyLastLine,attr,omitempty"`
	ShrinkToFit     *bool   `xml:"shrinkToFit,attr,omitempty"`
	ReadingOrder    *uint32 `xml:"readingOrder,attr,omitempty"`
}

// CT_CellProtection represents cell protection properties.
type CT_CellProtection struct {
	Locked *bool `xml:"locked,attr,omitempty"`
	Hidden *bool `xml:"hidden,attr,omitempty"`
}

// --- Cell Styles ---

// CT_CellStyles represents the cellStyles element containing named cell style definitions.
type CT_CellStyles struct {
	Count     *uint32        `xml:"count,attr,omitempty"`
	CellStyle []CT_CellStyle `xml:"cellStyle"`
}

// CT_CellStyle represents a single named cell style.
type CT_CellStyle struct {
	Name          string  `xml:"name,attr"`
	XfId          uint32  `xml:"xfId,attr"`
	BuiltinId     *uint32 `xml:"builtinId,attr,omitempty"`
	ILevel        *uint32 `xml:"iLevel,attr,omitempty"`
	Hidden        *bool   `xml:"hidden,attr,omitempty"`
	CustomBuiltin *bool   `xml:"customBuiltin,attr,omitempty"`
}

// --- Differential Formatting ---

// CT_Dxfs represents the dxfs element containing differential formatting records.
type CT_Dxfs struct {
	Count *uint32  `xml:"count,attr,omitempty"`
	Dxf   []CT_Dxf `xml:"dxf"`
}

// CT_Dxf represents a single differential formatting record.
type CT_Dxf struct {
	Font       *CT_Font           `xml:"font,omitempty"`
	NumFmt     *CT_NumFmt         `xml:"numFmt,omitempty"`
	Fill       *CT_Fill           `xml:"fill,omitempty"`
	Alignment  *CT_CellAlignment  `xml:"alignment,omitempty"`
	Border     *CT_Border         `xml:"border,omitempty"`
	Protection *CT_CellProtection `xml:"protection,omitempty"`
}

// --- Table Styles ---

// CT_TableStyles represents the tableStyles element containing table style definitions.
type CT_TableStyles struct {
	Count             *uint32         `xml:"count,attr,omitempty"`
	DefaultTableStyle string          `xml:"defaultTableStyle,attr,omitempty"`
	DefaultPivotStyle string          `xml:"defaultPivotStyle,attr,omitempty"`
	TableStyle        []CT_TableStyle `xml:"tableStyle"`
}

// CT_TableStyle represents a single table style definition.
type CT_TableStyle struct {
	Name              string                 `xml:"name,attr"`
	Pivot             *bool                  `xml:"pivot,attr,omitempty"`
	Table             *bool                  `xml:"table,attr,omitempty"`
	Count             *uint32                `xml:"count,attr,omitempty"`
	TableStyleElement []CT_TableStyleElement `xml:"tableStyleElement"`
}

// CT_TableStyleElement represents a single table style element.
type CT_TableStyleElement struct {
	Type  string  `xml:"type,attr"`
	Size  *uint32 `xml:"size,attr,omitempty"`
	DxfId *uint32 `xml:"dxfId,attr,omitempty"`
}

// --- Colors ---

// CT_Colors represents the colors element containing color definitions.
type CT_Colors struct {
	IndexedColors *CT_IndexedColors `xml:"indexedColors,omitempty"`
	MruColors     *CT_MRUColors     `xml:"mruColors,omitempty"`
}

// CT_IndexedColors represents the indexedColors element containing indexed color values.
type CT_IndexedColors struct {
	RgbColor []CT_RgbColor `xml:"rgbColor"`
}

// CT_RgbColor represents a single RGB color value.
type CT_RgbColor struct {
	Rgb string `xml:"rgb,attr,omitempty"`
}

// CT_MRUColors represents the mruColors element containing most recently used colors.
type CT_MRUColors struct {
	Color []CT_Color `xml:"color"`
}
