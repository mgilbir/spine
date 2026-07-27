package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// themeWithExtensions is a minimal but realistic Office 2013+ theme: it carries
// the thm15:themeFamily extension every modern theme has, a custom color list,
// and an extLst on each of the nested types the model used to ignore.
const themeWithExtensions = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme"><a:themeElements><a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4><a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink><a:extLst><a:ext uri="{CLRSCHEME-EXT}"><x:custom xmlns:x="urn:x" v="1"/></a:ext></a:extLst></a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/><a:extLst><a:ext uri="{MAJORFONT-EXT}"><x:custom xmlns:x="urn:x" v="2"/></a:ext></a:extLst></a:majorFont><a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont><a:extLst><a:ext uri="{FONTSCHEME-EXT}"><x:custom xmlns:x="urn:x" v="3"/></a:ext></a:extLst></a:fontScheme><a:fmtScheme name="Office"><a:fillStyleLst><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="0"><a:schemeClr val="phClr"/></a:gs></a:gsLst></a:gradFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:gradFill rotWithShape="1"><a:gsLst><a:gs pos="100000"><a:schemeClr val="phClr"/></a:gs></a:gsLst></a:gradFill></a:fillStyleLst><a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:pattFill prst="pct5"><a:fgClr><a:schemeClr val="phClr"/></a:fgClr><a:bgClr><a:schemeClr val="phClr"/></a:bgClr></a:pattFill><a:noFill/><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme><a:extLst><a:ext uri="{BASESTYLES-EXT}"><x:custom xmlns:x="urn:x" v="4"/></a:ext></a:extLst></a:themeElements><a:objectDefaults><a:spDef><a:spPr/><a:bodyPr/><a:lstStyle/><a:extLst><a:ext uri="{SPDEF-EXT}"><x:custom xmlns:x="urn:x" v="5"/></a:ext></a:extLst></a:spDef><a:extLst><a:ext uri="{OBJDEF-EXT}"><x:custom xmlns:x="urn:x" v="6"/></a:ext></a:extLst></a:objectDefaults><a:extraClrSchemeLst/><a:custClrLst><a:custClr name="Brand Red"><a:srgbClr val="CC0000"/></a:custClr><a:custClr name="Brand Blue"><a:srgbClr val="0000CC"/></a:custClr></a:custClrLst><a:extLst><a:ext uri="{05A4C25C-085E-4340-85A3-A5531E510DB2}"><thm15:themeFamily xmlns:thm15="http://schemas.microsoft.com/office/thememl/2012/main" name="Office Theme" id="{62F939B6-93AF-4DB8-9C6B-D6C7DFDC589F}" vid="{4A3C46E8-61CC-4603-A589-7422A47A8E4A}"/></a:ext></a:extLst></a:theme>`

// TestThemeEditPreservesExtensions pins C374 at the model level: an edited
// theme re-serializes from the struct, so anything the struct cannot say is
// deleted. custClrLst, the theme extLst (thm15:themeFamily) and the extLst of
// every nested type must survive a setter.
func TestThemeEditPreservesExtensions(t *testing.T) {
	var theme Theme
	if err := xmlb.Unmarshal([]byte(themeWithExtensions), &theme); err != nil {
		t.Fatalf("parse: %v", err)
	}

	ed := NewThemeEditor(&theme, []byte(themeWithExtensions))
	ed.SetName("Renamed")
	if !ed.Modified() {
		t.Fatal("SetName did not mark the theme modified")
	}
	out, err := ed.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)

	for _, want := range []string{
		`thm15:themeFamily`,
		`a:custClrLst`,
		`name="Brand Red"`,
		`val="CC0000"`,
		`{CLRSCHEME-EXT}`,
		`{MAJORFONT-EXT}`,
		`{FONTSCHEME-EXT}`,
		`{BASESTYLES-EXT}`,
		`{SPDEF-EXT}`,
		`{OBJDEF-EXT}`,
		`name="Renamed"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("edited theme lost %s (C374)", want)
		}
	}
}

// TestThemeFillStyleListPositionalOrder pins C401: fillStyleLst and
// bgFillStyleLst are positional (a:fillRef/@idx indexes them), so re-emitting
// them regrouped by kind silently repoints every styled shape at a different
// fill.
func TestThemeFillStyleListPositionalOrder(t *testing.T) {
	var theme Theme
	if err := xmlb.Unmarshal([]byte(themeWithExtensions), &theme); err != nil {
		t.Fatalf("parse: %v", err)
	}
	fs := theme.ThemeElements.FmtScheme

	// The source interleaves kinds against field order in both lists.
	if got := len(fs.FillStyleLst.GradFill); got != 2 {
		t.Fatalf("gradFill count = %d, want 2", got)
	}

	ed := NewThemeEditor(&theme, []byte(themeWithExtensions))
	ed.SetName("Renamed")
	out, err := ed.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)

	fillLst := between(t, got, "<a:fillStyleLst>", "</a:fillStyleLst>")
	if order := fillKindOrder(t, fillLst); !equalStrings(order, []string{"gradFill", "solidFill", "gradFill"}) {
		t.Errorf("fillStyleLst re-emitted as %v, want [gradFill solidFill gradFill] (C401)", order)
	}
	// The positional identity, not just the kind: entry 0 is the pos="0"
	// gradient and entry 2 the pos="100000" one.
	if i0, i2 := strings.Index(fillLst, `pos="0"`), strings.Index(fillLst, `pos="100000"`); i0 < 0 || i2 < 0 || i0 > i2 {
		t.Errorf("gradient stops swapped positions: %s", fillLst)
	}

	bgLst := between(t, got, "<a:bgFillStyleLst>", "</a:bgFillStyleLst>")
	if order := fillKindOrder(t, bgLst); !equalStrings(order, []string{"pattFill", "noFill", "solidFill"}) {
		t.Errorf("bgFillStyleLst re-emitted as %v, want [pattFill noFill solidFill] (C401)", order)
	}
}

// TestThemeUnmodifiedRegeneratesIdentically checks that regenerating an
// untouched theme from the model reproduces the source part byte for byte, so
// the new capture did not merely relocate the loss.
func TestThemeUnmodifiedRegeneratesIdentically(t *testing.T) {
	var theme Theme
	if err := xmlb.Unmarshal([]byte(themeWithExtensions), &theme); err != nil {
		t.Fatalf("parse: %v", err)
	}
	ed := NewThemeEditor(&theme, []byte(themeWithExtensions))
	out, err := ed.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != themeWithExtensions {
		t.Errorf("regenerated theme is not byte-identical to its source:\n got %s\nwant %s", out, themeWithExtensions)
	}
}

// between returns the substring of s between the first open and the following
// close marker, failing the test when either is absent.
func between(t *testing.T, s, open, close string) string {
	t.Helper()
	i := strings.Index(s, open)
	if i < 0 {
		t.Fatalf("marker %q not found in output", open)
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		t.Fatalf("marker %q not found in output", close)
	}
	return rest[:j]
}

// fillKindOrder lists the EG_FillProperties element kinds in the order they
// appear at the top level of a fill style list fragment.
func fillKindOrder(t *testing.T, fragment string) []string {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(
		`<a:wrap xmlns:a="` + NsDrawingML + `">` + fragment + `</a:wrap>`))
	var out []string
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				if _, ok := fillChoiceNameKind[t.Name.Local]; ok {
					out = append(out, t.Name.Local)
				}
			}
		case xml.EndElement:
			depth--
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
