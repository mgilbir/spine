package dml

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestWithTintEmitsOnSrgbClr pins C479: WithTint promised tinting with no
// theme-only caveat, but colorToSrgbClr never read Color.Tint, so every RGB
// color built through the convenience API lost it silently.
func TestWithTintEmitsOnSrgbClr(t *testing.T) {
	base := NewRGB(0x44, 0x72, 0xC4).ToColor()

	sf := solidFillFor(base.WithTint(0.5))
	if sf.SrgbClr == nil {
		t.Fatal("solid fill did not produce an srgbClr")
	}
	if sf.SrgbClr.Tint == nil {
		t.Fatal("WithTint(0.5) produced no a:tint on srgbClr (C479)")
	}
	if got := sf.SrgbClr.Tint.Val.Int32(); got != 50000 {
		t.Errorf("tint val = %d, want 50000", got)
	}
	if sf.SrgbClr.Shade != nil {
		t.Error("a positive tint must not also emit a:shade")
	}

	// A negative tint is a shade, matching the schemeClr mapping exactly.
	sf = solidFillFor(base.WithTint(-0.25))
	if sf.SrgbClr.Shade == nil {
		t.Fatal("WithTint(-0.25) produced no a:shade on srgbClr")
	}
	if got := sf.SrgbClr.Shade.Val.Int32(); got != 25000 {
		t.Errorf("shade val = %d, want 25000", got)
	}
	if sf.SrgbClr.Tint != nil {
		t.Error("a negative tint must not also emit a:tint")
	}

	// No tint set: no transform, so nothing changes for existing callers.
	sf = solidFillFor(base)
	if sf.SrgbClr.Tint != nil || sf.SrgbClr.Shade != nil {
		t.Error("an untinted color must emit a bare srgbClr")
	}

	// The theme path is unchanged.
	theme := Color{Type: ColorTypeTheme, Theme: ThemeColorAccent1}.WithTint(0.5)
	scheme := colorToSchemeClr(theme)
	if len(scheme.Tint) != 1 || scheme.Tint[0].Val.Int32() != 50000 {
		t.Errorf("schemeClr tint regressed: %+v", scheme.Tint)
	}
}

// solidFillFor runs a color through the public fill API and returns the
// a:solidFill it produces, the shape every convenience entry point shares.
func solidFillFor(c Color) *SolidFill {
	var spPr SpPr
	NewSolidFill(c).ApplyToSpPr(&spPr)
	return spPr.SolidFill
}

// TestEffectContainerBlurReplacement pins C480: EffectContainer.Blur and
// EffectDag.Cont are documented as settable programmatically, but the captured
// child list won, so assigning a new pointer after parse was discarded.
func TestEffectContainerBlurReplacement(t *testing.T) {
	src := `<a:cont><a:blur rad="10"/><a:alphaModFix amt="50000"/></a:cont>`
	var ec EffectContainer
	if err := parseElement(t, src, &ec); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ec.Blur == nil || ec.Blur.Rad != 10 {
		t.Fatalf("blur not parsed: %+v", ec.Blur)
	}

	// Replace the pointer (not mutate it): the new value must be emitted, at
	// the captured position, with the unmodeled sibling still intact.
	ec.Blur = &BlurXML{Rad: 99}
	out := marshalElement(t, "cont", &ec)
	if !strings.Contains(out, `rad="99"`) {
		t.Errorf("replaced Blur pointer discarded (C480): %s", out)
	}
	if strings.Contains(out, `rad="10"`) {
		t.Errorf("stale captured blur re-emitted: %s", out)
	}
	if !strings.Contains(out, "alphaModFix") {
		t.Errorf("raw-captured sibling lost: %s", out)
	}
	if strings.Index(out, "blur") > strings.Index(out, "alphaModFix") {
		t.Errorf("substituted blur moved out of its captured position: %s", out)
	}

	// Clearing the field removes the child.
	ec.Blur = nil
	if out := marshalElement(t, "cont", &ec); strings.Contains(out, "blur") {
		t.Errorf("cleared Blur still emitted: %s", out)
	}

	// Setting the field on a container that captured no blur appends one.
	var noBlur EffectContainer
	if err := parseElement(t, `<a:cont><a:alphaModFix amt="50000"/></a:cont>`, &noBlur); err != nil {
		t.Fatalf("parse: %v", err)
	}
	noBlur.Blur = &BlurXML{Rad: 7}
	if out := marshalElement(t, "cont", &noBlur); !strings.Contains(out, `rad="7"`) {
		t.Errorf("Blur set after parse not emitted: %s", out)
	}
}

