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

// C38: duotone colors are positional (dark then light); a mixed-kind pair must
// re-emit in document order through the production Builder, not grouped by kind.
func TestDuotone_PositionalOrderPreserved(t *testing.T) {
	var d Duotone
	in := `<duotone xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<prstClr val="black"/><srgbClr val="D9C3A5"/></duotone>`
	if err := xml.Unmarshal([]byte(in), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := buildFragment(t, "duotone", &d)
	want := `<a:duotone><a:prstClr val="black"/><a:srgbClr val="D9C3A5"/></a:duotone>`
	if out != want {
		t.Errorf("duotone order not preserved:\n got: %s\nwant: %s", out, want)
	}

	// encoding/xml path preserves the same order.
	enc, err := xml.Marshal(&d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if p, s := strings.Index(string(enc), "prstClr"), strings.Index(string(enc), "srgbClr"); p < 0 || s < 0 || p > s {
		t.Errorf("encoding/xml duotone order not preserved: %s", enc)
	}
}

// C38: a programmatically built duotone (no captured order) still emits its
// colors via the grouped fallback.
func TestDuotone_GroupedFallback(t *testing.T) {
	d := Duotone{
		SrgbClr:   []*SrgbClr{{Val: "000000"}},
		SchemeClr: []*SchemeClrTransform{{Val: "accent1"}},
	}
	out := buildFragment(t, "duotone", &d)
	if !strings.Contains(out, "srgbClr") || !strings.Contains(out, "schemeClr") {
		t.Errorf("constructed duotone lost colors: %s", out)
	}
}

// C95: all 28 EG_ColorTransform kinds round-trip, and arg-less kinds
// (comp/inv/gray/gamma/invGamma) are EMPTY complex types: no val attribute.
func TestSchemeClrTransform_AllKindsRoundTrip(t *testing.T) {
	var s SchemeClrTransform
	in := `<schemeClr xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" val="accent1">` +
		`<gray/><red val="50000"/><gamma/></schemeClr>`
	if err := xml.Unmarshal([]byte(in), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := buildFragment(t, "schemeClr", &s)
	want := `<a:schemeClr val="accent1"><a:gray/><a:red val="50000"/><a:gamma/></a:schemeClr>`
	if out != want {
		t.Errorf("transform round-trip mismatch:\n got: %s\nwant: %s", out, want)
	}

	// encoding/xml path: arg-less kinds carry no bogus val attribute either.
	enc, err := xml.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(enc), `gray val=`) || strings.Contains(string(enc), `gamma val=`) {
		t.Errorf("arg-less transform re-emitted with fabricated val: %s", enc)
	}
	if !strings.Contains(string(enc), `red val="50000"`) {
		t.Errorf("red transform lost on encoding/xml marshal: %s", enc)
	}
}

// C95: the full red/green/blue family and invGamma are modeled, not skipped.
func TestSchemeClrTransform_RGBFamily(t *testing.T) {
	var s SchemeClrTransform
	in := `<schemeClr xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" val="dk1">` +
		`<redMod val="1"/><redOff val="2"/><green val="3"/><greenMod val="4"/><greenOff val="5"/>` +
		`<blue val="6"/><blueMod val="7"/><blueOff val="8"/><invGamma/><comp/><inv/></schemeClr>`
	if err := xml.Unmarshal([]byte(in), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := buildFragment(t, "schemeClr", &s)
	want := `<a:schemeClr val="dk1"><a:redMod val="1"/><a:redOff val="2"/><a:green val="3"/>` +
		`<a:greenMod val="4"/><a:greenOff val="5"/><a:blue val="6"/><a:blueMod val="7"/>` +
		`<a:blueOff val="8"/><a:invGamma/><a:comp/><a:inv/></a:schemeClr>`
	if out != want {
		t.Errorf("rgb-family transforms mismatch:\n got: %s\nwant: %s", out, want)
	}
}

// C99: WithAlpha(0) means fully transparent and must emit <a:alpha val="0"/>;
// an unspecified alpha emits nothing; WithAlpha(50) is unchanged.
func TestWithAlpha_ZeroIsTransparentNotUnset(t *testing.T) {
	spPr := &SpPr{}
	NewSolidFill(ColorRed.WithAlpha(0)).ApplyToSpPr(spPr)
	if spPr.SolidFill.SrgbClr.Alpha == nil || spPr.SolidFill.SrgbClr.Alpha.Val != 0 {
		t.Errorf("WithAlpha(0) did not emit alpha val=0: %+v", spPr.SolidFill.SrgbClr.Alpha)
	}

	spPr = &SpPr{}
	NewSolidFill(ColorRed).ApplyToSpPr(spPr)
	if spPr.SolidFill.SrgbClr.Alpha != nil {
		t.Errorf("color without WithAlpha emitted an alpha transform: %+v", spPr.SolidFill.SrgbClr.Alpha)
	}

	spPr = &SpPr{}
	NewSolidFill(ColorRed.WithAlpha(50)).ApplyToSpPr(spPr)
	if spPr.SolidFill.SrgbClr.Alpha == nil || spPr.SolidFill.SrgbClr.Alpha.Val != 50000 {
		t.Errorf("WithAlpha(50) = %+v, want val=50000", spPr.SolidFill.SrgbClr.Alpha)
	}
}

// C93: a:blip effect children (duotone plus an unmodeled child) survive the
// production Builder in document order; nothing is dropped.
func TestBlipXML_EffectChildrenRoundTrip(t *testing.T) {
	in := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="rId2">` +
		`<a:lum bright="70000"/>` +
		`<a:duotone><a:prstClr val="black"/><a:srgbClr val="D9C3A5"/></a:duotone>` +
		`<a:futureFx amt="5"><a:sub val="1"/></a:futureFx>` +
		`<a:grayscl/>` +
		`</a:blip>`
	var v BlipXML
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(v.Effects) != 4 {
		t.Fatalf("effects lost on parse: got %d, want 4", len(v.Effects))
	}
	out := buildFragment(t, "blip", &v)
	want := `<a:blip r:embed="rId2">` +
		`<a:lum bright="70000"/>` +
		`<a:duotone><a:prstClr val="black"/><a:srgbClr val="D9C3A5"/></a:duotone>` +
		`<a:futureFx amt="5"><a:sub val="1"/></a:futureFx>` +
		`<a:grayscl/>` +
		`</a:blip>`
	if out != want {
		t.Errorf("blip round-trip mismatch:\n got: %s\nwant: %s", out, want)
	}
}

// C93: a blip with only attributes still emits a self-closed element, and its
// extLst is kept after the effect children.
func TestBlipXML_AttrsAndExtLst(t *testing.T) {
	in := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` r:embed="rId3" cstate="print"/>`
	var v BlipXML
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := buildFragment(t, "blip", &v)
	if out != `<a:blip r:embed="rId3" cstate="print"/>` {
		t.Errorf("attribute-only blip mismatch: %s", out)
	}

	in2 := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<a:biLevel thresh="50000"/><a:extLst><a:ext uri="{X}"/></a:extLst></a:blip>`
	var v2 BlipXML
	if err := xml.Unmarshal([]byte(in2), &v2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out2 := buildFragment(t, "blip", &v2)
	if !strings.Contains(out2, `<a:biLevel thresh="50000"/>`) || !strings.Contains(out2, `uri="{X}"`) {
		t.Errorf("biLevel or extLst lost: %s", out2)
	}
	if strings.Index(out2, "biLevel") > strings.Index(out2, "extLst") {
		t.Errorf("extLst must come after effect children: %s", out2)
	}
}

// C217: rotWithShape/scaled have no XSD default, so an explicit "0" must
// round-trip instead of being deleted.
func TestGradFill_ExplicitFalseAttrsPreserved(t *testing.T) {
	var v GradFill
	in := `<gradFill xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" rotWithShape="0">` +
		`<lin ang="5400000" scaled="0"/></gradFill>`
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := buildFragment(t, "gradFill", &v)
	if !strings.Contains(out, `rotWithShape="0"`) {
		t.Errorf("explicit rotWithShape=0 deleted: %s", out)
	}
	if !strings.Contains(out, `scaled="0"`) {
		t.Errorf("explicit scaled=0 deleted: %s", out)
	}
}

// clrChange useA defaults to true in the XSD: an explicit useA="0" must not be
// deleted (which would flip it back to true).
func TestClrChange_ExplicitFalseUseAPreserved(t *testing.T) {
	var v ClrChange
	in := `<clrChange xmlns="http://schemas.openxmlformats.org/drawingml/2006/main" useA="0">` +
		`<clrFrom><srgbClr val="0000FF"/></clrFrom><clrTo><srgbClr val="00FF00"/></clrTo></clrChange>`
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := buildFragment(t, "clrChange", &v)
	if !strings.Contains(out, `useA="0"`) {
		t.Errorf("explicit useA=0 deleted: %s", out)
	}
}

// C218: fld id is required by the XSD (ST_Guid); it must always be emitted.
func TestFld_IdAlwaysEmitted(t *testing.T) {
	out, err := xml.Marshal(&Fld{Type: "slidenum"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `id=`) {
		t.Errorf("required fld id attribute dropped: %s", out)
	}

	var v Fld
	in := `<fld xmlns="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` id="{7CE827D9-EC4E-4A2B-A6C1-9E28E7B816E6}" type="slidenum"/>`
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out2 := buildFragment(t, "fld", &v)
	if !strings.Contains(out2, `id="{7CE827D9-EC4E-4A2B-A6C1-9E28E7B816E6}"`) {
		t.Errorf("fld id lost on round-trip: %s", out2)
	}
}

// C219: wp cNvGraphicFramePr holds a:graphicFrameLocks, which must parse and
// survive re-marshal instead of being dropped.
func TestWPInline_GraphicFrameLocks(t *testing.T) {
	in := `<wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1" noSelect="1"/></wp:cNvGraphicFramePr>` +
		`</wp:inline>`
	var v WPInline
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.CNvGraphicFramePr == nil || v.CNvGraphicFramePr.GraphicFrameLocks == nil {
		t.Fatalf("graphicFrameLocks not parsed: %+v", v.CNvGraphicFramePr)
	}
	locks := v.CNvGraphicFramePr.GraphicFrameLocks
	if !locks.NoChangeAspect || !locks.NoSelect {
		t.Errorf("lock flags lost: %+v", locks)
	}
	out, err := xml.Marshal(&v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "graphicFrameLocks") || !strings.Contains(string(out), "noChangeAspect") {
		t.Errorf("graphicFrameLocks dropped on re-marshal: %s", out)
	}
}

// A negative gradient angle is normalized into [0, 21600000) instead of
// emitting a schema-invalid negative ang.
func TestNewGradientFill_NegativeAngleNormalized(t *testing.T) {
	spPr := &SpPr{}
	NewGradientFill(-45,
		GradientStop{Position: 0, Color: ColorRed},
		GradientStop{Position: 1, Color: ColorBlue},
	).ApplyToSpPr(spPr)
	if got := spPr.GradFill.Lin.Ang; got != 18900000 {
		t.Errorf("Lin.Ang = %d, want 18900000 (-45deg normalized to 315deg)", got)
	}
}
