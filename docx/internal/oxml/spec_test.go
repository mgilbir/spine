package oxml

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mgilbir/spine/common/omml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/spec/spectest"
)

// Dublin Core / OPC property leaf element wrappers for spec example testing.
// The spec examples use display names (Creator, Title, etc.) rather than the
// actual namespaced XML element names (dc:creator, dc:title, etc.).
// These are xsd:string or xsd:int leaf elements from Dublin Core (DCES),
// Dublin Core Terms, and OPC extended properties.
type specStringElement struct {
	Value string `xml:",chardata"`
}

type specIntElement struct {
	Value int32 `xml:",chardata"`
}

// specRawElement captures inner content as a raw string. Used for HTML
// fragments in spec examples — these are illustrative XHTML showing the
// source HTML that produces the corresponding WML output.
type specRawElement struct {
	Inner string `xml:",innerxml"`
}

var wmlTypeMap = map[string]reflect.Type{
	// Root document types (with XMLName)
	"document":  reflect.TypeOf(CT_Document{}),
	"styles":    reflect.TypeOf(CT_Styles{}),
	"numbering": reflect.TypeOf(CT_Numbering{}),
	"settings":  reflect.TypeOf(CT_Settings{}),
	"comments":  reflect.TypeOf(CT_Comments{}),
	"footnotes": reflect.TypeOf(CT_Footnotes{}),
	"endnotes":  reflect.TypeOf(CT_Endnotes{}),
	"hdr":       reflect.TypeOf(CT_HdrFtr{}),
	"ftr":       reflect.TypeOf(CT_HdrFtr{}),

	// Paragraph and run types
	"body": reflect.TypeOf(CT_Body{}),
	"p":    reflect.TypeOf(CT_P{}),
	"pPr":  reflect.TypeOf(CT_PPr{}),
	"r":    reflect.TypeOf(CT_R{}),
	"rPr":  reflect.TypeOf(CT_RPr{}),
	"t":    reflect.TypeOf(CT_Text{}),
	"br":   reflect.TypeOf(CT_Br{}),
	"sym":  reflect.TypeOf(CT_Sym{}),

	// Table types
	"tbl":        reflect.TypeOf(CT_Tbl{}),
	"tblPr":      reflect.TypeOf(CT_TblPr{}),
	"tblGrid":    reflect.TypeOf(CT_TblGrid{}),
	"gridCol":    reflect.TypeOf(CT_GridCol{}),
	"tr":         reflect.TypeOf(CT_Tr{}),
	"trPr":       reflect.TypeOf(CT_TrPr{}),
	"tc":         reflect.TypeOf(CT_Tc{}),
	"tcPr":       reflect.TypeOf(CT_TcPr{}),
	"tblStylePr": reflect.TypeOf(CT_TblStylePr{}),

	// Numbering types
	"abstractNum": reflect.TypeOf(CT_AbstractNum{}),
	"num":         reflect.TypeOf(CT_Num{}),
	"lvl":         reflect.TypeOf(CT_Lvl{}),
	"lvlText":     reflect.TypeOf(CT_LvlText{}),
	"lvlOverride": reflect.TypeOf(CT_NumLvl{}),

	// Section types
	"sectPr":    reflect.TypeOf(CT_SectPr{}),
	"pgSz":      reflect.TypeOf(CT_PgSz{}),
	"pgMar":     reflect.TypeOf(CT_PgMar{}),
	"pgBorders": reflect.TypeOf(CT_PgBorders{}),
	"pgNumType": reflect.TypeOf(CT_PgNumType{}),
	"cols":      reflect.TypeOf(CT_Columns{}),
	"col":       reflect.TypeOf(CT_Column{}),
	"docGrid":   reflect.TypeOf(CT_DocGrid{}),
	"lnNumType": reflect.TypeOf(CT_LnNumType{}),

	// Style types
	"style":        reflect.TypeOf(CT_Style{}),
	"docDefaults":  reflect.TypeOf(CT_DocDefaults{}),
	"rPrDefault":   reflect.TypeOf(CT_RPrDefault{}),
	"pPrDefault":   reflect.TypeOf(CT_PPrDefault{}),
	"latentStyles": reflect.TypeOf(CT_LatentStyles{}),
	"lsdException": reflect.TypeOf(CT_LsdException{}),

	// Comment / footnote types
	"comment":  reflect.TypeOf(CT_Comment{}),
	"footnote": reflect.TypeOf(CT_FtnEdn{}),
	"endnote":  reflect.TypeOf(CT_FtnEdn{}),

	// SDT types
	"sdt":        reflect.TypeOf(CT_SdtBlock{}),
	"sdtPr":      reflect.TypeOf(CT_SdtPr{}),
	"sdtEndPr":   reflect.TypeOf(CT_SdtPr{}),
	"sdtContent": reflect.TypeOf(CT_SdtContentBlock{}),

	// Bookmark types
	"bookmarkStart":     reflect.TypeOf(CT_BookmarkStart{}),
	"bookmarkEnd":       reflect.TypeOf(CT_BookmarkEnd{}),
	"commentRangeStart": reflect.TypeOf(CT_CommentRangeStart{}),
	"permStart":         reflect.TypeOf(CT_PermStart{}),
	"proofErr":          reflect.TypeOf(CT_ProofErr{}),

	// Field types
	"hyperlink": reflect.TypeOf(CT_Hyperlink{}),
	"fldSimple": reflect.TypeOf(CT_SimpleField{}),
	"fldChar":   reflect.TypeOf(CT_FldChar{}),

	// Track change types
	"ins":          reflect.TypeOf(CT_RunTrackChange{}),
	"del":          reflect.TypeOf(CT_RunTrackChange{}),
	"rPrChange":    reflect.TypeOf(CT_RPrChange{}),
	"pPrChange":    reflect.TypeOf(CT_PPrChange{}),
	"sectPrChange": reflect.TypeOf(CT_SectPrChange{}),
	"tblPrChange":  reflect.TypeOf(CT_TblPrChange{}),
	"trPrChange":   reflect.TypeOf(CT_TrPrChange{}),
	"tcPrChange":   reflect.TypeOf(CT_TcPrChange{}),

	// Property types
	"b":          reflect.TypeOf(CT_OnOff{}),
	"i":          reflect.TypeOf(CT_OnOff{}),
	"u":          reflect.TypeOf(CT_Underline{}),
	"color":      reflect.TypeOf(CT_Color{}),
	"sz":         reflect.TypeOf(CT_HpsMeasure{}),
	"szCs":       reflect.TypeOf(CT_HpsMeasure{}),
	"highlight":  reflect.TypeOf(CT_Highlight{}),
	"shd":        reflect.TypeOf(CT_Shd{}),
	"rFonts":     reflect.TypeOf(CT_Fonts{}),
	"lang":       reflect.TypeOf(CT_Lang{}),
	"spacing":    reflect.TypeOf(CT_Spacing{}),
	"ind":        reflect.TypeOf(CT_Ind{}),
	"jc":         reflect.TypeOf(CT_Jc{}),
	"pBdr":       reflect.TypeOf(CT_PBdr{}),
	"tblBorders": reflect.TypeOf(CT_TblBorders{}),
	"tcBorders":  reflect.TypeOf(CT_TcBorders{}),
	"tabs":       reflect.TypeOf(CT_Tabs{}),
	"tab":        reflect.TypeOf(CT_TabStop{}),
	"numPr":      reflect.TypeOf(CT_NumPr{}),
	"framePr":    reflect.TypeOf(CT_FramePr{}),

	// Border elements
	"top":    reflect.TypeOf(CT_Border{}),
	"bottom": reflect.TypeOf(CT_Border{}),
	"left":   reflect.TypeOf(CT_Border{}),
	"right":  reflect.TypeOf(CT_Border{}),

	// Footnote properties
	"footnotePr": reflect.TypeOf(CT_FtnProps{}),
	"endnotePr":  reflect.TypeOf(CT_EdnProps{}),

	// Drawing
	"drawing": reflect.TypeOf(CT_Drawing{}),

	// Background
	"background": reflect.TypeOf(CT_Background{}),

	// Section properties
	"paperSrc": reflect.TypeOf(CT_PaperSrc{}),

	// Simple settings properties
	"name":   reflect.TypeOf(CT_String{}),
	"type":   reflect.TypeOf(CT_String{}),
	"vAlign": reflect.TypeOf(CT_String{}),

	// CT_OnOff settings properties
	"embedTrueTypeFonts":                  reflect.TypeOf(CT_OnOff{}),
	"alignBordersAndEdges":                reflect.TypeOf(CT_OnOff{}),
	"alwaysMergeEmptyNamespace":           reflect.TypeOf(CT_OnOff{}),
	"alwaysShowPlaceholderText":           reflect.TypeOf(CT_OnOff{}),
	"autoFormatOverride":                  reflect.TypeOf(CT_OnOff{}),
	"bordersDoNotSurroundFooter":          reflect.TypeOf(CT_OnOff{}),
	"bordersDoNotSurroundHeader":          reflect.TypeOf(CT_OnOff{}),
	"displayBackgroundShape":              reflect.TypeOf(CT_OnOff{}),
	"doNotAutoCompressPictures":           reflect.TypeOf(CT_OnOff{}),
	"doNotDemarcateInvalidXml":            reflect.TypeOf(CT_OnOff{}),
	"doNotDisplayPageBoundaries":          reflect.TypeOf(CT_OnOff{}),
	"doNotEmbedSmartTags":                 reflect.TypeOf(CT_OnOff{}),
	"doNotHyphenateCaps":                  reflect.TypeOf(CT_OnOff{}),
	"doNotIncludeSubdocsInStats":          reflect.TypeOf(CT_OnOff{}),
	"doNotShadeFormData":                  reflect.TypeOf(CT_OnOff{}),
	"doNotTrackMoves":                     reflect.TypeOf(CT_OnOff{}),
	"doNotUseMarginsForDrawingGridOrigin": reflect.TypeOf(CT_OnOff{}),
	"doNotValidateAgainstSchema":          reflect.TypeOf(CT_OnOff{}),
	"formsDesign":                         reflect.TypeOf(CT_OnOff{}),
	"hideGrammaticalErrors":               reflect.TypeOf(CT_OnOff{}),
	"hideSpellingErrors":                  reflect.TypeOf(CT_OnOff{}),
	"mirrorMargins":                       reflect.TypeOf(CT_OnOff{}),
	"noPunctuationKerning":                reflect.TypeOf(CT_OnOff{}),
	"printFormsData":                      reflect.TypeOf(CT_OnOff{}),
	"printFractionalCharacterWidth":       reflect.TypeOf(CT_OnOff{}),
	"printPostScriptOverText":             reflect.TypeOf(CT_OnOff{}),
	"removePersonalInformation":           reflect.TypeOf(CT_OnOff{}),
	"saveFormsData":                       reflect.TypeOf(CT_OnOff{}),
	"saveInvalidXml":                      reflect.TypeOf(CT_OnOff{}),
	"savePreviewPicture":                  reflect.TypeOf(CT_OnOff{}),
	"showEnvelope":                        reflect.TypeOf(CT_OnOff{}),
	"showXMLTags":                         reflect.TypeOf(CT_OnOff{}),
	"strictFirstAndLastChars":             reflect.TypeOf(CT_OnOff{}),
	"styleLockQFSet":                      reflect.TypeOf(CT_OnOff{}),
	"styleLockTheme":                      reflect.TypeOf(CT_OnOff{}),
	"updateFields":                        reflect.TypeOf(CT_OnOff{}),
	"doNotSuppressBlankLines":             reflect.TypeOf(CT_OnOff{}),
	"viewMergedData":                      reflect.TypeOf(CT_OnOff{}),
	"useXSLTWhenSaving":                   reflect.TypeOf(CT_OnOff{}),
	"optimizeForBrowser":                  reflect.TypeOf(CT_OnOff{}),

	// CT_String settings properties
	"characterSpacingControl": reflect.TypeOf(CT_String{}),
	"documentType":            reflect.TypeOf(CT_String{}),
	"view":                    reflect.TypeOf(CT_String{}),
	"stylePaneSortMethod":     reflect.TypeOf(CT_String{}),
	"dataType":                reflect.TypeOf(CT_String{}),
	"mainDocumentType":        reflect.TypeOf(CT_String{}),
	"addressFieldName":        reflect.TypeOf(CT_String{}),
	"destination":             reflect.TypeOf(CT_String{}),
	"decimalSymbol":           reflect.TypeOf(CT_String{}),
	"listSeparator":           reflect.TypeOf(CT_String{}),
	"tblOverlap":              reflect.TypeOf(CT_String{}),

	// CT_DecimalNumber settings properties
	"consecutiveHyphenLimit":            reflect.TypeOf(CT_DecimalNumber{}),
	"displayHorizontalDrawingGridEvery": reflect.TypeOf(CT_DecimalNumber{}),
	"displayVerticalDrawingGridEvery":   reflect.TypeOf(CT_DecimalNumber{}),
	"summaryLength":                     reflect.TypeOf(CT_String{}),
	"colDelim":                          reflect.TypeOf(CT_DecimalNumber{}),
	"numIdMacAtCleanup":                 reflect.TypeOf(CT_DecimalNumber{}),

	// CT_TwipsMeasure settings properties
	"drawingGridHorizontalSpacing": reflect.TypeOf(CT_TwipsMeasure{}),
	"drawingGridVerticalSpacing":   reflect.TypeOf(CT_TwipsMeasure{}),
	"hyphenationZone":              reflect.TypeOf(CT_TwipsMeasure{}),
	"defaultTabStop":               reflect.TypeOf(CT_TwipsMeasure{}),

	// CT_Empty
	"forceUpgrade": reflect.TypeOf(CT_Empty{}),

	// New container types (Batch 2)
	"compat":                reflect.TypeOf(CT_Compat{}),
	"compatSetting":         reflect.TypeOf(CT_CompatSetting{}),
	"clrSchemeMapping":      reflect.TypeOf(CT_ClrSchemeMapping{}),
	"webSettings":           reflect.TypeOf(CT_WebSettings{}),
	"revisionView":          reflect.TypeOf(CT_RevisionView{}),
	"documentProtection":    reflect.TypeOf(CT_DocumentProtection{}),
	"captions":              reflect.TypeOf(CT_Captions{}),
	"caption":               reflect.TypeOf(CT_Caption{}),
	"autoCaption":           reflect.TypeOf(CT_AutoCaption{}),
	"docVars":               reflect.TypeOf(CT_DocVars{}),
	"rsids":                 reflect.TypeOf(CT_Rsids{}),
	"proofState":            reflect.TypeOf(CT_ProofState{}),
	"readModeInkLockDown":   reflect.TypeOf(CT_ReadModeInkLockDown{}),
	"zoom":                  reflect.TypeOf(CT_Zoom{}),
	"writeProtection":       reflect.TypeOf(CT_WriteProtection{}),
	"stylePaneFormatFilter": reflect.TypeOf(CT_StylePaneFormatFilter{}),

	// Form field types
	"ffData":  reflect.TypeOf(CT_FFData{}),
	"control": reflect.TypeOf(CT_Control{}),

	// Custom XML types
	"customXml":   reflect.TypeOf(CT_CustomXml{}),
	"customXmlPr": reflect.TypeOf(CT_CustomXmlPr{}),

	// Font table types
	"font": reflect.TypeOf(CT_Font{}),

	// Table property exceptions
	"tblPrEx": reflect.TypeOf(CT_TblPrEx{}),

	// Header/footer references
	"headerReference": reflect.TypeOf(CT_HeaderReference{}),
	"footerReference": reflect.TypeOf(CT_HeaderReference{}),

	// Mail merge types (mailmerge.go)
	"mailMerge":       reflect.TypeOf(CT_MailMerge{}),
	"odso":            reflect.TypeOf(CT_Odso{}),
	"fieldMapData":    reflect.TypeOf(CT_OdsoFieldMapData{}),
	"recipientData":   reflect.TypeOf(CT_RecipientData{}),
	"recipients":      reflect.TypeOf(CT_Recipients{}),
	"saveThroughXslt": reflect.TypeOf(CT_SaveThroughXslt{}),
	"query":           reflect.TypeOf(CT_String{}),
	"connectString":   reflect.TypeOf(CT_String{}),
	"checkErrors":     reflect.TypeOf(CT_DecimalNumber{}),
	"udl":             reflect.TypeOf(CT_String{}),
	"fHdr":            reflect.TypeOf(CT_OnOff{}),

	// Ruby / phonetic guide types (ruby.go)
	"rubyPr":   reflect.TypeOf(CT_RubyPr{}),
	"rubyBase": reflect.TypeOf(CT_RubyContent{}),
	"rt":       reflect.TypeOf(CT_RubyContent{}),

	// Glossary / building block types (docparts.go)
	"glossaryDocument": reflect.TypeOf(CT_GlossaryDocument{}),
	"docPart":          reflect.TypeOf(CT_DocPart{}),
	"docPartPr":        reflect.TypeOf(CT_DocPartPr{}),
	"gallery":          reflect.TypeOf(CT_String{}),

	// OLE object types (objects.go)
	"object":      reflect.TypeOf(CT_Object{}),
	"objectEmbed": reflect.TypeOf(CT_ObjectEmbed{}),
	"objectLink":  reflect.TypeOf(CT_ObjectLink{}),

	// Frameset types (frameset.go)
	"frameset": reflect.TypeOf(CT_Frameset{}),
	"frame":    reflect.TypeOf(CT_Frame{}),

	// Additional settings types (settings_types.go)
	"activeWritingStyle": reflect.TypeOf(CT_ActiveWritingStyle{}),
	"attachedTemplate":   reflect.TypeOf(CT_Rel{}),
	"attachedSchema":     reflect.TypeOf(CT_String{}),
	"themeFontLang":      reflect.TypeOf(CT_ThemeFontLang{}),
	"shapeDefaults":      reflect.TypeOf(CT_ShapeDefaults{}),
	"hdrShapeDefaults":   reflect.TypeOf(CT_ShapeDefaults{}),
	"defaultTableStyle":  reflect.TypeOf(CT_String{}),
	"clickAndTypeStyle":  reflect.TypeOf(CT_String{}),
	"date":               reflect.TypeOf(CT_SdtDate{}),
	"smartTagType":       reflect.TypeOf(CT_SmartTagType{}),
	"smartTagPr":         reflect.TypeOf(CT_SmartTagPr{}),

	// Run types (run_types.go)
	"ptab":   reflect.TypeOf(CT_Ptab{}),
	"legacy": reflect.TypeOf(CT_Legacy{}),
	"dir":    reflect.TypeOf(CT_DirContentRun{}),
	"bdo":    reflect.TypeOf(CT_BdoContentRun{}),

	// Numbering types (run_types.go)
	"numPicBullet": reflect.TypeOf(CT_NumPicBullet{}),

	// Font table types (run_types.go)
	"fonts": reflect.TypeOf(CT_FontsList{}),
	"pitch": reflect.TypeOf(CT_String{}),

	// Table width (properties.go)
	"tblW": reflect.TypeOf(CT_TblWidth{}),

	// Div types (run_types.go)
	"div": reflect.TypeOf(CT_Div{}),

	// Dublin Core elements (DCES) — dc:creator, dc:title, etc.
	// Spec examples use PascalCase display names without namespace prefixes.
	"Creator":     reflect.TypeOf(specStringElement{}),
	"Title":       reflect.TypeOf(specStringElement{}),
	"Subject":     reflect.TypeOf(specStringElement{}),
	"Description": reflect.TypeOf(specStringElement{}),
	"Keywords":    reflect.TypeOf(specStringElement{}),

	// Dublin Core Terms — dcterms:created, dcterms:modified
	"DateCreated":  reflect.TypeOf(specStringElement{}),
	"DateModified": reflect.TypeOf(specStringElement{}),

	// OPC extended properties (docProps/app.xml)
	"TotalTime":      reflect.TypeOf(specIntElement{}),
	"Revision":       reflect.TypeOf(specStringElement{}),
	"LastPrinted":    reflect.TypeOf(specStringElement{}),
	"LastModifiedBy": reflect.TypeOf(specStringElement{}),

	// HTML fragments — illustrative XHTML showing source HTML that produces WML.
	// Inner content captured as raw string since it's not WML markup.
	"html": reflect.TypeOf(specRawElement{}),

	// Office Math (OMML) types — m: namespace elements that appear in WML examples.
	"f": reflect.TypeOf(omml.Fraction{}),
}

