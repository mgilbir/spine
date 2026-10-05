package docxrender

import (
	"context"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/render"
)

// wordTestWordStyles follows what Word writes into a new document: theme
// fonts and a default paragraph spacing in the defaults, linked heading styles
// with theme colours, a hyperlink character style, and a table style that the
// body does not use.
const wordTestWordStyles = `<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:asciiTheme="minorHAnsi" w:eastAsiaTheme="minorHAnsi" w:hAnsiTheme="minorHAnsi" w:cstheme="minorBidi"/><w:sz w:val="22"/><w:szCs w:val="22"/><w:lang w:val="en-US" w:eastAsia="en-US" w:bidi="ar-SA"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>
<w:latentStyles w:defLockedState="0" w:defUIPriority="99" w:defSemiHidden="0" w:defUnhideWhenUsed="0" w:defQFormat="0" w:count="376"><w:lsdException w:name="Normal" w:uiPriority="0" w:qFormat="1"/></w:latentStyles>
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>
<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/><w:uiPriority w:val="1"/><w:semiHidden/><w:unhideWhenUsed/></w:style>
<w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:name w:val="Normal Table"/><w:uiPriority w:val="99"/><w:semiHidden/><w:unhideWhenUsed/><w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:link w:val="Heading1Char"/><w:uiPriority w:val="9"/><w:qFormat/><w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="240" w:after="0"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:rFonts w:asciiTheme="majorHAnsi" w:eastAsiaTheme="majorEastAsia" w:hAnsiTheme="majorHAnsi" w:cstheme="majorBidi"/><w:color w:val="2F5496" w:themeColor="accent1" w:themeShade="BF"/><w:sz w:val="32"/><w:szCs w:val="32"/></w:rPr></w:style>
<w:style w:type="character" w:customStyle="1" w:styleId="Heading1Char"><w:name w:val="Heading 1 Char"/><w:basedOn w:val="DefaultParagraphFont"/><w:link w:val="Heading1"/><w:uiPriority w:val="9"/><w:rPr><w:rFonts w:asciiTheme="majorHAnsi" w:eastAsiaTheme="majorEastAsia" w:hAnsiTheme="majorHAnsi" w:cstheme="majorBidi"/><w:color w:val="2F5496" w:themeColor="accent1" w:themeShade="BF"/><w:sz w:val="32"/><w:szCs w:val="32"/></w:rPr></w:style>
<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:basedOn w:val="DefaultParagraphFont"/><w:uiPriority w:val="99"/><w:unhideWhenUsed/><w:rPr><w:color w:val="0563C1" w:themeColor="hyperlink"/><w:u w:val="single"/></w:rPr></w:style>`

const wordTestWordTheme = `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme"><a:themeElements><a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4><a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink></a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme></a:themeElements></a:theme>`

const wordTestWordSettings = `<w:zoom w:percent="100"/><w:proofState w:spelling="clean" w:grammar="clean"/><w:defaultTabStop w:val="720"/><w:characterSpacingControl w:val="doNotCompress"/><w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat><w:rsids><w:rsidRoot w:val="00AB1234"/></w:rsids><w:themeFontLang w:val="en-US"/><w:clrSchemeMapping w:bg1="light1" w:t1="dark1" w:bg2="light2" w:t2="dark2" w:accent1="accent1" w:accent2="accent2" w:accent3="accent3" w:accent4="accent4" w:accent5="accent5" w:accent6="accent6" w:hyperlink="hyperlink" w:followedHyperlink="followedHyperlink"/><w:decimalSymbol w:val="."/><w:listSeparator w:val=","/>`

func TestARealisticDocumentDrawsInStrictMode(t *testing.T) {
	letter := `<w:sectPr w:rsidR="00AB1234"><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/><w:cols w:space="720"/><w:docGrid w:linePitch="360"/></w:sectPr>`
	lorem := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 12)
	body := `<w:p w:rsidR="00AB1234" w:rsidRDefault="00AB1234" w14:paraId="1A2B3C4D"><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:bookmarkStart w:id="0" w:name="_Toc1"/><w:r><w:t>Introduction</w:t></w:r><w:bookmarkEnd w:id="0"/></w:p>` +
		`<w:p><w:r><w:rPr><w:noProof/></w:rPr><w:t xml:space="preserve">` + lorem + `</w:t></w:r><w:proofErr w:type="spellStart"/><w:r><w:t>wrongg</w:t></w:r><w:proofErr w:type="spellEnd"/></w:p>` +
		`<w:p><w:pPr><w:jc w:val="both"/></w:pPr><w:r><w:t xml:space="preserve">See </w:t></w:r><w:hyperlink r:id="rId5" w:history="1"><w:r><w:rPr><w:rStyle w:val="Hyperlink"/></w:rPr><w:t>the site</w:t></w:r></w:hyperlink><w:r><w:t xml:space="preserve"> and page </w:t></w:r><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:rPr><w:noProof/></w:rPr><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:r><w:t>.</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:pStyle w:val="Normal"/><w:rPr><w:b/></w:rPr></w:pPr></w:p>` + letter
	doc := wordTestDoc(t, body, wordTestParts{styles: wordTestWordStyles, settings: wordTestWordSettings, theme: wordTestWordTheme})
	face, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	var requests []render.FontRequest
	opts := render.Options{Limits: render.Limits{MaxShapeWork: 1 << 30}, Fonts: func(_ context.Context, r render.FontRequest) (*shape.Face, error) {
		requests = append(requests, r)
		return face, nil
	}}
	pages, err := Prepare(context.Background(), doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	if pages.Count() != 1 {
		t.Fatalf("%d pages", pages.Count())
	}
	// Headings use the major theme font, body text the minor one.
	families := map[string]bool{}
	for _, r := range requests {
		families[r.Family] = true
	}
	if !families["Calibri Light"] || !families["Calibri"] {
		t.Errorf("theme fonts: %+v", requests)
	}
	// The heading is Word's 2F5496 and the link its Hyperlink blue; the PAGE
	// field draws its cached result and not its instruction.
	var heading, link, field bool
	var all strings.Builder
	for _, tx := range pages.texts(1) {
		all.WriteString(tx.text)
	}
	if v, ok := pages.textOf(1, "Introduction"); ok {
		heading = rgbOf(v.Color) == "2f5496" && v.Size.Px() > 20
	}
	if v, ok := pages.textOf(1, "site"); ok {
		link = rgbOf(v.Color) == "0563c1"
	}
	field = strings.Contains(all.String(), "page 1.") && !strings.Contains(all.String(), "PAGE")
	if !heading || !link || !field {
		t.Errorf("heading %v link %v field %v in %q", heading, link, field, all.String())
	}
}

func TestLibraryBuiltDocumentsDraw(t *testing.T) {
	d := docx.Create()
	sec := d.DefaultSection()
	sec.SetPageSize(595, 842)
	sec.SetMargins(docx.PageMargins{Top: 72, Bottom: 72, Left: 72, Right: 72})
	d.AddHeading("Title", 1)
	d.AddParagraphWithText("Some body text that is long enough to wrap around the page when set at its size, and then some more of it.")
	r := d.AddParagraph().AddRun()
	r.SetText("bold coloured")
	r.SetBold(true)
	r.SetColor("336699")
	face, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Limits: render.Limits{MaxShapeWork: 1 << 30}, Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }}
	pages, err := Prepare(context.Background(), d, opts)
	if err != nil {
		t.Fatal(err)
	}
	if pages.Count() != 1 || len(pages.texts(1)) == 0 {
		t.Fatalf("%d pages", pages.Count())
	}
}
