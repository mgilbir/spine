package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C145: converters round to the nearest unit instead of truncating.
func TestConverters_Round(t *testing.T) {
	if got := Inches(0.57); got != 521208 {
		t.Errorf("Inches(0.57) = %d, want 521208 (rounded, not truncated)", got)
	}
	if got := EMU(19049).ToPixels(); got != 2 { // 19049/9525 = 1.9999 -> 2
		t.Errorf("EMU(19049).ToPixels() = %d, want 2", got)
	}
}

// C40 + C99: a theme-colored gradient stop maps to a schemeClr, not black srgb;
// a system color yields a concrete color child rather than an empty solidFill.
func TestFill_ThemeAndSystemColorsNotLost(t *testing.T) {
	stop := GradientStop{Position: 0, Color: ThemeColorAccent1.ToColor()}
	var spPr SpPr
	NewGradientFill(45, stop, GradientStop{Position: 1, Color: ColorWhite}).ApplyToSpPr(&spPr)

	gs := spPr.GradFill.GsLst.Gs[0]
	if gs.SchemeClr == nil {
		t.Fatalf("theme gradient stop lost: SchemeClr is nil (gs=%+v)", gs)
	}
	if gs.SrgbClr != nil {
		t.Errorf("theme gradient stop wrongly rendered as srgb: %+v", gs.SrgbClr)
	}
	if gs.SchemeClr.Val != "accent1" {
		t.Errorf("SchemeClr.Val = %q, want accent1", gs.SchemeClr.Val)
	}

	// System color: solid fill must have a non-empty color child.
	sysColor := Color{Type: ColorTypeSystem, RGB: NewRGB(0x11, 0x22, 0x33)}
	sf := colorToSolidFill(sysColor)
	if sf.SrgbClr == nil || sf.SrgbClr.Val != "112233" {
		t.Errorf("system color produced empty/invalid solidFill: %+v", sf)
	}
}

// C99: WithAlpha clamps out-of-range percentages.
func TestWithAlpha_Clamps(t *testing.T) {
	if a := ColorRed.WithAlpha(150).Alpha; a != 100000 {
		t.Errorf("WithAlpha(150).Alpha = %d, want 100000", a)
	}
	if a := ColorRed.WithAlpha(-20).Alpha; a != 0 {
		t.Errorf("WithAlpha(-20).Alpha = %d, want 0", a)
	}
}