// wmlOutOfScope lists elements that appear in WML test data but are NOT WML types.
// These are from other namespaces (OPC, VML, MathML) and are tested in their
// respective test suites.
var wmlOutOfScope = map[string]string{
	// OPC package relationships
	"Relationships": "OPC package relationship",
	// VML shapes (tested in VML suite)
	"shape": "VML shape, tested in VML suite",
	// W3C MathML (different spec from Office Math)
	"math": "W3C MathML element, not Office Math (OMML)",
}

func wmlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "spec", "testdata", "wml_examples.json")
}

func TestWML_SpecExamples_WellFormed(t *testing.T) {
	examples := spectest.LoadExamples(t, wmlTestdataPath())
	spectest.TestWellFormedExamples(t, examples, spectest.WrapWML)
}

func TestWML_SpecExamples_Unmarshal(t *testing.T) {
	examples := spectest.LoadExamples(t, wmlTestdataPath())
	spectest.TestUnmarshalExamplesWithSkips(t, examples, wmlTypeMap, spectest.WrapWML, wmlOutOfScope)
}

func TestWML_SpecExamples_RoundTrip(t *testing.T) {
	examples := spectest.LoadExamples(t, wmlTestdataPath())
	marshalFn := func(v interface{}, rootElem string) ([]byte, error) {
		if raw, ok := v.(*specRawElement); ok {
			return []byte("<" + rootElem + ">" + raw.Inner + "</" + rootElem + ">"), nil
		}
		b := xmlb.NewWordprocessingMLBuilder()
		b.MarshalElement(xmlb.NSWordprocessingML, rootElem, v)
		return b.Bytes(), nil
	}
	spectest.TestRoundTripExamplesWithSkips(t, examples, wmlTypeMap, spectest.WrapWML, marshalFn, wmlOutOfScope)
}
