// Package xml provides XML namespace constants for OOXML documents.
package xml

// OOXML namespace URIs
const (
	// Core namespaces
	NSRelationships       = "http://schemas.openxmlformats.org/package/2006/relationships"
	NSContentTypes        = "http://schemas.openxmlformats.org/package/2006/content-types"
	NSCoreProperties      = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties"
	NSExtendedProperties  = "http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"

	// DrawingML namespaces
	NSDrawingML           = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NSDrawingMLChart      = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	NSDrawingMLDiagram    = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
	NSDrawingMLPicture    = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	NSDrawingMLSpreadsheet = "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"
	NSDrawingMLWordprocessing = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"

	// PresentationML namespaces
	NSPresentationML      = "http://schemas.openxmlformats.org/presentationml/2006/main"
	NSPresentationRels    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

	// SpreadsheetML namespaces
	NSSpreadsheetML       = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"

	// WordprocessingML namespaces
	NSWordprocessingML    = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

	// Dublin Core namespaces
	NSDublinCore          = "http://purl.org/dc/elements/1.1/"
	NSDcTerms             = "http://purl.org/dc/terms/"
	NSDcmiType            = "http://purl.org/dc/dcmitype/"

	// XML Schema namespaces
	NSXmlSchema           = "http://www.w3.org/2001/XMLSchema"
	NSXmlSchemaInstance   = "http://www.w3.org/2001/XMLSchema-instance"
)

// Microsoft Office extension namespaces
const (
	NSDrawing2010        = "http://schemas.microsoft.com/office/drawing/2010/main"
	NSDrawing2014        = "http://schemas.microsoft.com/office/drawing/2014/main"
	NSDrawingSVG2016     = "http://schemas.microsoft.com/office/drawing/2016/SVG/main"
	NSThemeML2012        = "http://schemas.microsoft.com/office/thememl/2012/main"
	NSDrawingDiagram2008 = "http://schemas.microsoft.com/office/drawing/2008/diagram"
	NSPowerPoint2010     = "http://schemas.microsoft.com/office/powerpoint/2010/main"
	NSPowerPoint2012     = "http://schemas.microsoft.com/office/powerpoint/2012/main"
	NSWord2010           = "http://schemas.microsoft.com/office/word/2010/wordml"
	NSWord2012           = "http://schemas.microsoft.com/office/word/2012/wordml"

	// Markup Compatibility namespace (ECMA-376 Part 3)
	NSMarkupCompatibility = "http://schemas.openxmlformats.org/markup-compatibility/2006"
)

// Common namespace prefixes
const (
	PrefixDrawingML           = "a"
	PrefixPresentationML      = "p"
	PrefixSpreadsheetML       = "x"
	PrefixWordprocessingML    = "w"
	PrefixRelationships       = "r"
	PrefixDrawingMLChart      = "c"
	PrefixDrawingMLDiagram    = "dgm"
	PrefixDublinCore          = "dc"
	PrefixDcTerms             = "dcterms"
	PrefixContentTypes        = "ct"
	PrefixDrawing2010         = "a14"
	PrefixDrawing2014         = "a16"
	PrefixDrawingSVG2016      = "asvg"
	PrefixThemeML2012         = "thm15"
	PrefixDrawingDiagram2008  = "dsp"
	PrefixPowerPoint2010      = "p14"
	PrefixPowerPoint2012      = "p15"
	PrefixWord2010            = "w14"
	PrefixWord2012            = "w15"
	PrefixMarkupCompatibility = "mc"
)

// ExtensionPrefixToNS maps mc:Choice/@Requires prefix names to namespace URIs.
var ExtensionPrefixToNS = map[string]string{
	"p14": NSPowerPoint2010,
	"p15": NSPowerPoint2012,
	"a14": NSDrawing2010,
	"a16": NSDrawing2014,
	"w14": NSWord2010,
	"w15": NSWord2012,
}

// Extension URI constants identify known extension types by their URI attribute.
const (
	ExtURICreationId     = "{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}"
	ExtURIColId          = "{9D8B030D-6E8A-4147-A177-3AD203B41FA5}"
	ExtURIRowId          = "{0D108BD9-81ED-4DB2-BD59-A6C34878D82A}"
	ExtURIUseLocalDpi    = "{28A0092B-C50C-407E-A947-70E740481C1C}"
	ExtURIShadowObscured = "{53640926-AAD7-44D8-BBD7-CCE9431645EC}"
	ExtURIHiddenFill     = "{909E8E84-426E-40DD-AFC4-6F175D3DCCD1}"
	ExtURIHiddenLine     = "{91240B29-F687-4F45-9708-019B960494DF}"
	ExtURIHiddenEffects  = "{AF507438-7753-43E0-B8FC-AC1667EBCBE1}"
	ExtURIImgProps       = "{BEBA8EAE-BF5A-486C-A8C5-ECC9F3942E4B}"
	ExtURISvgBlip        = "{96DAC541-7B7A-43D3-8B79-37D633B846F1}"
	ExtURIThemeFamily    = "{05A4C25C-085E-4340-85A3-A5531E510DB2}"
	ExtURIDataModelExt   = "http://schemas.microsoft.com/office/drawing/2008/diagram"

	// PresentationML extension URIs (p14 - PowerPoint 2010)
	ExtURIPMLCreationId        = "{BB962C8B-B14F-4D97-AF65-F5344CB8AC3E}"
	ExtURIPMLModId             = "{D42A27DB-BD31-4B8C-83A1-F6EECF244321}"
	ExtURIShowMediaCtrls       = "{2FDB2607-1784-4EEB-B798-7EB5836EED8A}"
	ExtURIDefaultImageDpi      = "{D31A062A-798A-4329-ABDD-BBA856620510}"
	ExtURIDiscardImageEditData = "{E76CE94A-603C-4142-B9EB-6D1370010A27}"
	ExtURILaserClr             = "{EC167BDD-8182-4AB7-AECC-EB403E3ABB37}"

	// PresentationML extension URIs (p15 - PowerPoint 2012)
	ExtURIPresenceInfo           = "{19B8F6BF-5375-455C-9EA6-DF929625EA0E}"
	ExtURISldGuideLst            = "{EFAFB233-063F-42B5-8137-9DF3F51BA10A}"
	ExtURISldGuideLstMaster      = "{27BBF7A9-308A-43DC-89C8-2F10F3537804}"
	ExtURISldGuideLstLayout      = "{DCECCB84-F9BA-43D5-87BE-67443E8EF086}"
	ExtURIChartTrackingRefBased  = "{FD5EFAAD-0ECE-453E-9831-46B23BE46B34}"
)