// C37: a:rtl carries its value in a val attribute, not element chardata.
func TestRtl_ValAttribute(t *testing.T) {
	var rpr RPr
	in := `<rPr xmlns="http://schemas.openxmlformats.org/drawingml/2006/main"><rtl val="1"/></rPr>`
	if err := xml.Unmarshal([]byte(in), &rpr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rpr.Rtl == nil || rpr.Rtl.Val != true {
		t.Fatalf("Rtl not parsed from val attr: %+v", rpr.Rtl)
	}
	out, err := xml.Marshal(&rpr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `<rtl`) || !strings.Contains(string(out), `val="true"`) {
		t.Errorf("rtl not marshaled with val attr: %s", out)
	}
	if strings.Contains(string(out), `>true</rtl>`) {
		t.Errorf("rtl marshaled as chardata instead of val attr: %s", out)
	}
}

// C38: a duotone with mixed color kinds preserves both colors.
func TestDuotone_PreservesColors(t *testing.T) {
	var d Duotone
	in := `<duotone xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<sysClr val="window" lastClr="FFFFFF"/><srgbClr val="000000"/></duotone>`
	if err := xml.Unmarshal([]byte(in), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(d.SysClr) != 1 || len(d.SrgbClr) != 1 {
		t.Errorf("duotone colors lost: sys=%d srgb=%d", len(d.SysClr), len(d.SrgbClr))
	}
}

// C39: a gradient stop using a scheme color round-trips.
func TestGs_SchemeColor(t *testing.T) {
	var gs Gs
	in := `<gs xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" pos="50000"><schemeClr val="accent2"/></gs>`
	if err := xml.Unmarshal([]byte(in), &gs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gs.SchemeClr == nil || gs.SchemeClr.Val != "accent2" {
		t.Errorf("scheme color stop lost: %+v", gs)
	}
}

// C96: GradFill flip attribute is preserved.
func TestGradFill_Flip(t *testing.T) {
	gf := GradFill{Flip: "xy"}
	out, err := xml.Marshal(&gf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `flip="xy"`) {
		t.Errorf("flip attr dropped: %s", out)
	}
}

// C97: ArcTo start/sweep angles are emitted even when zero.
func TestArcTo_ZeroAnglesEmitted(t *testing.T) {
	arc := ArcToXML{WR: 100, HR: 100, StAng: 0, SwAng: 0}
	out, err := xml.Marshal(&arc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `stAng="0"`) || !strings.Contains(string(out), `swAng="0"`) {
		t.Errorf("required zero angles dropped: %s", out)
	}
}

// C98: TcTxStyle.Font is a font collection (latin/ea/cs), not a font reference.
func TestTcTxStyle_FontCollection(t *testing.T) {
	var s TcTxStyle
	in := `<tcTxStyle xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<font><latin typeface="Calibri"/></font></tcTxStyle>`
	if err := xml.Unmarshal([]byte(in), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Font == nil || s.Font.Latin == nil || s.Font.Latin.Typeface != "Calibri" {
		t.Errorf("tcTxStyle font collection not parsed: %+v", s.Font)
	}
}

// buildFragment marshals v as an a:-namespace element through the production
// Builder and returns the serialized bytes.
func buildFragment(t *testing.T, localName string, v interface{}) string {
	t.Helper()
	b := xmlb.NewPresentationMLBuilder()
	b.MarshalElement(nsA, localName, v)
	return b.String()
}

// C178: single-EG_ColorChoice containers preserve all six color kinds through
// the production Builder instead of re-emitting a schema-invalid empty element.
func TestSingleColorContainers_AllColorKindsRoundTrip(t *testing.T) {
	const ns = "http://schemas.openxmlformats.org/drawingml/2006/main"
	tests := []struct {
		name  string
		local string
		input string
		v     interface{}
		want  string // exact color child that must survive
	}{
		{
			name:  "glow sysClr",
			local: "glow",
			input: `<glow xmlns="` + ns + `" rad="63500"><sysClr val="windowText" lastClr="000000"/></glow>`,
			v:     &GlowXML{},
			want:  `<a:sysClr val="windowText" lastClr="000000"/>`,
		},
		{
			name:  "innerShdw prstClr",
			local: "innerShdw",
			input: `<innerShdw xmlns="` + ns + `" blurRad="63500"><prstClr val="black"/></innerShdw>`,
			v:     &InnerShdw{},
			want:  `<a:prstClr val="black"/>`,
		},
		{
			name:  "prstShdw hslClr",
			local: "prstShdw",
			input: `<prstShdw xmlns="` + ns + `" prst="shdw13" dist="38100"><hslClr hue="14400000" sat="100000" lum="50000"/></prstShdw>`,
			v:     &PrstShdw{},
			want:  `<a:hslClr hue="14400000" sat="100000" lum="50000"/>`,
		},
		{
			name:  "clrRepl scrgbClr",
			local: "clrRepl",
			input: `<clrRepl xmlns="` + ns + `"><scrgbClr r="50000" g="25000" b="0"/></clrRepl>`,
			v:     &ClrRepl{},
			want:  `<a:scrgbClr r="50000" g="25000" b="0"/>`,
		},
		{
			name:  "alphaInv sysClr",
			local: "alphaInv",
			input: `<alphaInv xmlns="` + ns + `"><sysClr val="window" lastClr="FFFFFF"/></alphaInv>`,
			v:     &AlphaInv{},
			want:  `<a:sysClr val="window" lastClr="FFFFFF"/>`,
		},
		{
			name:  "buClr hslClr",
			local: "pPr",
			input: `<pPr xmlns="` + ns + `"><buClr><hslClr hue="14400000" sat="100000" lum="50000"/></buClr></pPr>`,
			v:     &PPr{},
			want:  `<a:buClr><a:hslClr hue="14400000" sat="100000" lum="50000"/></a:buClr>`,
		},
		{
			name:  "tcTxStyle prstClr",
			local: "tcTxStyle",
			input: `<tcTxStyle xmlns="` + ns + `"><fontRef idx="minor"/><prstClr val="black"/></tcTxStyle>`,
			v:     &TcTxStyle{},
			want:  `<a:prstClr val="black"/>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := xml.Unmarshal([]byte(tt.input), tt.v); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			out := buildFragment(t, tt.local, tt.v)
			if !strings.Contains(out, tt.want) {
				t.Errorf("color child lost on Builder re-marshal:\n got: %s\nwant substring: %s", out, tt.want)
			}
		})
	}
}
