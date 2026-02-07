package oxml

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mgilbir/spine/spec/spectest"
)

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
	spectest.TestUnmarshalExamples(t, examples, wmlTypeMap, spectest.WrapWML)
}

func TestWML_SpecExamples_RoundTrip(t *testing.T) {
	examples := spectest.LoadExamples(t, wmlTestdataPath())
	spectest.TestRoundTripExamples(t, examples, wmlTypeMap, spectest.WrapWML)
}