// TestEffectDagContReplacement is C480 for EffectDag.Cont.
func TestEffectDagContReplacement(t *testing.T) {
	src := `<a:effectDag><a:cont name="a"><a:blur rad="1"/></a:cont></a:effectDag>`
	var ed EffectDag
	if err := parseElement(t, src, &ed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ed.Cont == nil {
		t.Fatal("cont not parsed")
	}
	ed.Cont = &EffectContainer{Name: "replaced"}
	out := marshalElement(t, "effectDag", &ed)
	if !strings.Contains(out, `name="replaced"`) {
		t.Errorf("replaced Cont pointer discarded (C480): %s", out)
	}
	if strings.Contains(out, `name="a"`) {
		t.Errorf("stale captured cont re-emitted: %s", out)
	}
}

// TestTileExplicitZeroScale pins C481: canonical "0" leaves Percentage.orig
// empty, so IsZeroAttr reported zero and omitempty deleted sx="0" — flipping an
// explicit 0% scale to the 100000 schema default.
func TestTileExplicitZeroScale(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		v    func() interface{}
	}{
		{"TileXML", `<a:tile sx="0" sy="100000"/>`, func() interface{} { return &TileXML{} }},
		{"Tile", `<a:tile sx="0" sy="100000"/>`, func() interface{} { return &Tile{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.v()
			if err := parseElement(t, tc.src, v); err != nil {
				t.Fatalf("parse: %v", err)
			}
			out := marshalElement(t, "tile", v)
			if out != tc.src {
				t.Errorf("explicit sx=\"0\" not preserved (C481):\n got %s\nwant %s", out, tc.src)
			}
		})
	}
}

// clrWrap is a decode target for the five pointer-based color types, used with
// UnmarshalWithSource so the duplicate-transform raw capture is armed.
type clrWrap struct {
	SrgbClr  *SrgbClr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SysClr   *SystemClr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	HslClr   *HslClr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	PrstClr  *PrstClr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
	ScRgbClr *ScRgbClr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
}

// TestColorTypesMarshalXMLPreservesOrder pins C482: the five pointer-based
// color types had custom UnmarshalXML + MarshalToBuilder but no MarshalXML, so
// the stdlib path fell back to struct tags — reordering the transforms into
// field order and dropping raw-captured duplicates entirely.
func TestColorTypesMarshalXMLPreservesOrder(t *testing.T) {
	tests := []struct {
		name string
		elem string
		src  string
		get  func(*clrWrap) interface{}
	}{
		{
			name: "srgbClr duplicate shade",
			elem: "srgbClr",
			src:  `<a:srgbClr val="FF0000"><a:gamma></a:gamma><a:shade val="50000"></a:shade><a:shade val="25000"></a:shade></a:srgbClr>`,
			get:  func(w *clrWrap) interface{} { return w.SrgbClr },
		},
		{
			name: "sysClr interleaved",
			elem: "sysClr",
			src:  `<a:sysClr val="windowText" lastClr="000000"><a:lumMod val="65000"></a:lumMod><a:inv></a:inv><a:tint val="20000"></a:tint></a:sysClr>`,
			get:  func(w *clrWrap) interface{} { return w.SysClr },
		},
		{
			name: "hslClr interleaved",
			elem: "hslClr",
			src:  `<a:hslClr hue="14400000" sat="50000" lum="50000"><a:shade val="30000"></a:shade><a:gray></a:gray></a:hslClr>`,
			get:  func(w *clrWrap) interface{} { return w.HslClr },
		},
		{
			name: "prstClr interleaved",
			elem: "prstClr",
			src:  `<a:prstClr val="black"><a:alpha val="50000"></a:alpha><a:comp></a:comp></a:prstClr>`,
			get:  func(w *clrWrap) interface{} { return w.PrstClr },
		},
		{
			name: "scrgbClr interleaved",
			elem: "scrgbClr",
			src:  `<a:scrgbClr r="50000" g="50000" b="50000"><a:invGamma></a:invGamma><a:lumOff val="10000"></a:lumOff></a:scrgbClr>`,
			get:  func(w *clrWrap) interface{} { return w.ScRgbClr },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w clrWrap
			doc := pctWrapperNS + tt.src + `</a:wrap>`
			if err := xmlb.UnmarshalWithSource([]byte(doc), &w); err != nil {
				t.Fatalf("parse: %v", err)
			}
			v := tt.get(&w)
			if v == nil {
				t.Fatalf("%s not decoded", tt.elem)
			}
			var buf bytes.Buffer
			enc := xml.NewEncoder(&buf)
			if err := enc.EncodeElement(v, xml.StartElement{Name: xml.Name{Local: tt.elem}}); err != nil {
				t.Fatalf("stdlib marshal: %v", err)
			}
			if err := enc.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
			// encoding/xml writes the element unprefixed, so compare against
			// the source with the a: prefix dropped.
			want := strings.ReplaceAll(tt.src, "a:", "")
			if got := buf.String(); got != want {
				t.Errorf("stdlib marshal lost transform order or duplicates (C482):\n got %s\nwant %s", got, want)
			}
		})
	}
}

// TestLockAttrCaptureSiblings pins C483: CxnSpLocks, GrpSpLocks and
// GraphicFrameLocks dropped explicit noGrp="0" and any unmodeled attribute,
// while their siblings SpLocks/PicLocks in the same file captured.
func TestLockAttrCaptureSiblings(t *testing.T) {
	tests := []struct {
		name string
		elem string
		src  string
		v    func() interface{}
	}{
		{"cxnSpLocks", "cxnSpLocks", `<a:cxnSpLocks noGrp="0" noRot="1"/>`, func() interface{} { return &CxnSpLocks{} }},
		{"grpSpLocks", "grpSpLocks", `<a:grpSpLocks noGrp="0" noUngrp="1"/>`, func() interface{} { return &GrpSpLocks{} }},
		{"graphicFrameLocks", "graphicFrameLocks", `<a:graphicFrameLocks noGrp="0" noDrilldown="1"/>`, func() interface{} { return &GraphicFrameLocks{} }},
		// The captured siblings, so the test fails if they ever regress too.
		{"spLocks", "spLocks", `<a:spLocks noGrp="0" noSelect="1"/>`, func() interface{} { return &SpLocks{} }},
		{"picLocks", "picLocks", `<a:picLocks noGrp="0" noCrop="1"/>`, func() interface{} { return &PicLocks{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.v()
			if err := parseElement(t, tt.src, v); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if out := marshalElement(t, tt.elem, v); out != tt.src {
				t.Errorf("explicit noGrp=\"0\" not preserved (C483):\n got %s\nwant %s", out, tt.src)
			}
		})
	}
}
