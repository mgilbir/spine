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

// Common namespace prefixes
const (
	PrefixDrawingML           = "a"
	PrefixPresentationML      = "p"
	PrefixSpreadsheetML       = "x"
	PrefixWordprocessingML    = "w"
	PrefixRelationships       = "r"
	PrefixDublinCore          = "dc"
	PrefixDcTerms             = "dcterms"
	PrefixContentTypes        = "ct"
)
